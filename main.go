package main

import (
    "bufio"
    "flag"
    "fmt"
    "io"
    "os"
    "strings"
    "time"

    "github.com/fatih/color"
)

var version = "1.3.0"

const defaultLogDir  = "/var/log/logtriage"
const defaultLogFile = "/var/log/logtriage/triage.log"

var severityRank = map[string]int{
    SeverityInfo:     1,
    SeverityUnknown:  2,
    SeverityWarning:  3,
    SeverityCritical: 4,
}

type stats struct{ read, critical, warning, info, unknown, errors int }

func (s *stats) inc(sev string) {
    switch sev {
    case SeverityCritical:
        s.critical++
    case SeverityWarning:
        s.warning++
    case SeverityInfo:
        s.info++
    default:
        s.unknown++
    }
}

func main() {
    fileFlag    := flag.String("file",         "",      "JSON-lines log file to read (all events displayed)")
    outFlag     := flag.String("out",          "",      "Append plain-text events to this file (for cron)")
    silentFlag  := flag.Bool("silent",         false,   "Suppress terminal output — only write to -out")
    watchFlag   := flag.Bool("watch",          false,   "Tail a -file live (keep running, show new lines)")
    logsFlag    := flag.Bool("logs",           false,   "Pretty-print the saved background triage log")
    logsLive    := flag.Bool("logs-live",      false,   "Live-tail the saved background triage log")
    minSevFlag  := flag.String("min-severity", "info",  "Minimum severity: info | warning | critical")
    versionFlag := flag.Bool("version",        false,   "Print version and exit")
    flag.Usage   = printUsage
    flag.Parse()

    if *versionFlag {
        fmt.Printf("logtriage v%s\n", version)
        os.Exit(0)
    }

    if *logsLive {
        tailPlainLog(defaultLogFile, true)
        return
    }
    if *logsFlag {
        tailPlainLog(defaultLogFile, false)
        return
    }

    minRank, ok := severityRank[strings.ToUpper(*minSevFlag)]
    if !ok {
        fmt.Fprintf(os.Stderr, "logtriage: unknown -min-severity %q (use info|warning|critical)\n", *minSevFlag)
        os.Exit(1)
    }

    var outFile *os.File
    if *outFlag != "" {
        var err error
        outFile, err = os.OpenFile(*outFlag, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
        if err != nil {
            fmt.Fprintf(os.Stderr, "logtriage: cannot open -out %q: %v\n", *outFlag, err)
            os.Exit(1)
        }
        defer outFile.Close()
        fmt.Fprintf(outFile, "\n%s\n", strings.Repeat("═", 72))
        fmt.Fprintf(outFile, "  logtriage session  |  %s  |  min: %s\n",
            time.Now().Format("2006-01-02 15:04:05"), strings.ToUpper(*minSevFlag))
        fmt.Fprintf(outFile, "%s\n\n", strings.Repeat("═", 72))
    }

    var input *os.File
    var inputLabel string
    var mode string

    switch {
    case *fileFlag != "" && *watchFlag:
        mode       = "watch"
        inputLabel = *fileFlag + "  (watching for new lines)"
    case *fileFlag != "":
        mode       = "file"
        inputLabel = *fileFlag + "  (full scan)"
        f, err := os.Open(*fileFlag)
        if err != nil {
            fmt.Fprintf(os.Stderr, "logtriage: cannot open %q: %v\n", *fileFlag, err)
            os.Exit(1)
        }
        defer f.Close()
        input = f
    default:
        mode       = "stdin"
        inputLabel = "stdin  (interactive / pipe)"
        input      = os.Stdin
    }

    if !*silentFlag {
        printBanner(inputLabel, strings.ToUpper(*minSevFlag), outFile != nil, mode)
    }

    var s stats

    if mode == "watch" {
        watchFile(*fileFlag, minRank, outFile, *silentFlag, &s)
        return
    }

    processReader(input, minRank, outFile, *silentFlag, &s)

    if !*silentFlag {
        printSummary(s)
    }
    if outFile != nil {
        fmt.Fprintf(outFile, "\nSummary — read:%d  critical:%d  warning:%d  info:%d  errors:%d\n",
            s.read, s.critical, s.warning, s.info, s.errors)
    }
}

func processReader(r io.Reader, minRank int, outFile *os.File, silent bool, s *stats) {
    scanner := bufio.NewScanner(r)
    scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        s.read++

        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }

        event, err := ParseLine([]byte(line))
        if err != nil {
            s.errors++
            if !silent {
                color.New(color.FgHiBlack).Fprintf(os.Stderr,
                    "   ✗ parse error (line %d): %v\n", s.read, err)
            }
            if outFile != nil {
                fmt.Fprintf(outFile, "[PARSE ERROR] line %d: %v\n", s.read, err)
            }
            continue
        }

        if severityRank[event.Severity] < minRank {
            continue
        }

        s.inc(event.Severity)

        if !silent {
            renderEvent(event, s)
        }
        if outFile != nil {
            writePlainEvent(outFile, event)
        }
    }

    if err := scanner.Err(); err != nil {
        fmt.Fprintf(os.Stderr, "logtriage: read error: %v\n", err)
    }
}

