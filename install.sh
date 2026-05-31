#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# logtriage installer — makes it a real system-level tool
# Usage:  sudo bash install.sh
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

TOOL_NAME="logtriage"
INSTALL_DIR="/usr/local/bin"
LOG_DIR="/var/log/logtriage"
CRON_FILE="/etc/cron.d/logtriage"
SERVICE_FILE="/etc/systemd/system/logtriage.service"
SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ── Default log source — CHANGE THIS to your actual log file ─────────────────
LOG_SOURCE="/var/log/syslog"

RED='\033[1;31m'; YEL='\033[1;33m'; GRN='\033[1;32m'
CYN='\033[1;36m'; DIM='\033[0;90m'; RST='\033[0m'

step() { echo -e "${CYN}  ▸ $*${RST}"; }
ok()   { echo -e "${GRN}    ✔ $*${RST}"; }
warn() { echo -e "${YEL}    ⚠ $*${RST}"; }
die()  { echo -e "${RED}    ✘ $*${RST}"; exit 1; }

echo
echo -e "${CYN}  ════════════════════════════════════════════════════════${RST}"
echo -e "${CYN}    logtriage  —  System Installer                        ${RST}"
echo -e "${CYN}  ════════════════════════════════════════════════════════${RST}"
echo

[[ $EUID -eq 0 ]] || die "Please run as root:  sudo bash install.sh"
command -v go &>/dev/null || die "Go not found. Install from https://go.dev/dl/"

# ── 1. Build ──────────────────────────────────────────────────────────────────
step "Building binary …"
cd "$SOURCE_DIR"
go mod tidy
go build -ldflags "-s -w -X main.version=$(date +%Y.%m.%d)" -o "$TOOL_NAME" .
ok "Binary built"

# ── 2. Install binary to PATH ─────────────────────────────────────────────────
step "Installing to ${INSTALL_DIR}/${TOOL_NAME} …"
install -m 0755 "$TOOL_NAME" "${INSTALL_DIR}/${TOOL_NAME}"
ok "Binary installed → type 'logtriage' from anywhere"

# ── 3. Log directory ──────────────────────────────────────────────────────────
step "Creating ${LOG_DIR} …"
mkdir -p "$LOG_DIR"
chmod 750 "$LOG_DIR"
ok "Log dir ready"

# ── 4. Cron job (every 5 min, warning+ only) ──────────────────────────────────
step "Writing cron job → ${CRON_FILE} …"
cat > "$CRON_FILE" << CRONEOF
# logtriage background cron — runs every 5 minutes
# Edit LOG_SOURCE below to point at your actual log file.
SHELL=/bin/bash
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin

*/5 * * * *   root   ${INSTALL_DIR}/${TOOL_NAME} -file ${LOG_SOURCE} -silent -min-severity warning -out ${LOG_DIR}/triage.log
CRONEOF
chmod 0644 "$CRON_FILE"
ok "Cron installed  (every 5 min, output → ${LOG_DIR}/triage.log)"

# ── 5. systemd service (optional continuous daemon) ───────────────────────────
step "Writing systemd service → ${SERVICE_FILE} …"
cat > "$SERVICE_FILE" << SVCEOF
[Unit]
Description=logtriage — live security log triage daemon
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${TOOL_NAME} -file ${LOG_SOURCE} -silent -out ${LOG_DIR}/live.log
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
SVCEOF
systemctl daemon-reload
ok "systemd service registered  (logtriage.service)"
warn "Service is NOT auto-started. Enable it manually when you want continuous monitoring:"
echo -e "     ${DIM}sudo systemctl enable --now logtriage${RST}"

# ── Done ──────────────────────────────────────────────────────────────────────
echo
echo -e "${GRN}  ════════════════════════════════════════════════════════${RST}"
echo -e "${GRN}    All done!                                             ${RST}"
echo -e "${GRN}  ════════════════════════════════════════════════════════${RST}"
echo
echo -e "  ${CYN}Quick-start commands:${RST}"
echo -e "  ${DIM}─────────────────────────────────────────────────────────${RST}"
echo -e "  ${YEL}Interactive (type JSON, see results):${RST}"
echo -e "    logtriage"
echo
echo -e "  ${YEL}Read & display ALL events from a file:${RST}"
echo -e "    logtriage -file /path/to/logs.json"
echo
echo -e "  ${YEL}Read file, show warnings and above only:${RST}"
echo -e "    logtriage -file /path/to/logs.json -min-severity warning"
echo
echo -e "  ${YEL}Live tail (pipe mode):${RST}"
echo -e "    tail -f /var/log/app.json | logtriage"
echo
echo -e "  ${YEL}Check background cron output (all runs):${RST}"
echo -e "    logtriage -logs"
echo -e "    cat ${LOG_DIR}/triage.log"
echo
echo -e "  ${YEL}Enable continuous background daemon:${RST}"
echo -e "    sudo systemctl enable --now logtriage"
echo -e "    sudo systemctl status logtriage"
echo -e "    logtriage -logs-live"
echo
