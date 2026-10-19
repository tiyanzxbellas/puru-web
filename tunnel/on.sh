#!/data/data/com.termux/files/usr/bin/bash
cd "$(dirname "$0")"
PID=$(./lt-pid.sh | head -1)
if [ -n "$PID" ]; then
  echo "Tunnel localtunnel SUDAH JALAN (PID $PID)"
  echo "URL: $(cat lt.url 2>/dev/null)"
  exit 0
fi
if ! curl -s -m 5 -o /dev/null http://127.0.0.1:18099/api/info; then
  echo "PERINGATAN: web Puru di 127.0.0.1:18099 belum nyala."
fi
rm -f lt.url lt.pid
setsid nohup node lt-start.cjs >> lt.run.log 2>&1 < /dev/null &
# tunggu sampai 15s: kalau subdomain pilihan "masih dipegang" server,
# runner otomatis coba nama cadangan (butuh beberapa detik)
for i in $(seq 1 15); do
  [ -s lt.url ] && break
  sleep 1
done
URL=$(cat lt.url 2>/dev/null)
if [ -n "$URL" ]; then
  echo "Tunnel AKTIF: $URL"
  echo "Cek: $(curl -s -m 10 -o /dev/null -w '%{http_code}' "$URL/api/info")"
else
  echo "Gagal start (URL belum keluar dalam 15s). Log:"; tail -5 lt.log 2>/dev/null; tail -5 lt.run.log 2>/dev/null
fi