func watchFile(path string, minRank int, outFile *os.File, silent bool, s *stats) {
    f, err := os.Open(path)
    if err != nil {
        fmt.Fprintf(os.Stderr, "logtriage: cannot open %q: %v\n", path, err)
        os.Exit(1)
    }
    defer f.Close()

    if _, err := f.Seek(0, io.SeekEnd); err != nil {
        fmt.Fprintf(os.Stderr, "logtriage: seek error: %v\n", err)
        os.Exit(1)
    }

    reader := bufio.NewReader(f)
    dim    := color.New(color.FgHiBlack)

    if !silent {
        dim.Printf("   Watching %s for new events … (Ctrl+C to stop)\n\n", path)
    }

    for {
        line, err := reader.ReadString('\n')
        if err != nil {
            time.Sleep(1 * time.Second)
            continue
        }
        line = strings.TrimSpace(line)
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        s.read++
        event, perr := ParseLine([]byte(line))
        if perr != nil {
            s.errors++
            continue
        }
        if severityRank[event.Severity] < minRank {
            continue
        }
        s.inc(event.Severity)
        if !silent {
            renderEvent(event, s)
        }
        if outFile != nil {
            writePlainEvent(outFile, event)
        }
    }
}

func tailPlainLog(path string, live bool) {
    dim  := color.New(color.FgHiBlack)
    cyn  := color.New(color.FgHiCyan, color.Bold)
    bold := color.New(color.FgWhite, color.Bold)

    cyn.Println()
    cyn.Println("  ╔══════════════════════════════════════════════════════════╗")
    if live {
        cyn.Println("  ║   logtriage — Live Background Log Viewer                 ║")
    } else {
        cyn.Println("  ║   logtriage — Background Log Viewer                      ║")
    }
    cyn.Println("  ╚══════════════════════════════════════════════════════════╝")
    dim.Printf("   File: %s\n\n", path)

    f, err := os.Open(path)
    if err != nil {
        fmt.Fprintf(os.Stderr,
            "\n   logtriage: cannot open log file %q\n   Has the cron job or daemon run yet?\n   Expected location: %s\n\n",
            path, path)
        os.Exit(1)
    }
    defer f.Close()

    if live {
        f.Seek(0, io.SeekEnd)
    }

    scanner := bufio.NewScanner(f)
    for {
        for scanner.Scan() {
            line := scanner.Text()
            switch {
            case strings.Contains(line, "[CRITICAL]"):
                color.New(color.FgHiRed, color.Bold).Println("  " + line)
            case strings.Contains(line, "[WARNING ]"):
                color.New(color.FgHiYellow, color.Bold).Println("  " + line)
            case strings.Contains(line, "[INFO    ]"):
                color.New(color.FgHiGreen).Println("  " + line)
            case strings.HasPrefix(strings.TrimSpace(line), "═"):
                cyn.Println("  " + line)
            case strings.HasPrefix(strings.TrimSpace(line), "logtriage session"):
                bold.Println("  " + line)
            case strings.HasPrefix(strings.TrimSpace(line), "╰─"):
                dim.Println("  " + line)
            default:
                dim.Println("  " + line)
            }
        }
        if !live {
            break
        }
        time.Sleep(1 * time.Second)
    }
}

