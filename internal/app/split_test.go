package app

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Jawaban pendek dikirim utuh (satu pesan).
func TestSplitForTelegramShort(t *testing.T) {
	in := "halo mas, ini jawaban pendek"
	got := splitForTelegram(in, 4000)
	if len(got) != 1 || got[0] != in {
		t.Fatalf("jawaban pendek harus 1 potong utuh, dapat %d: %q", len(got), got)
	}
}

// Jawaban panjang dipecah, tidak ada satu pun yang lewat batas, dan tidak ada
// teks yang hilang.
func TestSplitForTelegramLong(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 400; i++ {
		sb.WriteString("baris ke-")
		sb.WriteString(strings.Repeat("x", 30))
		sb.WriteString("\n")
	}
	in := sb.String()
	if len([]rune(in)) <= 4000 {
		t.Fatalf("test butuh teks > 4000 rune, dapat %d", len([]rune(in)))
	}

	got := splitForTelegram(in, 4000)
	if len(got) < 2 {
		t.Fatalf("teks panjang harus jadi >1 potong, dapat %d", len(got))
	}
	var joined strings.Builder
	for i, p := range got {
		if n := len([]rune(p)); n > 4000 {
			t.Fatalf("potong ke-%d kepanjangan: %d rune", i, n)
		}
		joined.WriteString(p)
	}
	if strings.Count(joined.String(), "baris ke-") != 400 {
		t.Fatalf("ada baris yang hilang: %d", strings.Count(joined.String(), "baris ke-"))
	}
}

// Pemotongan tidak boleh merusak UTF-8 / emoji.
func TestSplitForTelegramKeepsRunes(t *testing.T) {
	in := strings.Repeat("mantap mas ", 900) + strings.Repeat("🦞", 300)
	got := splitForTelegram(in, 4000)
	for i, p := range got {
		if !utf8.ValidString(p) {
			t.Fatalf("potong ke-%d bukan UTF-8 valid", i)
		}
	}
	if !strings.Contains(strings.Join(got, ""), "🦞") {
		t.Fatal("emoji hilang")
	}
}
