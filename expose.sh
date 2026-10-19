#!/bin/sh
# expose.sh — bikin Puru Web (default 127.0.0.1:18099) bisa dibuka dari internet.
# Pilih: ./expose.sh [auto|cloudflared|ngrok|ssh|lt]
# localtunnel (lt) -> pakai ./tunnel/on.sh kalau ada (nama tetap, mis. puru-ai.loca.lt)
set -e
cd "$(dirname "$0")"

PORT="${PORT:-18099}"
TARGET="http://127.0.0.1:${PORT}"
MODE="${1:-auto}"

banner() {
  echo ""
  echo "⚠️  Web ini BELUM ADA LOGIN."
  echo "    Siapa pun yang dapat URL bisa: ngobrol pakai kuota model kamu,"
  echo "    baca file di workspace/downloads, upload file, dan menyuruh"
  echo "    agent menjalankan perintah shell di HP ini."
  echo "    Bagikan hanya ke orang yang kamu percaya — matikan (Ctrl-C) setelah selesai."
  echo ""
}

# 1) pastikan server lokal hidup
if command -v curl >/dev/null 2>&1; then
  if ! curl -s -m 3 -o /dev/null "$TARGET/api/info"; then
    echo "❌ Server lokal $TARGET belum jalan."
    echo "   Jalankan dulu di terminal lain:  ./run_web.sh"
    exit 1
  fi
else
  echo "ℹ️  curl tidak ada — lewati pengecekan server. Pastikan ./run_web.sh sudah jalan."
fi
echo "✔ Server lokal hidup di $TARGET"

pick() {
  case "$MODE" in
    auto)
      for c in cloudflared ngrok lt; do
        if command -v "$c" >/dev/null 2>&1; then echo "$c"; return; fi
      done
      echo "ssh" ;;
    *) echo "$MODE" ;;
  esac
}

WHICH="$(pick)"
banner

case "$WHICH" in
  cloudflared)
    command -v cloudflared >/dev/null 2>&1 || {
      echo "❌ cloudflared belum terpasang. Install dulu:  pkg install -y cloudflared"; exit 1; }
    echo "▶ cloudflared quick tunnel… (URL muncul di bawah, Ctrl-C untuk stop)"
    exec cloudflared tunnel --url "$TARGET" ;;
  ngrok)
    command -v ngrok >/dev/null 2>&1 || {
      echo "❌ ngrok belum terpasang / belum ada authtoken. Lihat TERMUX-DEPLOY.md §2b"; exit 1; }
    # Di Termux/Android ngrok WAJIB lewat wrapper proot (fix DNS /etc/resolv.conf
    # yang gak ada + CA Termux). Lihat TERMUX-DEPLOY.md §2d.
    if [ -x ./ngrok-termux.sh ]; then
      echo "▶ ngrok via ngrok-termux.sh (fix DNS + CA Termux)…"
      exec ./ngrok-termux.sh http "$PORT"
    fi
    echo "▶ ngrok… (URL muncul di bawah, Ctrl-C untuk stop)"
    exec ngrok http "$PORT" ;;
  lt)
    # Runner repo (nama bisa dipilih + anti-crash CLI di Android).
    if [ -x ./tunnel/on.sh ]; then
      echo "▶ localtunnel via tunnel/on.sh (nama: ${LT_SUB:-puru-ai}) …"
      exec ./tunnel/on.sh
    fi
    command -v lt >/dev/null 2>&1 || {
      echo "❌ localtunnel belum terpasang. Install:  npm i -g localtunnel"; exit 1; }
    echo "▶ localtunnel… (Ctrl+C untuk stop)"
    exec lt --port "$PORT" --subdomain "${LT_SUB:-puru-ai}" ;;
  ssh)
    echo "▶ SSH tunnel via localhost.run… (URL muncul di bawah, Ctrl-C untuk stop)"
    exec ssh -o StrictHostKeyChecking=no -R "80:localhost:${PORT}" nokey@localhost.run ;;
  *)
    echo "❌ Mode tidak dikenal: $WHICH"
    echo "   Pakai: auto | cloudflared | ngrok | ssh | lt"
    exit 1 ;;
esac
