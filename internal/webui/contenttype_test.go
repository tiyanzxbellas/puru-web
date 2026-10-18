package webui

import "testing"

func TestContentTypeFor(t *testing.T) {
	cases := map[string]string{
		"lagu.mp3":   "audio/mpeg",
		"sing.m4a":   "audio/mp4",
		"x.aac":      "audio/aac",
		"klipp.mkv":  "video/x-matroska",
		"pic.PNG":    "image/png",
		"data.bin":   "application/octet-stream",
		"arsip.zip":  "application/zip",
		"arsip.7z":   "application/x-7z-compressed",
		"paket.apk":  "application/vnd.android.package-archive",
		"catatan.md": "text/markdown",
		"halo.html":  "text/html; charset=utf-8",
		"tanpaext":   "application/octet-stream",
	}
	for name, want := range cases {
		if got := contentTypeFor(name); got != want {
			t.Errorf("contentTypeFor(%q) = %q, want %q", name, got, want)
		}
	}
}
