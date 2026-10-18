// Command puruweb: web chat UI for PURU-AI.
//
// Same engine as `puru chat` / Telegram gateway (model + history + memory
// + skills), but served over HTTP with a dark web UI where text and code
// blocks are rendered separately (copy button on top of every code block,
// copy button under every AI reply).
//
// Usage:
//
//	puruweb                 # reads ~/.puru/config.json
//	puruweb -config /path/config.json
//	puruweb -addr 0.0.0.0:8090
//	puruweb -chat 9000000005   # open a specific web chat slot
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/purujawa06-bot/PURU-AI/internal/ai"
	"github.com/purujawa06-bot/PURU-AI/internal/config"
	"github.com/purujawa06-bot/PURU-AI/internal/history"
	"github.com/purujawa06-bot/PURU-AI/internal/memory"
	"github.com/purujawa06-bot/PURU-AI/internal/prompt"
	"github.com/purujawa06-bot/PURU-AI/internal/webui"
)

func main() {
	configPath := flag.String("config", "", "path to config.json (default: auto-resolve)")
	addr := flag.String("addr", "", "listen address host:port (default: from config host/port)")
	chat := flag.Int64("chat", webui.DefaultWebChatID, "web chat slot id")
	flag.Parse()

	cfg, err := config.Load(config.ResolvePath(*configPath))
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	hc := &http.Client{Timeout: 0} // agent manages its own timeouts
	llm, err := ai.NewModel(cfg, hc)
	if err != nil {
		log.Fatalf("ai model: %v", err)
	}
	hist := history.New(cfg.HistoryDir())
	mem := memory.New(cfg.Workspace)
	mem.Model = llm
	agent := &ai.Agent{Client: llm, Config: cfg, HTTP: hc}

	system := renderedSystem(cfg)

	srv := webui.New(agent, hist, mem, cfg, system)

	listen := *addr
	if listen == "" {
		host := cfg.Host
		if host == "" {
			host = "0.0.0.0"
		}
		port := cfg.Port
		if port == 0 {
			port = 8080
		}
		listen = fmt.Sprintf("%s:%d", host, port)
	}

	httpSrv := &http.Server{
		Addr:              listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("🦞 puruweb jalan di http://%s (chat slot %d, model %s)", listen, *chat, cfg.Model.Model)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	// graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
	log.Println("puruweb berhenti.")
}

// renderedSystem mirrors cmd/puru's renderedSystemPrompt (system prompt is
// needed only for token accounting + compaction threshold).
func renderedSystem(cfg *config.Config) string {
	mem := ""
	if b, err := os.ReadFile(cfg.MemoryPath()); err == nil {
		mem = string(b)
	}
	s, err := prompt.Get(mem, memory.LatestSummary(cfg.Workspace), cfg.Workspace, cfg.SkillsPolicy())
	if err != nil {
		return ""
	}
	return s
}
