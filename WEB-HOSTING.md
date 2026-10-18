# 🌍 Hosting & Akses Publik — Puru Web (`puruweb`)

Panduan ini ngejawab satu pertanyaan: **gimana biar web Puru (`puruweb`) bisa
dibuka dari HP/browser lain atau dari internet?**

> Status: repo ini sebelumnya cuma punya cara jalan **lokal**. Dokumen ini
> yang ngisi bagian itu. Semua perintah di bawah udah disesuaikan buat
> **Termux/Android** (tapi jalan juga di Linux/macOS).

---

## 0. Ringkasan pilihan

| # | Cara | Perlu apa | Orang lain bisa lihat? | Dari internet? | Catatan |
| :-: | :--- | :--- | :--: | :--: | :--- |
| 1 | Lokal `127.0.0.1:18099` | — | ❌ | ❌ | default sekarang, cuma HP sendiri |
| 2 | LAN `0.0.0.0:18099` | IP lokal HP + Wi-Fi sama | ✅ | ❌ | gagal kalau jaringan paket data (CGNAT) |
| 3 | Tunnel **cloudflared** (quick) | `pkg install cloudflared` | ✅ | ✅ | gratis, **tanpa akun**, URL random |
| 4 | Tunnel **ngrok** | akun ngrok + authtoken | ✅ | ✅ | gratis (1 agent), URL random |
| 5 | Tunnel **SSH** (localhost.run / serveo) | `ssh` (udah ada) | ✅ | ✅ | **nol install**, nol akun |
| 6 | **localtunnel** (`lt`) | `npm i -g localtunnel` | ✅ | ✅ | nol akun |
| 7 | **VPS** + reverse proxy | VPS + domain | ✅ | ✅ | 24/7, HP boleh mati |
| 8 | **Docker** (VPS / HF Space) | Docker | ✅ | ✅ | sama seperti #7 |
| 9 | **Vercel** | — | — | — | ❌ **NGGAK BISA** → lihat §7 |

**Pendek kata:** kalau cuma mau *tes dari HP lain* → pakai §3 (cloudflared).
Kalau mau *online 24 jam* → pakai §6 (VPS/Docker), bukan Vercel.

---

## ⚠️ BACA DULU — web ini belum ada login

`puruweb` **TIDAK PUNYA autentikasi apa pun** (nggak ada password, token,
maupun session). Siapa pun yang dapat URL-nya bisa:

- ngobrol pakai kuota model mas (API key nempel di server),
- **baca semua file** di `~/.puru/workspace/downloads/` (`/api/files`,
  `/api/file?path=...`),
- upload file ke HP mas (`/api/upload`),
- dan yang paling bahaya: **nyuruh AI ngejalanin perintah shell di HP mas**
  (`run_shell_command` → `rm`, `curl`, `git push`, apa pun).

Jadi:

- **Jangan** bagikan URL tunnel ke publik / grup / sosmed.
- Pakai tunnel cuma buat **tes sendiri**, lalu matikan (`Ctrl-C`).
- Kalau mau dipakai serius → **pasang auth dulu** (§5) baru di-expose.
- Set `restrict_workspace: true` di config (sudah default di
  `.build/config.json`) minimal membatasi file ke folder workspace.

---

## 1. Lokal (default sekarang)

```bash
cd "~/project source code/puruweb-run"
./run_web.sh                 # default 127.0.0.1:18099
# atau manual:
./.build/puruweb -config .build/config.json -addr 127.0.0.1:18099
```

Buka `http://127.0.0.1:18099` dari browser **HP mas sendiri**.
`127.0.0.1` itu *loopback* = alamat diri sendiri, jadi orang lain **nggak
bisa** walau satu Wi-Fi.

---

## 2. LAN (satu Wi-Fi, tanpa tunnel)

Jalankan server bind ke semua interface:

