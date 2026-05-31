package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	SeverityCritical = "CRITICAL"
	SeverityWarning  = "WARNING"
	SeverityInfo     = "INFO"
	SeverityUnknown  = "UNKNOWN"
)

type ParsedEvent struct {
	Timestamp      string
	Severity       string
	Sentence       string
	Classification string
}

type RawEnvelope struct {
	EventType  string `json:"event_type"`
	EventID    int    `json:"event_id"`
	Severity   string `json:"severity"`
	User       string `json:"user"`
	Parent     string `json:"parent"`
	Proc       string `json:"proc"`
	Args       string `json:"args"`
	TargetUser string `json:"target_user"`
	Action     string `json:"action"`
	SrcIP      string `json:"src_ip"`
	Service    string `json:"service"`
	Status     string `json:"status"`
	DestIP     string `json:"dest_ip"`
	DestPort   int    `json:"dest_port"`
	Bytes      int64  `json:"bytes"`
}

func ParseLine(line []byte) (*ParsedEvent, error) {
	var raw RawEnvelope
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil, err
	}

	severity := strings.ToUpper(raw.Severity)
	if severity == "" {
		severity = SeverityUnknown
	}

	event := &ParsedEvent{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Severity:  severity,
	}

	switch raw.EventType {
	case "process":
		event.Sentence = fmt.Sprintf("The user '%s' (acting through parent process '%s') started a program called '%s' with the arguments: %s", raw.User, raw.Parent, raw.Proc, raw.Args)
		event.Classification = "Potential Reverse Shell / Remote Code Execution"
		event.Severity = SeverityCritical

	case "iam":
		if raw.Action == "delete" {
			event.Sentence = fmt.Sprintf("Administrative account '%s' deleted a local user account called '%s'.", raw.User, raw.TargetUser)
		} else {
			event.Sentence = fmt.Sprintf("Administrative account '%s' created a local user account called '%s'.", raw.User, raw.TargetUser)
		}
		event.Classification = "Persistence Mechanism Detected"
		event.Severity = SeverityWarning

	case "auth":
		if strings.ToLower(raw.Status) == "failed" {
			event.Sentence = fmt.Sprintf("A FAILED login attempt was made against the account '%s' via %s from source IP %s.", raw.User, raw.Service, raw.SrcIP)
			event.Classification = "Brute Force / Unauthorized Access Attempt"
			event.Severity = SeverityWarning
		} else {
			event.Sentence = fmt.Sprintf("A SUCCESSFUL login was recorded for the account '%s' via %s from source IP %s.", raw.User, raw.Service, raw.SrcIP)
			event.Classification = "Authenticated Session Established"
			event.Severity = SeverityInfo
		}

	case "network":
		humanBytes := float64(raw.Bytes)
		unit := "B"
		if humanBytes >= 1024*1024 {
			humanBytes /= (1024 * 1024)
			unit = "MB"
		} else if humanBytes >= 1024 {
			humanBytes /= 1024
			unit = "KB"
		}
		event.Sentence = fmt.Sprintf("Internal host %s established an outbound network connection to %s on port %d (%.1f %s transferred).", raw.SrcIP, raw.DestIP, raw.DestPort, humanBytes, unit)
		event.Classification = "Standard Outbound Traffic / Potential Exfiltration"
		event.Severity = SeverityInfo

	default:
		event.Sentence = fmt.Sprintf("An unrecognized log event was received (type: '%s'). Manual review recommended.", raw.EventType)
		event.Classification = "Unknown / Unclassified Event"
		event.Severity = SeverityUnknown
	}

	return event, nil
}
