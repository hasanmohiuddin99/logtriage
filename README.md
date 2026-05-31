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
