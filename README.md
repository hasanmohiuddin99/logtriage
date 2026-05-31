# logtriage

A fast, lightweight CLI utility written in Go that transforms raw, nested JSON security logs into clean, human-readable English sentences. It categorizes events by severity and provides immediate operational context for quick triage.

## Features
- **Human-Readable Parsing:** Converts complex JSON fields into clear, descriptive sentences.
- **Immediate Classification:** Flags potential security risks (e.g., Reverse Shells, Persistence, Brute Force).
- **Visual Distinction:** Uses terminal color-coding based on event severity (CRITICAL, WARNING, INFO).
- **Flexible Inputs:** Supports scanning full files, live-tailing a file (`-watch`), or piping data directly via `stdin`.

## Installation

Ensure you have Go installed, then clone the repository and build:

```bash
git clone [https://github.com/hasanmohiuddin99/logtriage.git](https://github.com/hasanmohiuddin99/logtriage.git)
cd logtriage
go build -o logtriage
sudo cp logtriage /usr/local/bin/

Alternatively, if you want to use the provided installation script:
Bash

chmod +x install.sh
./install.sh

Usage

Once installed, you can call logtriage from any directory on your system.
1. Scan an Existing Log File

To process an entire static JSON log file from start to finish:
Bash

logtriage -file /path/to/logs.json

2. Live-Tail an Active Log File

To monitor a log file in real-time as new events are being written (similar to tail -f):
Bash

logtriage -file /path/to/logs.json -watch

3. Pipe Logs via Stdin

You can pipe the output of any other command or log shipper straight into the utility:
Bash

cat /path/to/logs.json | logtriage

4. Interactive Mode

Simply run the binary by itself to manually paste or type raw JSON log lines directly into the terminal:
Bash

logtriage

5. Filter by Minimum Severity

Filter out lower-priority noise by specifying the lowest severity tier you want to display (info, warning, or critical):
Bash

logtriage -file /path/to/logs.json -min-severity warning