```bash
ADDR=0.0.0.0:18099 ./run_web.sh
# atau: ./.build/puruweb -config .build/config.json -addr 0.0.0.0:18099
```

Cari IP lokal HP:

```bash
ifconfig | grep "inet "        # contoh: 10.145.103.160
```

Orang lain (Wi-Fi sama) buka `http://<IP-LOKAL>:18099`.

**Syarat & jebakan:**

- HP & pengunjung harus **satu jaringan** yang sama.
- Kalau IP-nya `10.x.x.x` / `100.64.x.x` dan itu **paket data seluler**,
  biasanya **CGNAT** → Wi-Fi sebelah nggak bisa akses. Cek: kalau
  jaringan seluler, lupakan cara ini.
- Termux harus tetap jalan; layar boleh mati tapi aktifkan
  `termux-wake-lock` biar nggak di-kill Android.

---

## 3. Tunnel cloudflared (⭐ rekomendasi — gratis, tanpa akun)

Paket `cloudflared` **tersedia di repo Termux** (candidate `2026.10.1`).

```bash
pkg install -y cloudflared

# server lokal harus jalan dulu (terminal lain):
./run_web.sh

# lalu bikin tunnel:
cloudflared tunnel --url http://127.0.0.1:18099
```

Output-nya nampilin URL random seperti:

```
+--------------------------------------------------------------------------------------------+
|  Your quick Tunnel has been created! Visit it at (it may take some time to be reachable):  |
|  https://kata-random-panjang.trycloudflare.com                                             |
+--------------------------------------------------------------------------------------------+
```

- URL `https://*.trycloudflare.com` itu yang dibagi. **URL berubah tiap
  dijalankan ulang** (quick tunnel nggak ada akun/domain tetap).
- Tunnel mati begitu terminal di-`Ctrl-C`.
- Jalanin di **tmux/screen** kalau mau tetap hidup:
  `tmux new -s web 'cloudflared tunnel --url http://127.0.0.1:18099'`.
- Mau URL tetap + domain sendiri → butuh akun Cloudflare + named tunnel
  (`cloudflared tunnel login/create/route`). Itu urusan lanjutan.

**Helper:** `./expose.sh cloudflared` (lihat §8) udah nyiapin semua ini.

---

## 4. Tunnel lain

### 4a. ngrok

`ngrok` **tidak ada** di repo Termux dan **tidak terpasang** di HP ini.
Sekarang ngrok wajib pakai akun + authtoken (gratis, tapi ada limit).

```bash
# arm64 (Android/Termux biasanya kompatibel; kalau gagal, pakai npm/pip)
curl -LO https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-linux-arm64.tgz
tar -xzf ngrok-v3-stable-linux-arm64.tgz -C $PREFIX/bin

# authtoken dari dashboard ngrok
ngrok config add-authtoken <TOKEN_MAS>

./run_web.sh                        # terminal 1
ngrok http 18099                    # terminal 2  -> https://xxxx.ngrok-free.app
```

Catatan: ngrok free sering nampilin halaman warning ("Visit Site") sebelum
masuk ke app, dan URL-nya random tiap start.

### 4b. SSH tunnel — nol install, nol akun

`ssh` udah ada di Termux (openssh `10.6p1`):

```bash
./run_web.sh                                        # terminal 1

ssh -o StrictHostKeyChecking=no -R 80:localhost:18099 nokey@localhost.run
# atau
ssh -o StrictHostKeyChecking=no -R 80:localhost:18099 serveo.net
```

URL-nya muncul di output SSH (mis. `https://xxxx.loca.lt`). Kelebihan:
nggak perlu install apa pun. Kekurangan: kadang lambat / sering putus.

### 4c. localtunnel (`lt`)

```bash
npm i -g localtunnel
lt --port 18099
# -> https://xxxx.loca.lt
```

`lt` kadang minta "tunnel password" (biasanya = IP publik HP mas); kalau
diminta, muncul di halaman itu sendiri.

---

