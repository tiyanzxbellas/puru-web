# `tunnel/` — localtunnel gratis, nama bisa dipilih

Bikin Puru Web (`127.0.0.1:18099`) bisa dibuka dari internet lewat
**localtunnel**: gratis, tanpa akun, **tanpa token**, dan subdomain bisa dipilih
sendiri — mis. **`https://puru-ai.loca.lt`**.

Panduan lengkap (interstitial, keamanan, autostart Termux:Boot, bandingkan jalur
lain) ada di [`../TUNNEL-GRATIS.md`](../TUNNEL-GRATIS.md).

## Pakai

```bash
npm i -g localtunnel        # sekali saja

./on.sh                     # nyalain  -> kasih URL + cek HTTP publik
./status.sh                 # PID + URL + HTTP code
./off.sh                    # matiin
```

Nama lain:

```bash
LT_SUB=puruweb ./on.sh
LT_SUB=puru-ai,puruweb,puru,puruai ./on.sh     # daftar cadangan, dicoba berurutan
LT_PORT=3000 ./on.sh                            # kalau server di port lain
```

Server web-nya harus jalan dulu (di terminal lain: `./run_web.sh`).

## Isi folder

| File | Fungsi |
| :--- | :--- |
| `lt-start.cjs` | runner — pakai **modul** localtunnel langsung |
| `on.sh` / `off.sh` / `status.sh` | kontrol tunnel |
| `lt-pid.sh` | cari PID node runner lewat `/proc/*/cmdline` |
| `lt.url`, `lt.log`, `lt.run.log` | URL aktif + log (dibuat otomatis, di-`.gitignore`) |

## Kenapa nggak pakai CLI `lt` biasa?

```
Unsupported platform: android
    at .../localtunnel/node_modules/openurl/openurl.js:54:11
```

CLI resmi manggil modul `openurl` (buat auto-buka browser) saat start, dan modul
itu nggak kenal platform `android` → langsung crash. Runner `lt-start.cjs`
manggil `require('localtunnel')` tanpa bagian itu, plus bisa nulis URL ke file
dan coba beberapa subdomain.

## Batasan

1. Pengunjung **browser** kena halaman peringatan localtunnel dulu — ketik IP
   publik kita (yang ditampilkan di halaman itu) → Continue. Muncul **sekali per
   IP / 7 hari**. Request non-browser (curl/API) tembus langsung, atau pakai
   header `bypass-tunnel-reminder`.
2. Subdomain bisa diambil orang lain saat tunnel kita mati → pakai `LT_SUB`
   sebagai daftar cadangan.
3. `puruweb` **belum ada login**: siapa pun yang punya URL bisa chat, baca
   `downloads/`, upload, dan menjalankan shell di HP. Matikan dengan `./off.sh`
   kalau sudah selesai.
