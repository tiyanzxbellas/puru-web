# 🌐 Tunnel Gratis buat Puru Web — nama bisa dipilih (`puru-ai.loca.lt`)

Panduan ini khusus **localtunnel**: gratis, **tanpa akun, tanpa token, tanpa install binary**,
dan **subdomain bisa kita pilih sendiri**.

Hasil yang sudah jalan di HP Termux ini:

```
https://puru-ai.loca.lt   ->   127.0.0.1:18099   (server Puru Web di HP)
```

Nama `puru-ai` jadi milik kita **selama tunnel hidup**. Kalau tunnel mati, nama itu
kembali bebas dan bisa dipakai orang lain.

---

## 1. Pakai (3 perintah)

```bash
npm i -g localtunnel          # sekali saja (butuh node/npm — di Termux: pkg install nodejs)

./tunnel/on.sh                # nyalain -> nampilin URL + cek HTTP publik
./tunnel/status.sh            # cek PID, URL, HTTP code
./tunnel/off.sh               # matiin
```

Ganti nama (koma = daftar cadangan, dicoba berurutan):

```bash
LT_SUB=puruweb ./tunnel/on.sh        # -> https://puruweb.loca.lt
LT_SUB=puru-ai,puruweb,puru ./tunnel/on.sh
```

Port beda dari 18099:

```bash
LT_PORT=3000 ./tunnel/on.sh
```

> Server Puru Web-nya harus nyala dulu di terminal lain: `./run_web.sh`.

---

## 2. Isi folder `tunnel/`

| File | Fungsi |
| :--- | :--- |
| `lt-start.cjs` | runner — panggil **modul** localtunnel langsung (CLI `lt` crash di Android) |
| `on.sh` | nyalain tunnel, tunggu `lt.url`, langsung tes HTTP publik |
| `off.sh` | matiin proses tunnel |
| `status.sh` | PID + URL + HTTP `/api/info` |
| `lt-pid.sh` | cari PID node runner lewat `/proc/*/cmdline` (`pgrep -f` nggak andal di Termux) |
| `lt.url` / `lt.log` | URL aktif + log (dibuat otomatis, jangan di-commit) |

**Kenapa perlu runner `.cjs`?** CLI resmi `lt` langsung mati di Termux:

```
Unsupported platform: android
    at Object.<anonymous> (.../localtunnel/node_modules/openurl/openurl.js:54:11)
```

Modul `openurl` (buat auto-buka browser) nggak kenal platform `android`, padahal dia
dipanggil saat start. Runner ini manggil `require('localtunnel')` langsung, jadi
nolak masalah itu. Bonus: bisa multi-subdomain + nulis URL ke file.

---

## 3. Halaman peringatan localtunnel (harus tahu)

Pengunjung yang pakai **browser** kena satu halaman dulu sebelum masuk UI Puru:

```
Tunnel website ahead!
This tunnel is hosted by: 146.19.196.71
To continue, enter the IP shown above.     [ ____________ ]  [ Continue ]
```

**Cara lewat:** ketik IP yang ditampilkan (IP publik kita; cek sendiri dengan
`curl -s https://api.ipify.org`) → **Continue** → masuk.

Yang perlu dicatat:

- Muncul **sekali per IP pengunjung, tahan 7 hari** — bukan tiap kali buka.
- Kalau muncul permintaan *"tunnel password"*, isinya **IP publik kita sendiri**
  (angka yang sama).
- Request **non-browser** (curl, API, webhook) **tembus tanpa halaman itu** —
  localtunnel cuma nampilin halaman kalau User-Agent-nya browser sungguhan.
- Program bisa nembus dengan header `bypass-tunnel-reminder: <apa saja>`
  (pernyataan resmi localtunnel).

---

## 4. Bukti verifikasi (dari HP ini)

| Cek | Hasil |
| :--- | :--- |
| `GET https://puru-ai.loca.lt/api/info` | **200** `{"model":"puru",...,"fresh":true}` |
| `GET https://puru-ai.loca.lt/` (non-browser) | **200**, 39.151 byte — byte-identik dengan UI di disk |
| `GET /` pakai User-Agent Chrome | halaman peringatan (sesuai §3) |
| `./tunnel/status.sh` | `JALAN (PID ...)` + `HTTP: 200` |

---

## 5. Nyalain otomatis tiap HP reboot (opsional)

Butuh app **Termux:Boot** (F-Droid). Setelah itu:

```bash
mkdir -p ~/.termux/boot
cat > ~/.termux/boot/99-puru-tunnel.sh <<'BOOT'
#!/data/data/com.termux/files/usr/bin/bash
termux-wake-lock
# tunggu HP siap (wifi/kuota + server web)
sleep 45
cd "$HOME/puru-web"        # sesuaikan folder repo
./run_web.sh >> .build/boot.log 2>&1 &
sleep 20
./tunnel/on.sh >> tunnel/boot.log 2>&1
BOOT
chmod +x ~/.termux/boot/99-puru-tunnel.sh
```

Catatan: set baterai aplikasi Termux ke **Unrestricted** biar Android nggak ngebunuh
prosesnya saat HP nganggur.

---

## 6. Bandingkan jalur lain (biar bisa milih)

| Jalur | Nama | Biaya | Token/akun? | Halaman peringatan? |
| :--- | :--- | :--- | :--- | :--- |
| **localtunnel** (dipakai sekarang) | `puru-ai.loca.lt` | gratis | ✗ | ya — sekali per IP / 7 hari (§3) |
| cloudflared quick | `xxxx.trycloudflare.com` | gratis | ✗ | ✗ |
| SSH localhost.run | `xxxx.lhr.life` | gratis | ✗ | ✗ (paling simpel, tapi sering putus) |
| serveo.net | `puru-ai.serveo.net` | gratis | daftar SSH pubkey | ya |
| ngrok free | `xxxx.ngrok-free.dev` (acak) | gratis | **✓ authtoken** | ya ("Visit Site") |
| ngrok Hobbyist | `puru-ai.ngrok.app` | $10/bln | ✓ | ✗ |
| Cloudflare Tunnel (named) | `puru-ai.tiyan.my.id` | gratis | ✓ akun CF + pindah NS domain | ✗ |

Cara ambil **authtoken ngrok** (kalau mau jalur ngrok):
`<https://dashboard.ngrok.com/signup>` → daftar → salin dari
`<https://dashboard.ngrok.com/get-started/your-authtoken>` → `ngrok config add-authtoken <TOKEN>`.
Detail + fix DNS `proot` di Termux ada di [`TERMUX-DEPLOY.md`](TERMUX-DEPLOY.md) §2.

---

## 7. Keamanan — jangan dilewat

`puruweb` **belum ada login**. Siapa pun yang dapat URL-nya bisa:

- ngobrol pakai kuota model kamu,
- baca semua file di `workspace/downloads/`,
- upload file,
- menyuruh agent **menjalankan perintah shell di HP ini**.

Selagi belum ada auth: bagikan cuma ke orang yang dipercaya, dan matikan tunnel
(`./tunnel/off.sh`) kalau sudah selesai. Opsi nambah auth ada di
[`WEB-HOSTING.md`](WEB-HOSTING.md) §5.

---

*Terakhir diupdate: 2026-10-19 — PuruClaw 🦞*
