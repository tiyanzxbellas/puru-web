#!/data/data/com.termux/files/usr/bin/bash
cd "$(dirname "$0")"
PIDS=$(./lt-pid.sh)
if [ -z "$PIDS" ]; then echo "Tunnel localtunnel tidak jalan."; rm -f lt.url; exit 0; fi
for p in $PIDS; do kill "$p" 2>/dev/null && echo "dimatikan PID $p"; done
sleep 1; rm -f lt.url lt.pid
