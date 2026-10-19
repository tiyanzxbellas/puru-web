#!/bin/sh
# ngrok-termux.sh — jalankan ngrok agent di Termux/Android tanpa root.
#
# KENAPA PERLU INI?
#   ngrok = binary Go statis -> pakai resolver Go sendiri, yang baca
#   /etc/resolv.conf. Di Android, /etc itu symlink ke /system/etc dan
#   READ-ONLY, plus TIDAK ada /etc/resolv.conf -> resolver jatuh ke
#   fallback 127.0.0.1:53 / [::1]:53 -> "connection refused".
#   Hal yang sama untuk CA: Go nyari /etc/ssl/certs/ca-certificates.crt
#   yang gak ada di Termux (CA-nya ada di $PREFIX/etc/tls/cert.pem).
#   Skrip ini nge-bind resolv.conf + CA ke path yang dicari pakai `proot`
#   (tanpa root), lalu meneruskan semua argumen ke ngrok.
#
# PAKAI:
#   ./ngrok-termux.sh http 18099
#   ./ngrok-termux.sh http 18099 --url https://nama.ngrok-free.app
#   NGROK_DNS=8.8.8.8 ./ngrok-termux.sh http 18099
#
# ENV:
#   NGROK_DNS         DNS server (default 1.1.1.1)
#   NGROK_BIN         path binary ngrok (default: `ngrok` dari PATH)
#   NGROK_TERMUX_DIR  folder kerja (default $HOME/.ngrok-termux)
set -e

PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
NGROK_DNS="${NGROK_DNS:-1.1.1.1}"
WORK="${NGROK_TERMUX_DIR:-$HOME/.ngrok-termux}"

NGROK_BIN="${NGROK_BIN:-$(command -v ngrok 2>/dev/null || true)}"
if [ -z "$NGROK_BIN" ]; then
  echo "❌ ngrok tidak ditemukan di PATH."
  echo "   Install dulu (lihat TERMUX-DEPLOY.md §2b):"
  echo "     curl -LO https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-linux-arm64.tgz"
  echo "     tar xzf ngrok-v3-stable-linux-arm64.tgz"
  echo "     mkdir -p ~/bin && mv ngrok ~/bin/ && chmod +x ~/bin/ngrok"
  echo "     export PATH=\"\$HOME/bin:\$PATH\"   # taruh juga di ~/.bashrc"
  exit 1
fi

if ! command -v proot >/dev/null 2>&1; then
  echo "❌ proot belum ada.  Install:  pkg install -y proot"
  exit 1
fi

mkdir -p "$WORK/etc/ssl/certs"

# 1) resolv.conf (Android gak punya; /etc read-only -> di-bind pakai proot)
printf 'nameserver %s\nnameserver 8.8.8.8\n' "$NGROK_DNS" > "$WORK/resolv.conf"
echo "✔ resolv.conf → /etc/resolv.conf   (DNS $NGROK_DNS)"

# 2) CA bundle (Go nyari di /etc/ssl/certs/ca-certificates.crt)
CA="$WORK/etc/ssl/certs/ca-certificates.crt"
if [ ! -s "$CA" ]; then
  for c in "$PREFIX/etc/tls/cert.pem" "$PREFIX/etc/ssl/certs/ca-certificates.crt"; do
    [ -s "$c" ] && cp -f "$c" "$CA" && break
  done
fi
if [ -s "$CA" ]; then
  echo "✔ CA bundle   → /etc/ssl/certs/ca-certificates.crt"
fi

# 3) jalanin ngrok di dalam proot (bind dua file di atas)
echo "▶ ngrok $*"
exec proot \
  -b "$WORK/resolv.conf:/etc/resolv.conf" \
  -b "$CA:/etc/ssl/certs/ca-certificates.crt" \
  "$NGROK_BIN" "$@"
