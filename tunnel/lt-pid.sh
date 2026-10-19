#!/data/data/com.termux/files/usr/bin/bash
# Cari PID runner localtunnel (lt-start.cjs).
#   1) utamakan pidfile (paling akurat)
#   2) fallback: scan /proc/*/cmdline (tanpa pgrep -f)
# CATATAN: `pgrep -f lt-start.cjs` / pola scan bebas bisa "menemukan" shell
# pemanggilnya sendiri (command line-nya memuat nama file itu) -> shell-nya
# bunuh diri. Karena itu PID sendiri + induknya di-skip, dan pidfile dipakai
# lebih dulu.
cd "$(dirname "$0")" || exit 0

if [ -f lt.pid ]; then
  p=$(cat lt.pid 2>/dev/null)
  if [ -n "$p" ] && [ -r "/proc/$p/cmdline" ] && \
     tr '\0' ' ' < "/proc/$p/cmdline" 2>/dev/null | grep -q "lt-start.cjs"; then
    echo "$p"; exit 0
  fi
fi

for d in /proc/[0-9]*; do
  p=${d#/proc/}
  [ "$p" = "$$" ] && continue
  [ "$p" = "$PPID" ] && continue
  [ -r "$d/cmdline" ] || continue
  c=$(tr '\0' ' ' < "$d/cmdline" 2>/dev/null)
  case "$c" in
    *node*lt-start.cjs*) echo "$p" ;;
  esac
done
