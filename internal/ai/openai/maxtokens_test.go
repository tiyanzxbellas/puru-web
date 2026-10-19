package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tmc/langchaingo/llms"
)

// max_tokens harus ikut terkirim saat WithMaxTokens dipakai: tanpa ini
// provider memotong jawaban di default kecilnya (finish_reason "length").
func TestMaxTokensSentToProvider(t *testing.T) {
	var got wireRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer srv.Close()

	c, err := New(srv.URL, "test", "test", srv.Client())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = c.GenerateContent(context.Background(),
		[]llms.MessageContent{{Role: llms.ChatMessageTypeHuman, Parts: []llms.ContentPart{llms.TextContent{Text: "halo"}}}},
		llms.WithStreamingFunc(func(_ context.Context, _ []byte) error { return nil }),
		llms.WithMaxTokens(128000),
	)
	if err != nil {
		t.Fatalf("GenerateContent: %v", err)
	}
	if got.MaxTokens != 128000 {
		t.Fatalf("max_tokens = %d, want 128000", got.MaxTokens)
	}
}

// Tanpa WithMaxTokens field-nya harus dihilangkan (omitempty) supaya provider
// pakai defaultnya, bukan 0.
func TestMaxTokensOmittedWhenZero(t *testing.T) {
	var raw map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	c, _ := New(srv.URL, "test", "test", srv.Client())
	_, _ = c.GenerateContent(context.Background(),
		[]llms.MessageContent{{Role: llms.ChatMessageTypeHuman, Parts: []llms.ContentPart{llms.TextContent{Text: "halo"}}}},
		llms.WithStreamingFunc(func(_ context.Context, _ []byte) error { return nil }),
	)
	if _, ok := raw["max_tokens"]; ok {
		t.Fatalf("max_tokens harus absen, dapat %v", raw["max_tokens"])
	}
}
