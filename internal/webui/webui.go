// Package webui serves a self-contained web chat UI for PURU-AI.
//
// Routes:
//
//	GET    /                 → embedded chat page (dark UI, copy buttons)
//	GET    /health           → {"status":"ok"}
//	GET    /api/info         → engine info (model, workspace, tokens)
//	GET    /api/messages     → current web history (role/content)
//	POST   /api/chat         → run agent on {"message": "..."}
//	POST   /api/clear        → clear history
//	POST   /api/new_chat     → start a new chat slot
//	GET    /api/chats        → list chat slots
//
// The engine is identical to the CLI/Telegram path: Agent.ProcessMessage
// with history.Store, memory compaction, prompt.Get system prompt, skills.
// Web chats use their own history chat id (default 9000000001) so the
// Telegram bot history stays untouched.
package webui

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/purujawa06-bot/PURU-AI/internal/ai"
	"github.com/purujawa06-bot/PURU-AI/internal/config"
	"github.com/purujawa06-bot/PURU-AI/internal/history"
	"github.com/purujawa06-bot/PURU-AI/internal/memory"
	"github.com/purujawa06-bot/PURU-AI/internal/messages"
	"github.com/purujawa06-bot/PURU-AI/internal/tokens"
)

//go:embed index.html
var indexHTML []byte

// DefaultWebChatID is the history slot used by the web UI unless the
// client starts a new chat via /api/new_chat (next id above this).
const DefaultWebChatID int64 = 9000000001

// maxMessageBytes caps a single user message.
const maxMessageBytes = 32 << 10

// maxRun caps one agent run so a hung model call never wedges the slot.
const maxRun = 10 * time.Minute

// Server wires the agent + stores into an HTTP handler.
type Server struct {
	agent  *ai.Agent
	hist   *history.Store
	mem    *memory.Manager
	cfg    *config.Config
	system string

	mu   sync.Mutex
	busy map[int64]bool // chatID → running
}

// New builds the web server. system is the rendered system prompt (used
// for token accounting + compaction threshold, same as the CLI path).
func New(agent *ai.Agent, hist *history.Store, mem *memory.Manager, cfg *config.Config, system string) *Server {
	return &Server{
		agent:  agent,
		hist:   hist,
		mem:    mem,
		cfg:    cfg,
		system: system,
		busy:   map[int64]bool{},
	}
}