func renderEvent(e *ParsedEvent, s *stats) {
    var badgeColor, accentLine *color.Color
    switch e.Severity {
    case SeverityCritical:
        badgeColor = color.New(color.BgRed, color.FgHiWhite, color.Bold)
        accentLine = color.New(color.FgRed)
    case SeverityWarning:
        badgeColor = color.New(color.FgHiYellow, color.Bold)
        accentLine = color.New(color.FgYellow)
    case SeverityInfo:
        badgeColor = color.New(color.FgHiGreen, color.Bold)
        accentLine = color.New(color.FgGreen)
    default:
        badgeColor = color.New(color.FgHiBlack, color.Bold)
        accentLine = color.New(color.FgHiBlack)
    }

    dim   := color.New(color.FgHiBlack)
    tsCol := color.New(color.FgCyan)
    seqCol:= color.New(color.FgHiBlack, color.Bold)
    total := s.critical + s.warning + s.info + s.unknown

    accentLine.Print("  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄\n")

    fmt.Print("  ")
    seqCol.Printf("[#%04d]", total)
    fmt.Print("  ")
    tsCol.Printf("[%s]", e.Timestamp)
    fmt.Print("  ")
    badgeColor.Printf(" ■ %-8s ", e.Severity)
    fmt.Println()

    fmt.Print("           ")
    fmt.Printf("%s\n", e.Sentence)

    fmt.Print("           ")
    dim.Print("╰─ ")
    badgeColor.Printf("%s\n", e.Classification)
    fmt.Println()
}

func writePlainEvent(f *os.File, e *ParsedEvent) {
    fmt.Fprintf(f, "[%s] [%-8s]  %s\n", e.Timestamp, e.Severity, e.Sentence)
    fmt.Fprintf(f, "                        ╰─ %s\n\n", e.Classification)
}

func printBanner(source, minSev string, hasOut bool, mode string) {
    bold := color.New(color.FgHiCyan, color.Bold)
    dim  := color.New(color.FgHiBlack)
    hi   := color.New(color.FgWhite)

    bold.Println()
    bold.Println("  ██╗      ██████╗  ██████╗ ████████╗██████╗ ██╗ █████╗  ██████╗ ███████╗")
    bold.Println("  ██║     ██╔═══██╗██╔════╝ ╚══██╔══╝██╔══██╗██║██╔══██╗██╔════╝ ██╔════╝")
    bold.Println("  ██║     ██║   ██║██║  ███╗   ██║   ██████╔╝██║███████║██║  ███╗█████╗  ")
    bold.Println("  ██║     ██║   ██║██║   ██║   ██║   ██╔══██╗██║██╔══██║██║   ██║██╔══╝  ")
    bold.Println("  ███████╗╚██████╔╝╚██████╔╝   ██║   ██║  ██║██║██║  ██║╚██████╔╝███████╗")
    bold.Println("  ╚══════╝ ╚═════╝  ╚═════╝    ╚═╝   ╚═╝  ╚═╝╚═╝╚═╝  ╚═╝ ╚═════╝ ╚══════╝")
    dim.Printf("                          Security Log Triage Tool  v%s\n", version)
    fmt.Println()

    modeStr := map[string]string{
        "stdin": "Interactive / Pipe",
        "file":  "Full File Scan",
        "watch": "Live File Watch",
    }[mode]

    outStr := "disabled"
    if hasOut {
        outStr = "enabled (appending)"
    }

    dim.Println("  ┌─────────────────────────────────────────────────────────┐")
    printRow := func(label, val string) {
        dim.Print("  │  ")
        dim.Printf("%-20s", label)
        hi.Printf("%-37s", val)
        dim.Println("│")
    }
    printRow("Mode",         modeStr)
    printRow("Source",       source)
    printRow("Min Severity", minSev)
    printRow("File Output",  outStr)
    printRow("Started",      time.Now().Format("2006-01-02  15:04:05 MST"))
    dim.Println("  └─────────────────────────────────────────────────────────┘")
    fmt.Println()

    fmt.Print("  Severity  ")
    color.New(color.BgRed, color.FgHiWhite, color.Bold).Print(" ■ CRITICAL ")
    fmt.Print("   ")
    color.New(color.FgHiYellow, color.Bold).Print("■ WARNING")
    fmt.Print("   ")
    color.New(color.FgHiGreen, color.Bold).Print("■ INFO")
    fmt.Println("\n")

    if mode == "stdin" {
        dim.Println("  Paste or type JSON log lines, one per line.  Ctrl+C to stop.\n")
    }
}