## 5. Keamanan (WAJIB sebelum di-expose)

Checklist minimal kalau web ini mau diakses lebih dari diri sendiri:

1. **Pasang login.** Belum ada di kode. Opsi paling cepat:
   - **Cloudflare Access** (kalau lewat cloudflared named tunnel) — tinggal
     set policy email/OTP, nggak perlu ubah kode Go.
   - **Reverse proxy + basic auth** (nginx/Caddy) — lihat §6.
   - **Patch kode**: tambah middleware cek header/query token di
     `internal/webui/webui.go` sebelum semua handler.

   Kerangka patch minimal:
```go
func (s *Server) authOK(r *http.Request) bool {
    tok := s.cfg.WebToken // isi dari config/env
    if tok == "" { return true }
    if r.Header.Get("Authorization") == "Bearer "+tok { return true }
    return r.URL.Query().Get("token") == tok
}
// di handler: if !s.authOK(r) { http.Error(w, "unauthorized", 401); return }
```

2. **Jangan set `restrict_workspace: false`.** Biarin `true` biar tool file
   nggak bisa keluar dari `~/.puru/workspace`.

3. **Batasi tool shell** kalau bisa (hapus/disabled `run_shell_command`) —
   ini yang paling berisiko kalau web ter-expose.

4. **Matikan tunnel** kalau nggak dipakai. Ini bukan cuma soal orang lain —
   kuota model mas juga jalan tiap orang ngobrol.

5. Ganti `api_key` di `.build/config.json` kalau pernah bocor/commit.

---

## 6. Hosting 24/7 (HP boleh mati)

### 6a. VPS (paling lurus)

```bash
# di VPS Ubuntu/Debian (arsitektur umumnya amd64)
git clone https://github.com/tiyanzxbellas/puru-web.git
cd puru-web
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o puruweb ./cmd/puruweb
mkdir -p .build && cp example.config.json .build/config.json
nano .build/config.json     # isi base_url, api_key, workspace: /root/.puru/workspace
```

`systemd` unit (`/etc/systemd/system/puruweb.service`):

```ini
[Unit]
Description=Puru Web UI
After=network-online.target

[Service]
WorkingDirectory=/opt/puru-web
ExecStart=/opt/puru-web/puruweb -config /opt/puru-web/.build/config.json -addr 127.0.0.1:18099
Restart=always
RestartSec=5
Environment=GOMEMLIMIT=512MiB

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload && systemctl enable --now puruweb
```

Reverse proxy + HTTPS + **basic auth** (Caddy paling gampang):

```
# /etc/caddy/Caddyfile
web.domainku.com {
    basicauth {
        mas $2a$14$HASH_BCRYPT_DARI_caddy_hash-password
    }
    reverse_proxy 127.0.0.1:18099
}
```

