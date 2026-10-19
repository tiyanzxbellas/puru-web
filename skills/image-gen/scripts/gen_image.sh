#!/data/data/com.termux/files/usr/bin/sh
# gen_image.sh — text-to-image gratis (Pollinations, tanpa API key)
#
# Pemakaian:
#   sh gen_image.sh "prompt deskripsi" [outfile]
#   sh gen_image.sh --size 768 --model flux --seed 42 "prompt" out.jpg
#
# Output: path file JPEG (absolut) di baris terakhir stdout.
set -u

SIZE=1024
MODEL=flux
SEED=""
while [ $# -gt 0 ]; do
  case "$1" in
    --size) SIZE="$2"; shift 2 ;;
    --model) MODEL="$2"; shift 2 ;;
    --seed) SEED="$2"; shift 2 ;;
    --) shift; break ;;
    -h|--help)
      sed -n '2,9p' "$0"; exit 0 ;;
    *) break ;;
  esac
done

PROMPT="${1:-}"
OUT="${2:-}"
if [ -z "$PROMPT" ]; then
  echo "usage: sh gen_image.sh [--size N] [--model M] [--seed S] \"prompt\" [outfile]" >&2
  exit 2
fi

WS="${PURU_WORKSPACE:-/data/data/com.termux/files/home/.puru/workspace}"
if [ -z "$OUT" ]; then
  mkdir -p "$WS/downloads"
  OUT="$WS/downloads/img-$(date +%Y%m%d-%H%M%S).jpg"
fi
case "$OUT" in
  /*) : ;;
  *) OUT="$PWD/$OUT" ;;
esac

ENC=$(python3 -c 'import sys,urllib.parse;print(urllib.parse.quote(sys.argv[1]))' "$PROMPT")
URL="https://image.pollinations.ai/prompt/$ENC?width=$SIZE&height=$SIZE&nologo=true&model=$MODEL"
[ -n "$SEED" ] && URL="$URL&seed=$SEED"

i=1
while [ $i -le 4 ]; do
  code=$(curl -sL --max-time 180 -o "$OUT.tmp" -w '%{http_code}' "$URL")
  case "$code" in
    200)
      head -c 2 "$OUT.tmp" >/dev/null 2>&1
      if [ "$(head -c 2 "$OUT.tmp" | od -An -tx1 | tr -d ' ')" = "ffd8" ]; then
        mv "$OUT.tmp" "$OUT"
        ls -l "$OUT" >&2
        echo "$OUT"
        exit 0
      fi
      echo "gen_image: respons 200 tapi bukan JPEG" >&2
      ;;
    402)
      echo "gen_image: kena limit gratis (1 gambar/45s) — tunggu 45 detik…" >&2
      sleep 45 ;;
    *)
      echo "gen_image: HTTP $code (percobaan $i) — coba lagi…" >&2
      sleep 8 ;;
  esac
  i=$((i + 1))
done

echo "gen_image: GAGAL setelah 4 percobaan (prompt: $PROMPT)" >&2
rm -f "$OUT.tmp"
exit 1
