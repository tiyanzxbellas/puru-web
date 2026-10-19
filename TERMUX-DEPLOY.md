# 🌐 Deploy Puru Web di Termux (localhost) + Akses Publik pakai ngrok

Panduan singkat & **teruji di HP Android/Termux** buat dua hal:

1. **Jalanin web-nya lokal di Termux** → `http://127.0.0.1:18099` (dibuka dari
   browser HP yang sama).
2. **Deploy/buka ke internet pakai ngrok** → `https://xxxx.ngrok-free.app`.

> ⚠️ **Web Puru belum ada login.** Siapa pun yang dapat URL-nya bisa ngobrol
> (pakai kuota model kamu), baca `downloads/`, upload file, dan menyuruh agent
> jalanin perintah shell di HP. Bagikan cuma ke orang yang dipercaya, dan
> matikan tunnel (Ctrl-C) kalau sudah selesai. Lihat juga `WEB-HOSTING.md`.

---

## 📦 0. Prasyarat

```bash
pkg update -y && pkg install -y git go proot curl
```
- `go` → buat build `puruweb`.
- `proot` → **wajib buat ngrok di Termux** (lihat §3 kenapa).

Clone repo (kalau belum ada):

```bash
cd ~
git clone https://github.com/tiyanzxbellas/puru-web.git
cd puru-web
mkdir -p .build && cp example.config.json .build/config.json
nano .build/config.json      # isi model.base_url, model.api_key, telegram_bot_token (boleh dummy)
```

Build sekali (di HP kentang jalankan detached biar gak kena guard RAM):

```bash
(setsid nohup env GOGC=30 GOFLAGS=-p=1 go build -o .build/puruweb ./cmd/puruweb > .build/build.log 2>&1 &)
tail -f .build/build.log     # tunggu sampai selesai (~1-2 menit), lalu Ctrl-C
```

---

## 1. 🖥️ Jalanin di localhost (`http://127.0.0.1:18099`)

**Terminal 1:**

```bash
cd ~/puru-web
./run_web.sh                 # default listen 127.0.0.1:18099
# atau manual:
./.build/puruweb -config .build/config.json -addr 127.0.0.1:18099
```

Buka browser **di HP yang sama**:

```
http://127.0.0.1:18099
```

Cek sehat tanpa browser:

```bash
curl -s http://127.0.0.1:18099/api/info
# {"model":"puru","workspace":"...","chat_id":9000000001,"running":false,"fresh":true,...}
```

| Alamat | Siapa yang bisa buka | Catatan |
| :--- | :--- | :--- |
| `http://127.0.0.1:18099` | **HP ini sendiri** | default, paling aman |
| `http://localhost:18099` | HP ini sendiri | sama saja |
| `http://<IP-LAN-HP>:18099` | HP/laptop lain di **Wi-Fi yang sama** | perlu `-addr 0.0.0.0:18099` (lihat di bawah) |
| `https://xxxx.ngrok-free.app` | **siapa pun di internet** | pakai ngrok (§2) |

**Biar bisa dibuka dari device lain di Wi-Fi yang sama:**

```bash
ADDR=0.0.0.0:18099 ./run_web.sh
# cari IP HP:  ifconfig 2>/dev/null || ip addr        (biasanya 192.168.x.x)
# buka dari HP lain:  http://192.168.x.x:18099
```
> Kalau HP pakai **paket data**, IP-nya bukan 192.168.x.x tapi range CGNAT
> (`10.x` / `169.254.x`) → LAN sharing kemungkinan gagal. Pakai tunnel.

**Matiin server:** `Ctrl-C` di terminalnya, atau:

```bash
for d in /proc/[0-9]*; do [ "$(cat $d/comm 2>/dev/null)" = "puruweb" ] && kill ${d#/proc/}; done
```
> `pgrep -x puruweb` sering **gak match** di Termux — pakai loop `/proc` di atas.

---

## 2. 🚇 Deploy ke internet pakai ngrok