Lalu `caddy_hash-password` buat bikin hash, dan Caddy otomatis urus
sertifikat HTTPS (Let's Encrypt).

### 6b. Docker

Repo ini punya `Dockerfile`, tapi **buat `puru-ai` (gateway Telegram)** —
bukan `puruweb`. Buat web, pakai Dockerfile kecil ini:

```dockerfile
# Dockerfile.puruweb
FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /puruweb ./cmd/puruweb

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata tini
WORKDIR /app
COPY --from=build /puruweb /app/puruweb
COPY --from=build /app/example.config.json /app/example.config.json
VOLUME ["/root/.puru"]
ENV GOMEMLIMIT=512MiB
EXPOSE 8080
ENTRYPOINT ["/sbin/tini","--"]
CMD ["/app/puruweb","-config","/root/.puru/config.json","-addr","0.0.0.0:8080"]
```

```bash
docker build -f Dockerfile.puruweb -t puruweb .
docker run -d --name puruweb -p 8080:8080 -v puru-data:/root/.puru puruweb
```

> ⚠️ Di dalam container, `workspace` di config harus nunjuk ke
> `/root/.puru/workspace` (atau path yang di-mount), **bukan** path Termux
> `/data/data/com.termux/...`.

### 6c. Hugging Face Space (Docker SDK)

Bisa, tapi perlu penyesuaian:

- bikin Space tipe **Docker**, taruh `Dockerfile.puruweb` di atas sebagai
  `Dockerfile`,
- port wajib **7860** → `CMD [...,"-addr","0.0.0.0:7860"]`,
- simpan data di `/data` (persisten) supaya history & workspace nggak hilang
  tiap rebuild,
- Space gratis bisa *sleep* kalau nggak dipakai, dan tetap **tanpa auth** —
  pasang token dulu.

### 6d. Fly.io / Railway / Render

Sama polanya: butuh Dockerfile (§6b), volume buat `/root/.puru`, dan set
`PORT`/`-addr` sesuai yang diminta platform. Yang gratis biasanya punya
keterbatasan storage/tidur otomatis.

---

## 7. Kenapa Vercel NGGAK BISA

| Alasan | Penjelasan |
| :--- | :--- |
| Repo belum disiapkan | nggak ada `vercel.json`, nggak ada folder `api/` |
| `puruweb` bukan situs statis | dia **server Go** yang nyimpen state |
| Butuh filesystem persisten | `~/.puru/workspace`, riwayat chat `~/.puru/history`, `downloads/`, `MEMORY.md` — di Vercel FS read-only + `/tmp` ephemeral |
| Butuh proses luar | agent manggil **yt-dlp**, **ffmpeg**, tesseract, dll — nggak ada di runtime Vercel |
| Durasi jawaban panjang | Vercel Hobby ~60 detik; Puru bisa jalan sampai **60 menit** per jawaban |
| Login | nggak ada — mau deploy ke mana pun wajib dipasang dulu |

Yang **mungkin** cuma bentuk *hybrid*: UI statis di Vercel + backend tetap
di HP/VPS lewat tunnel. Tapi tetap butuh backend-nya nyala.

---

## 8. Helper: `expose.sh`

Sekalian gua sediain script biar nggak ngetik panjang:

```bash
./expose.sh                # auto: cloudflared -> ngrok -> lt -> ssh
./expose.sh cloudflared    # paksa cloudflared
./expose.sh ngrok
./expose.sh ssh            # localhost.run
./expose.sh lt
PORT=18099 ./expose.sh cloudflared
```

Script-nya ngecek server lokal hidup dulu, baru nyalain tunnel, dan
nampilin peringatan "no login" sebelum URL dibagi.

---

## 9. Troubleshooting

| Gejala | Sebab | Solusi |
| :--- | :--- | :--- |
| `bind: address already in use` | server lama masih jalan | kill by PID: `for d in /proc/[0-9]*; do [ "$(cat $d/comm)" = puruweb ] && kill ${d#/proc/}; done` |
| `Text file busy` saat ganti binary | binary-nya lagi jalan | kill proses dulu, baru `cp` |
| `memory_limited: true` saat `go build` | guard RAM sesi Termux | build detached: `(setsid nohup env GOGC=30 GOFLAGS=-p=1 go build -o .build/puruweb ./cmd/puruweb > .build/build.log 2>&1 &)` |
| DNS gagal / `no such host` di binary | binary linux-amd64 dipakai di Android | **build lokal** dengan `CGO_ENABLED=1` (bionic) — `run_web.sh` sudah begitu |
| Tunnel URL 502/1033 | server lokal mati / port beda | pastikan `./run_web.sh` jalan di port yang sama |
| `pgrep -x puruweb` kosong padahal jalan | pgrep Termux rewel | pakai loop `/proc/*/comm` (lihat tabel di atas) |
| Orang lain nggak bisa buka `0.0.0.0:18099` | beda jaringan / CGNAT | pakai tunnel (§3/§4) |

---

*Terakhir diupdate: 2026-10-19 — PuruClaw 🦞*
