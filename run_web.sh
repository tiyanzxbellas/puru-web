#!/bin/sh
# Puru Web UI — jalankan puruweb (web AI chat) dari repo ini
# Termux/Android/Linux/macOS. Binari android HARUS dinilai lokal (CGO=1 biar DNS aman).
set -e
cd "$(dirname "$0")"
PORT="${PORT:-8080}"
ADDR="${ADDR:-127.0.0.1:$PORT}"

if [ ! -f .build/config.json ]; then
  echo "❌ Belum ada config. Jalankan dulu:"
  echo "   mkdir -p .build && cp example.config.json .build/config.json"
  echo "   lalu isi telegram_bot_token + model.base_url + model.api_key"
  exit 1
fi

echo "▶ Build puruweb (CGO default, biar DNS pakai libc sistem)…"
mkdir -p .build
go build -o .build/puruweb ./cmd/puruweb

echo "🦞 puruweb jalan di http://$ADDR (Ctrl-C untuk berhenti)"
exec ./.build/puruweb -config .build/config.json -addr "$ADDR"