### 2a. Daftar & ambil authtoken (sekali saja)
1. Buka <https://dashboard.ngrok.com/signup> → daftar **gratis** (email/Google).
2. Salin authtoken dari <https://dashboard.ngrok.com/get-started/your-authtoken>
   (bentuknya `2abc...xyz`, ±49 karakter).

### 2b. Install ngrok di Termux (ARM64)
Termux **tidak punya paket `ngrok`**, jadi ambil binary resminya:

```bash
cd ~
curl -LO https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-linux-arm64.tgz
tar xzf ngrok-v3-stable-linux-arm64.tgz          # keluar binary: ngrok (±30 MB)
mkdir -p ~/bin && mv ngrok ~/bin/ && chmod +x ~/bin/ngrok
export PATH="$HOME/bin:$PATH"                    # tambahkan ke ~/.bashrc biar permanen
ngrok version                                    # -> ngrok version 3.39.11
```
> Diuji di HP ARM64 (aarch64), Android 13 → binary-nya **jalan** (`ngrok version` OK).
> Kalau HP 32-bit pakai `...-linux-arm.tgz`.

### 2c. Simpan authtoken

```bash
ngrok config add-authtoken 2xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

### 2d. ⚠️ FIX WAJIB: ngrok butuh `proot` di Termux

**Gejala kalau langsung dijalankan:**

```
failed to reconnect session ... err="failed to dial ngrok server with address
\"connect.ngrok-agent.com:443\": dial tcp: lookup connect.ngrok-agent.com on
[::1]:53: read udp [::1]:...->[::1]:53: read: connection refused"
failed to check for update ... x509: certificate signed by unknown authority
```

**Sebabnya (bukan salah authtoken kamu):**
- ngrok itu binary **Go statis** → pakai *resolver Go sendiri*, yang baca
  `/etc/resolv.conf`.
- Di Android `/etc` itu symlink ke `/system/etc` dan **read-only**, plus
  **gak ada** `/etc/resolv.conf` → resolver Go jatuh ke fallback
  `127.0.0.1:53` / `[::1]:53` → **connection refused**.
- Sama juga CA: path CA yang dicari Go (`/etc/ssl/certs/ca-certificates.crt`)
  gak ada di Termux → TLS gagal verify. (Termux naruh CA di
  `$PREFIX/etc/tls/cert.pem`.)

**Solusi: bungkus ngrok pakai `proot`** yang nge-*bind* file `resolv.conf` (dan
CA) ke path yang dicari Go — **tanpa root**. Skripnya disediain di repo:
[`ngrok-termux.sh`](ngrok-termux.sh).

```bash
# Terminal 2 (terminal 1 tetap jalanin ./run_web.sh)
cd ~/puru-web
./ngrok-termux.sh http 18099
```

Output:

```
✔ resolv.conf → /etc/resolv.conf   (DNS 1.1.1.1)
✔ CA bundle   → /etc/ssl/certs/ca-certificates.crt
▶ ngrok http 18099
Forwarding  https://xxxx-xx-xx.ngrok-free.app -> http://localhost:18099
```

URL `https://....ngrok-free.app` itu yang dibagikan. **Ctrl-C** buat matiin.

**Bukti fix-nya bekerja** (dari HP ini, authtoken dummy sengaja dipakai):

| Percobaan | Hasil |
| :--- | :--- |
| `./ngrok http 18099` (langsung) | ❌ `failed to dial ... [::1]:53: connection refused` |
| `./ngrok-termux.sh http 18099` | ✅ tersambung ke server ngrok (cuma berhenti di validasi authtoken karena dummy) |

### 2e. Cek & kontrol

```bash
# dashboard inspector lokal (request yang masuk, replay, dll)
# buka di browser HP:  http://127.0.0.1:4040
curl -s http://127.0.0.1:4040/api/tunnels
```

### 2f. URL tetap (static domain) — gratis 1 per akun
Ngrok gratis memberi **1 static domain**. Bikin di
<https://dashboard.ngrok.com/domains>, lalu:

```bash
./ngrok-termux.sh http 18099 --url https://nama-kamu.ngrok-free.app
```

