---
name: image-gen
description: Generate an image from a text prompt (text-to-image) without any API key, using the free Pollinations endpoint. Use when the user asks to "buat gambar", "gambar kan", "bikin ilustrasi/foto/logo/art", or wants a picture created from a description — on Telegram or in the web UI.
---

# Image Generation (free, no API key)

The runtime has **no native image model**, but a text prompt can be turned into a real
JPEG picture for free with Pollinations (`image.pollinations.ai`) — no key, no login.
The result is a normal file: send it with `telegram_sendfile` (it shows inline in the
web UI and as a photo in Telegram).

## Golden path — use the bundled script

```bash
sh "$HOME/.puru/workspace/skills/image-gen/scripts/gen_image.sh" "PROMPT" [OUTFILE]
```

- Default output: `downloads/img-<timestamp>.jpg` (workspace-relative path).
- The script URL-encodes the prompt, requests a 1024x1024 JPEG, and **retries** the
  free-tier rate limit (`1 gambar / 45 detik per IP`, HTTP 402) automatically.
- Add `--size 768` / `--model flux` / `--seed 42` before the prompt if needed:
  `sh gen_image.sh --size 768 --seed 42 "kucing astronot neon" out.jpg`
- Then: `telegram_sendfile(path: "<printed path>", caption: "<prompt singkat>")`.

## Raw options (if the script is unavailable)

```bash
# docet: satu gambar per 45 detik per IP; 402 = kena limit, tunggu lalu ulangi
curl -sL -o downloads/img.jpg \
  "https://image.pollinations.ai/prompt/<prompt-ter-URL-encode>?width=1024&height=1024&nologo=true&model=flux&seed=7"
```

| Param | Isi | Catatan |
|---|---|---|
| prompt | deskripsi gambar (English lebih akurat) | wajib, URL-encode |
| width/height | 256 – 2048 | kuadrat 1024 bagus, 768 lebih cepat |
| model | `flux` (default bagus) atau `sana` | daftar: GET `/models` |
| seed | bilangan bulat | seed sama = gambar sama |
| nologo | `true` | hilangkan watermark kecil |
| enhance | `true`/`false` | biarkan kosong |

## Aturan praktis

1. Prompt bahasa Inggris, detail bila perlu: subjek + aksi + gaya + pencahayaan
   (contoh: `"cyberpunk street food stall at night, neon signs, rain, cinematic"`).
2. Satu gambar = satu request. Beberapa gambar sekaligus = **tunggu 45 detik** antar
   request, atau kirim request berurutan dengan script ini (sudah ada retry).
3. Ukuran file biasanya 30 – 200 KB. Kalau > 49 MB (jarang), turunkan width/height.
4. Jangan pakai untuk wajah orang nyata / konten NSFW — endpoint menolak.
5. Kalau endpoint mati (`402 FREE_LIMIT` terus, atau HTTP 5xx > 3x): laporkan ke user
   apa adanya, tawarkan ulang lewat ImageMagick (gambar bentuk/teks, bukan AI).