// Handler returns the full route tree.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	mux.HandleFunc("/api/info", s.handleInfo)
	mux.HandleFunc("/api/messages", s.handleMessages)
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/clear", s.handleClear)
	mux.HandleFunc("/api/new_chat", s.handleNewChat)
	mux.HandleFunc("/api/chats", s.handleChats)
	mux.HandleFunc("/", s.handleIndex)
	return mux
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, dst any) error {
	b, err := io.ReadAll(io.LimitReader(r.Body, maxMessageBytes+4096))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

func userMsg(s string) []*messages.Message {
	m := &messages.Message{Role: "user"}
	messages.SetContentString(m, s)
	return []*messages.Message{m}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(indexHTML)
}

// ---------- API ----------

type infoResp struct {
	Model         string `json:"model"`
	Workspace     string `json:"workspace"`
	HistoryTokens int    `json:"history_tokens"`
	TokenLimit    int    `json:"token_limit"`
	ChatID        int64  `json:"chat_id"`
	Running       bool   `json:"running"`
	Time          string `json:"time"`
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromQuery(r, DefaultWebChatID)
	s.mu.Lock()
	busy := s.busy[chatID]
	s.mu.Unlock()
	stored := s.hist.Get(chatID)
	writeJSON(w, http.StatusOK, infoResp{
		Model:         s.cfg.Model.Model,
		Workspace:     s.cfg.Workspace,
		HistoryTokens: history.TokenCountFull(s.system, stored),
		TokenLimit:    s.cfg.HistoryTokenLimit,
		ChatID:        chatID,
		Running:       busy,
		Time:          time.Now().Format(time.RFC3339),
	})
}

type msgView struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func viewMessages(stored []*messages.Message) []msgView {
	out := make([]msgView, 0, len(stored))
	for _, m := range stored {
		if m == nil {
			continue
		}
		txt := messageText(m)
		if txt == "" {
			continue
		}
		// only user/assistant text in the UI (skip tool/system noise)
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		out = append(out, msgView{Role: m.Role, Content: txt})
	}
	return out
}

// messageText flattens a message into display text: only "text" parts
// (reasoning/tool-call/tool-result stay hidden from the UI; the engine
// still keeps them in history).
func messageText(m *messages.Message) string {
	return strings.TrimSpace(m.Text())
}

func chatIDFromQuery(r *http.Request, def int64) int64 {
	v := r.URL.Query().Get("chat_id")
	if v == "" {
		return def
	}
	var id int64
	if _, err := fmt.Sscanf(v, "%d", &id); err != nil || id == 0 {
		return def
	}
	return id
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromQuery(r, DefaultWebChatID)
	stored := s.hist.Get(chatID)
	writeJSON(w, http.StatusOK, map[string]any{
		"chat_id":  chatID,
		"messages": viewMessages(stored),
	})
}

type chatReq struct {
	Message string `json:"message"`
	ChatID  int64  `json:"chat_id"`
}

type chatResp struct {
	OK         bool   `json:"ok"`
	Reply      string `json:"reply"`
	Tokens     int    `json:"tokens"`
	HistTokens int    `json:"hist_tokens"`
	ChatID     int64  `json:"chat_id"`
	DurationMs int64  `json:"duration_ms"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if s.agent == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent not ready"})
		return
	}
	var req chatReq
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
		return
	}
	text := strings.TrimSpace(req.Message)
	if text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "empty message"})
		return
	}
	if len(text) > maxMessageBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "message too large"})
		return
	}
	chatID := req.ChatID
	if chatID == 0 {
		chatID = DefaultWebChatID
	}

	// single-flight per chat slot
	s.mu.Lock()
	if s.busy[chatID] {
		s.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]any{"error": "still_running", "message": "Masih memproses pertanyaan sebelumnya…"})
		return
	}
	s.busy[chatID] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.busy[chatID] = false
		s.mu.Unlock()
	}()

	start := time.Now()
	stored := s.hist.Get(chatID)

	// compaction (same as the CLI path)
	if history.TokenCountFull(s.system, stored) >= s.cfg.HistoryTokenLimit && len(stored) > 0 {
		cctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		rel, cerr := s.mem.Compact(cctx, stored)
		cancel()
		if cerr != nil {
			log.Printf("webui compact: %v", cerr)
		} else if rel != "" {
			stored = messages.KeepLastExchange(stored)
			_ = s.hist.Set(chatID, stored)
		}
	}

	opts := &ai.ProcessOptions{ChatID: chatID, Channel: "web"}
	if s.cfg.ShowToolsPreview() {
		opts.OnTool = func(name string, args map[string]any) {
			log.Printf("webui tool: %s", name)
		}
	}

	// independent context: reload/closing the browser must not kill the run
	ctx, cancel := context.WithTimeout(context.Background(), maxRun)
	defer cancel()
	res := s.agent.ProcessMessage(ctx, text, stored, opts)

	// persist (same shape as the CLI path)
	saved := append(append([]*messages.Message{}, stored...), userMsg(text)...)
	saved = append(saved, messages.SanitizeHistoryMessages(res.ResponseMessages)...)
	_ = s.hist.Set(chatID, saved)

	writeJSON(w, http.StatusOK, chatResp{
		OK:         true,
		Reply:      strings.TrimSpace(res.Text),
		Tokens:     res.TotalTokens,
		HistTokens: history.TokenCountFull(s.system, saved),
		ChatID:     chatID,
		DurationMs: time.Since(start).Milliseconds(),
	})
}

type clearReq struct {
	ChatID int64 `json:"chat_id"`
}

func (s *Server) handleClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var req clearReq
	_ = readJSON(r, &req)
	chatID := req.ChatID
	if chatID == 0 {
		chatID = DefaultWebChatID
	}
	s.mu.Lock()
	if s.busy[chatID] {
		s.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]any{"error": "still_running"})
		return
	}
	s.mu.Unlock()
	_ = s.hist.Clear(chatID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "chat_id": chatID})
}

// handleNewChat allocates the next web chat slot (>DefaultWebChatID).
func (s *Server) handleNewChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	dir := s.cfg.HistoryDir()
	var maxID int64 = DefaultWebChatID - 1
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			name := strings.TrimSuffix(e.Name(), ".json")
			var id int64
			if _, err := fmt.Sscanf(name, "%d", &id); err == nil && id > maxID && id < 9999999999 {
				maxID = id
			}
		}
	}
	next := maxID + 1
	if next <= DefaultWebChatID {
		next = DefaultWebChatID
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "chat_id": next})
}

// handleChats lists web chat slots with previews.
func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	type chatInfo struct {
		ID       int64  `json:"id"`
		Title    string `json:"title"`
		Updated  string `json:"updated"`
		Messages int    `json:"messages"`
	}
	dir := s.cfg.HistoryDir()
	out := []chatInfo{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"chats": out})
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		var id int64
		if _, err := fmt.Sscanf(name, "%d", &id); err != nil {
			continue
		}
		// only web-range slots (never expose telegram ids in the UI)
		if id < DefaultWebChatID {
			continue
		}
		stored := s.hist.Get(id)
		title := ""
		for _, m := range stored {
			if m.Role == "user" {
				title = trimTitle(messageText(m))
				break
			}
		}
		updated := ""
		if fi, err := os.Stat(filepath.Join(dir, e.Name())); err == nil {
			updated = fi.ModTime().Format("2006-01-02 15:04")
		}
		out = append(out, chatInfo{ID: id, Title: title, Updated: updated, Messages: len(stored)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"chats": out})
}

func trimTitle(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	runes := []rune(s)
	if len(runes) > 40 {
		return string(runes[:40]) + "…"
	}
	return s
}

// TokensOf counts the history tokens of a chat slot (tests/debug).
func (s *Server) TokensOf(chatID int64) int {
	return tokens.CountConversation(s.hist.Get(chatID))
}