### 2g. Jalan terus (24 jam, HP tetap nyala)
```bash
pkg install -y tmux
tmux new -s puru
# di dalam tmux: ./run_web.sh        (terminal kiri: Ctrl-B lalu % buat split)
#                ./ngrok-termux.sh http 18099
```
Tambahkan `termux-wake-lock` supaya Android gak ngebunuh prosesnya, dan set
baterai Termux ke **Unrestricted**.

---

## 3. 🧩 Struktur perintah ringkas

```bash
# ── Sekali saja ─────────────────────────────────────────────
pkg install -y git go proot
git clone https://github.com/tiyanzxbellas/puru-web.git ~/puru-web
cd ~/puru-web && mkdir -p .build && cp example.config.json .build/config.json
(setsid nohup env GOGC=30 GOFLAGS=-p=1 go build -o .build/puruweb ./cmd/puruweb > .build/build.log 2>&1 &)

curl -LO https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-linux-arm64.tgz
tar xzf ngrok-v3-stable-linux-arm64.tgz && mkdir -p ~/bin && mv ngrok ~/bin/ && chmod +x ~/bin/ngrok
ngrok config add-authtoken <AUTHTOKEN_KAMU>

# ── Tiap mau pakai ─────────────────────────────────────────
cd ~/puru-web
./run_web.sh                  # terminal 1 → http://127.0.0.1:18099
./ngrok-termux.sh http 18099  # terminal 2 → https://xxxx.ngrok-free.app
```

---

## 4. 🩺 Troubleshooting

| Gejala | Sebab | Solusi |
| :--- | :--- | :--- |
| `lookup connect.ngrok-agent.com on [::1]:53: connection refused` | resolver Go statis gak punya `/etc/resolv.conf` | jalankan lewat `./ngrok-termux.sh` (proot), bukan `ngrok` langsung |
| `x509: certificate signed by unknown authority` | CA Termux beda path | sudah dihandle `ngrok-termux.sh` (bind CA); manual: `export SSL_CERT_FILE=$PREFIX/etc/tls/cert.pem` |
| `ERR_NGROK_105` — "does not look like a proper authtoken" | token salah/kepotong/salah salin | salin ulang dari dashboard → `ngrok config add-authtoken ...` |
| `ERR_NGROK_4018` — authentication failed | authtoken dicabut / akun beda | bikin token baru di dashboard |
| `ERR_NGROK_108` — hanya boleh 1 sesi | akun gratis = 1 agent jalan | matikan ngrok lain dulu (`Ctrl-C` / `kill`) |
| `bind: address already in use` di port 4040 | ada ngrok lama masih hidup (akun gratis = 1 agent) | matikan ngrok lama: `for d in /proc/[0-9]*; do [ "$(cat $d/comm 2>/dev/null)" = "ngrok" ] && kill ${d#/proc/}; done` |
| `ngrok: not found` | `~/bin` belum masuk `PATH` | `export PATH="$HOME/bin:$PATH"` (taruh di `~/.bashrc`) |
| Server web gak bisa dibuka dari luar | `puruweb` listen di `127.0.0.1` doang | normal; tunnel ngrok tetap jalan (dia nyambung ke localhost). Buat LAN: `ADDR=0.0.0.0:18099 ./run_web.sh` |
| `proot: not found` | belum install | `pkg install -y proot` |
| Web diakses orang asing / kuota model kepakai | **belum ada login** | matiin tunnel; lihat `WEB-HOSTING.md` §Keamanan buat nambah auth |

---

## 5. 🔄 Alternatif tanpa proot & tanpa akun

Kalau cuma butuh "bisa dibuka dari HP lain sebentar" dan gak mau install
ngrok/proot:

```bash
./expose.sh ssh        # SSH tunnel localhost.run — nol install, nol akun
# atau
pkg install -y cloudflared && ./expose.sh cloudflared
# atau — localtunnel, gratis + nama bisa dipilih (dipakai sekarang)
npm i -g localtunnel && ./tunnel/on.sh      # -> https://puru-ai.loca.lt
```

Detail semua opsi (LAN, VPS, Docker, kenapa Vercel **tidak bisa**) ada di
[`WEB-HOSTING.md`](WEB-HOSTING.md).
