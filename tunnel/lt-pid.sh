#!/data/data/com.termux/files/usr/bin/bash
# cari PID node yang jalanin lt-start.cjs (pgrep -f tidak bisa diandalkan di Termux)
for d in /proc/[0-9]*; do
  [ -r "$d/cmdline" ] || continue
  c=$(tr '\0' ' ' < "$d/cmdline" 2>/dev/null)
  case "$c" in *lt-start.cjs*) echo "${d#/proc/}";; esac
done
