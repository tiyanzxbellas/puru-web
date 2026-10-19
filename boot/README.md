# `boot/` — autostart Puru Web saat HP nyala (Termux:Boot)

Isi folder ini **cuma nambah**, tidak mengubah/menghapus file lain di repo.

| File | Fungsi |
| :--- | :--- |
| `99-puru-web.sh` | skrip yang dinyalakan otomatis: `puruweb` (port **18099**) + tunnel **localtunnel** |

## Pasang (sekali saja)

1. Install **Termux:Boot** dari **F-Droid** (bukan Play Store):
   <https://f-droid.org/packages/com.termux.boot/>
   → buka app-nya **1x** (klik ikonnya) biar Android mengizinkan autostart.
2. Daftarkan skrip:

   ```sh
   mkdir -p ~/.termux/boot
   cat > ~/.termux/boot/99-puru-web.sh <<'EOF'
   #!/data/data/com.termux/files/usr/bin/sh
   exec "$HOME/project source code/puruweb-run/boot/99-puru-web.sh"
   EOF
   chmod +x ~/.termux/boot/99-puru-web.sh
   ```

3. Baterai: **Settings → Apps → Termux → Battery → Unrestricted**
   (biar Android tidak membekukan Termux).

Selesai. Tiap HP selesai booting, tunggu ~30 detik → web + tunnel hidup sendiri.

## Cek & debug

```sh
cat "$HOME/project source code/puruweb-run/boot.log"   # jalankan/lewati apa
./tunnel/status.sh                                     # status tunnel + URL
pkill -f 'puruweb -config' 2>/dev/null                 # (hati-hati) matikan web
```

Ubah lokasi repo: set `PURU_REPO_DIR` di launcher. Ubah port: set `PORT`.

## Catatan

- Ini **bukan** "Termux kebuka di layar" — Termux jalan di background.
- Cuma terpicu saat HP **booting**. Matikan web/tunnel manual → tetap mati sampai dinyalakan lagi.
- Kalau app Termux:Boot belum dibuka 1x, skrip **tidak** dijalankan.
