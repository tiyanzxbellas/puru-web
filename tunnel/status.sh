#!/data/data/com.termux/files/usr/bin/bash
cd "$(dirname "$0")"
PID=$(./lt-pid.sh | head -1)
[ -z "$PID" ] && { echo "localtunnel: MATI"; exit 0; }
URL=$(cat lt.url 2>/dev/null)
echo "localtunnel: JALAN (PID $PID)"
echo "URL      : $URL"
echo "HTTP     : $(curl -s -m 15 -o /dev/null -w '%{http_code}' "$URL/api/info")"
curl -s -m 15 "$URL/api/info" | head -c 200; echo