func printSummary(s stats) {
    dim  := color.New(color.FgHiBlack)
    bold := color.New(color.FgHiCyan, color.Bold)

    fmt.Println()
    dim.Println("  ┌─────────────────────────────────────────────────────────┐")
    dim.Print("  │  "); bold.Printf("%-57s", "Session Summary"); dim.Println("│")
    dim.Println("  ├─────────────────────────────────────────────────────────┤")

    row := func(label, val string, c *color.Color) {
        dim.Print("  │  ")
        dim.Printf("%-20s", label)
        c.Printf("%-37s", val)
        dim.Println("│")
    }
    row("Lines read",   fmt.Sprintf("%d", s.read),     color.New(color.FgWhite))
    row("Critical",     fmt.Sprintf("%d", s.critical),  color.New(color.FgHiRed, color.Bold))
    row("Warning",      fmt.Sprintf("%d", s.warning),   color.New(color.FgHiYellow, color.Bold))
    row("Info",         fmt.Sprintf("%d", s.info),      color.New(color.FgHiGreen, color.Bold))
    row("Unknown",      fmt.Sprintf("%d", s.unknown),   color.New(color.FgHiBlack))
    row("Parse errors", fmt.Sprintf("%d", s.errors),    color.New(color.FgRed))
    dim.Println("  └─────────────────────────────────────────────────────────┘")
    fmt.Println()
}

func printUsage() {
    c := color.New(color.FgHiCyan, color.Bold)
    d := color.New(color.FgHiBlack)
    w := color.New(color.FgWhite)

    c.Printf("\n  logtriage v%s — Security Log Triage Tool\n\n", version)

    c.Println("  MODES")
    w.Println("     logtriage")
    d.Println("       Interactive stdin — type/paste JSON lines, see results instantly\n")

    w.Println("     logtriage -file /path/to/logs.json")
    d.Println("       Scan ALL events in a file and display them with colours\n")

    w.Println("     logtriage -file /path/to/logs.json -watch")
    d.Println("       Live-tail a file — shows new lines as they arrive (like tail -f)\n")

    w.Println("     tail -f /var/log/app.json | logtriage")
    d.Println("       Pipe mode — works with any log shipper\n")

    w.Println("     logtriage -logs")
    d.Println("       Display all events collected by the background cron job\n")

    w.Println("     logtriage -logs-live")
    d.Println("       Live-tail the background cron log (watch cron results in real time)\n")

    c.Println("  FLAGS")
    d.Println("     -file PATH          JSON-lines log file")
    d.Println("     -watch              Tail -file live (use with -file)")
    d.Println("     -out PATH           Append plain-text output to file (for cron)")
    d.Println("     -silent             No terminal output (use with -out for cron)")
    d.Println("     -min-severity LVL   info | warning | critical  (default: info)")
    d.Println("     -logs               Show background cron log")
    d.Println("     -logs-live          Live-tail background cron log")
    d.Println("     -version            Print version\n")
}
