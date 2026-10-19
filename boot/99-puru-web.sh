#!/data/data/com.termux/files/usr/bin/sh
# ---------------------------------------------------------------
# Puru Web — skrip AUTOSTART (Termux:Boot)
# Dijalankan otomatis oleh app Termux:Boot tiap HP selesai booting.
#
# Yang dinyalakan:
#   1. puruweb (web AI chat)  -> http://127.0.0.1:18099
#   2. tunnel localtunnel     -> https://puru-ai.loca.lt
#
# Aman dipanggil berkali-kali: kalau sudah jalan, cuma dilewati.
# Log: <repo>/boot.log   |   Log web: <repo>/.build/server.log
# ---------------------------------------------------------------
termux-wake-lock 2>/dev/null

REPO="${PURU_REPO_DIR:-$HOME/project source code/puruweb-run}"
PORT="${PORT:-18099}"
URL="http://127.0.0.1:$PORT"
LOG="$REPO/boot.log"

exec >>"$LOG" 2>&1
echo "=== boot run $(date) ==="

cd "$REPO" || { echo "FATAL repo tidak ada: $REPO"; exit 1; }

# batas ukuran file: naikkan kalau parent mengizinkan (kalau tidak, diabaikan)
ulimit -f unlimited 2>/dev/null

# tunggu HP / storage / jaringan benar-benar siap
sleep 30

# ---- 1. web puru -------------------------------------------------
if curl -s -m 3 -o /dev/null "$URL/api/info"; then
  echo "puruweb sudah jalan (lewati)"
else
  if [ -x .build/puruweb ]; then
    mkdir -p .build
    setsid nohup ./.build/puruweb -config .build/config.json -addr "127.0.0.1:$PORT" >> .build/server.log 2>&1 </dev/null &
    i=1
    while [ $i -le 30 ]; do
      curl -s -m 2 -o /dev/null "$URL/api/info" && break
      sleep 1
      i=$((i+1))
    done
    if curl -s -m 3 -o /dev/null "$URL/api/info"; then
      echo "puruweb NYALA di $URL"
    else
      echo "puruweb GAGAL nyala (lihat .build/server.log)"
    fi
  else
    echo "FATAL .build/puruweb tidak ada — build dulu: ./run_web.sh"
  fi
fi

# ---- 2. tunnel localtunnel --------------------------------------
if [ -x tunnel/on.sh ]; then
  ./tunnel/on.sh
else
  echo "lewati tunnel: tunnel/on.sh tidak ada"
fi

echo "=== selesai $(date) ==="
