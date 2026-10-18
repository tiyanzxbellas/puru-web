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
//	POST   /api/upload       -> upload a file for the agent (multipart field file)
//	GET    /api/files        -> list files under the workspace downloads/ folder
//	GET    /api/file?path=   -> download one of those files (downloads/ only)
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
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
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
const maxMessageBytes = 256 << 10

// maxUploadBytes caps one uploaded file (49MB; under Telegram's 50 MiB).
const maxUploadBytes = 49 << 20

// maxRun caps one agent run so a hung model call never wedges the slot.
// Override with PURUWEB_MAX_RUN_MIN (minute); default 60 because heavy tasks
// (scrape/sniff) can run for a long time.
var maxRun = 60 * time.Minute

// Server wires the agent + stores into an HTTP handler.
type Server struct {
	agent  *ai.Agent
	hist   *history.Store
	mem    *memory.Manager
	cfg    *config.Config
	system string

	mu   sync.Mutex
	busy map[int64]bool // chatID → running

	filesMu sync.Mutex
	files   map[int64][]SentFile // chatID → files pushed by the agent

	freshSys string // system prompt tanpa memori (mode fresh)

	freshMu sync.Mutex
	fresh   map[int64]bool // chatID → run without long-term memory
}

// New builds the web server. system is the rendered system prompt (used
// for token accounting + compaction threshold, same as the CLI path).
func New(agent *ai.Agent, hist *history.Store, mem *memory.Manager, cfg *config.Config, system string) *Server {
	if v := strings.TrimSpace(os.Getenv("PURUWEB_MAX_RUN_MIN")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxRun = time.Duration(n) * time.Minute
		}
	}
	s := &Server{
		agent:  agent,
		hist:   hist,
		mem:    mem,
		cfg:    cfg,
		system: system,
		busy:   map[int64]bool{},
		files:  map[int64][]SentFile{},
		fresh:  map[int64]bool{},
	}
	s.loadFresh()
	s.loadFiles()
	return s
}

// SetFreshSystem sets the memory-less system prompt (mode fresh).
func (s *Server) SetFreshSystem(sys string) { s.freshSys = sys }

func (s *Server) freshSystem() string {
	if strings.TrimSpace(s.freshSys) == "" {
		return s.system
	}
	return s.freshSys
}

func (s *Server) freshPath() string {
	return filepath.Join(s.downloadsDir(), ".webui-fresh.json")
}

func (s *Server) loadFresh() {
	b, err := os.ReadFile(s.freshPath())
	if err != nil || len(b) == 0 {
		return
	}
	m := map[int64]bool{}
	if json.Unmarshal(b, &m) != nil {
		return
	}
	s.freshMu.Lock()
	for k, v := range m {
		s.fresh[k] = v
	}
	s.freshMu.Unlock()
}

func (s *Server) saveFresh() {
	s.freshMu.Lock()
	cp := map[int64]bool{}
	for k, v := range s.fresh {
		cp[k] = v
	}
	s.freshMu.Unlock()
	b, err := json.Marshal(cp)
	if err != nil {
		return
	}
	_ = os.MkdirAll(s.downloadsDir(), 0o755)
	_ = os.WriteFile(s.freshPath(), b, 0o644)
}

// isFresh reports whether the chat slot runs without long-term memory.
func (s *Server) isFresh(chatID int64) bool {
	s.freshMu.Lock()
	defer s.freshMu.Unlock()
	return s.fresh[chatID]
}

func (s *Server) setFresh(chatID int64, on bool) {
	s.freshMu.Lock()
	if on {
		s.fresh[chatID] = true
	} else {
		delete(s.fresh, chatID)
	}
	s.freshMu.Unlock()
	s.saveFresh()
}

// filesPath is where the sent-file index is persisted (dotfile, jadi nggak
// muncul di panel 📁 File).
func (s *Server) filesPath() string {
	return filepath.Join(s.downloadsDir(), ".webui-files.json")
}

// loadFiles mengembalikan daftar file per chat biar chip tetap ada setelah
// server di-restart.
func (s *Server) loadFiles() {
	b, err := os.ReadFile(s.filesPath())
	if err != nil || len(b) == 0 {
		return
	}
	m := map[string][]SentFile{}
	if json.Unmarshal(b, &m) != nil {
		return
	}
	s.filesMu.Lock()
	for k, v := range m {
		var id int64
		if _, err := fmt.Sscanf(k, "%d", &id); err != nil || id == 0 {
			continue
		}
		s.files[id] = v
	}
	s.filesMu.Unlock()
}

func (s *Server) saveFiles() {
	s.filesMu.Lock()
	cp := map[string][]SentFile{}
	for k, v := range s.files {
		if len(v) == 0 {
			continue
		}
		cp[fmt.Sprint(k)] = v
	}
	s.filesMu.Unlock()
	b, err := json.Marshal(cp)
	if err != nil {
		return
	}
	_ = os.MkdirAll(s.downloadsDir(), 0o755)
	_ = os.WriteFile(s.filesPath(), b, 0o644)
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
	mux.HandleFunc("/api/fresh", s.handleFresh)
	mux.HandleFunc("/api/upload", s.handleUpload)
	mux.HandleFunc("/api/files", s.handleFiles)
	mux.HandleFunc("/api/file", s.handleFile)
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
	Fresh         bool   `json:"fresh"`
	Time          string `json:"time"`
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromQuery(r, DefaultWebChatID)
	s.mu.Lock()
	busy := s.busy[chatID]
	s.mu.Unlock()
	stored := s.hist.Get(chatID)
	sys := s.system
	if s.isFresh(chatID) {
		sys = s.freshSystem()
	}
	writeJSON(w, http.StatusOK, infoResp{
		Model:         s.cfg.Model.Model,
		Workspace:     s.cfg.Workspace,
		HistoryTokens: history.TokenCountFull(sys, stored),
		TokenLimit:    s.cfg.HistoryTokenLimit,
		ChatID:        chatID,
		Running:       busy,
		Fresh:         s.isFresh(chatID),
		Time:          time.Now().Format(time.RFC3339),
	})
}

type msgView struct {
	Role    string     `json:"role"`
	Content string     `json:"content"`
	Files   []SentFile `json:"files,omitempty"`
}

// viewMessages renders the chat untuk UI dan menempelkan lagi file yang
// dikirim agent, jadi chip + preview video/gambar tetap tampil setelah reload
// atau server restart (dulu chip cuma ada di respons /api/chat saat itu).
func (s *Server) viewMessages(chatID int64) []msgView {
	out := buildViews(s.hist.Get(chatID))
	s.attachFiles(chatID, out)
	return out
}

func buildViews(stored []*messages.Message) []msgView {
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
		// Satu run agent bisa menghasilkan banyak pesan assistant (narasi
		// antar tool-call). UI cukup menampilkan jawaban FINAL tiap run,
		// biar tidak tampil sebagai balasan pendek-pendek.
		if n := len(out); n > 0 && out[n-1].Role == "assistant" && m.Role == "assistant" {
			out[n-1].Content = txt
			continue
		}
		out = append(out, msgView{Role: m.Role, Content: txt})
	}
	return out
}

// attachFiles menaruh lagi tiap file yang dikirim ke bubble-nya. Idx = jumlah
// bubble saat file dikirim = posisi jawaban final run itu. Kalau di luar
// jangkauan (run ke-interrupt), chip jatuh ke bubble assistant terakhir biar
// nggak hilang.
func (s *Server) attachFiles(chatID int64, out []msgView) {
	files := s.sentFiles(chatID)
	if len(files) == 0 || len(out) == 0 {
		return
	}
	fallback := len(out) - 1
	for i := len(out) - 1; i >= 0; i-- {
		if out[i].Role == "assistant" {
			fallback = i
			break
		}
	}
	for _, f := range files {
		idx := f.Idx
		if idx < 0 || idx >= len(out) || out[idx].Role != "assistant" {
			idx = fallback
		}
		out[idx].Files = append(out[idx].Files, f)
	}
}

// filesIdx = jumlah bubble yang tampil sekarang; itulah posisi jawaban final
// run yang sedang jalan, jadi file nempel di bubble yang tepat.
func (s *Server) filesIdx(chatID int64) int {
	return len(buildViews(s.hist.Get(chatID)))
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
	writeJSON(w, http.StatusOK, map[string]any{
		"chat_id":  chatID,
		"messages": s.viewMessages(chatID),
	})
}

type chatReq struct {
	Message string `json:"message"`
	ChatID  int64  `json:"chat_id"`
}

type chatResp struct {
	OK         bool       `json:"ok"`
	Reply      string     `json:"reply"`
	Tokens     int        `json:"tokens"`
	HistTokens int        `json:"hist_tokens"`
	ChatID     int64      `json:"chat_id"`
	DurationMs int64      `json:"duration_ms"`
	Files      []SentFile `json:"files,omitempty"`
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
	filesBefore := len(s.sentFiles(chatID))

	// Simpan pertanyaan user SEKARANG (sebelum run jalan): kalau tab di-reload
	// atau server restart di tengah proses, pertanyaannya tidak hilang.
	_ = s.hist.Set(chatID, append(append([]*messages.Message{}, stored...), userMsg(text)...))

	// Compaction pakai ukuran RIWAYAT saja (bukan system+riwayat). Dulu
	// system+riwayat dipakai, dan karena MEMORY.md bisa lebih besar dari
	// history_token_limit, compaction jalan di tiap pesan → riwayat yang
	// tampil kepangkas jadi 1 exchange (keluhan "pembicaraan hilang").
	if history.TokenCount(stored) >= s.cfg.HistoryTokenLimit && len(stored) > 4 {
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

	sys := s.system
	if s.isFresh(chatID) {
		sys = s.freshSystem()
	}

	opts := &ai.ProcessOptions{ChatID: chatID, Channel: "web", Fresh: s.isFresh(chatID)}
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
		HistTokens: history.TokenCountFull(sys, saved),
		Files:      s.newFiles(chatID, filesBefore),
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
	s.archiveHistory(chatID)
	_ = s.hist.Clear(chatID)
	// hapus riwayat = mulai dari nol: chat ini jalan tanpa memori jangka
	// panjang sampai dimatikan lagi lewat tombol 🧠 Memori.
	s.setFresh(chatID, true)
	// chat dikosongkan: chip file lama jangan ikut nempel di chat yang baru
	// (filenya sendiri tetap ada di server, masih bisa diambil dari panel).
	s.filesMu.Lock()
	delete(s.files, chatID)
	s.filesMu.Unlock()
	s.saveFiles()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "chat_id": chatID, "fresh": true})
}

// archiveHistory menyalin file riwayat ke <historydir>/.trash/ sebelum dihapus.
// "Hapus riwayat" tetap membuat chat fresh (tidak diingat AI), tapi percakapan
// lama tidak hilang permanen karena salah klik: masih ada salinannya di .trash.
func (s *Server) archiveHistory(chatID int64) {
	if s == nil || s.cfg == nil {
		return
	}
	dir := s.cfg.HistoryDir()
	b, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("%d.json", chatID)))
	if err != nil || len(b) == 0 {
		return
	}
	trash := filepath.Join(dir, ".trash")
	if err := os.MkdirAll(trash, 0o755); err != nil {
		log.Printf("webui trash: %v", err)
		return
	}
	name := fmt.Sprintf("%s-%d.json", time.Now().Format("20060102-150405"), chatID)
	if err := os.WriteFile(filepath.Join(trash, name), b, 0o644); err != nil {
		log.Printf("webui trash: %v", err)
		return
	}
	log.Printf("webui trash: chat %d -> %s (%d bytes)", chatID, name, len(b))
}

type freshReq struct {
	ChatID int64 `json:"chat_id"`
	Fresh  *bool `json:"fresh"`
}

// handleFresh toggles mode fresh (no long-term memory) for one chat slot.
func (s *Server) handleFresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var req freshReq
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
		return
	}
	chatID := req.ChatID
	if chatID == 0 {
		chatID = DefaultWebChatID
	}
	on := true
	if req.Fresh != nil {
		on = *req.Fresh
	}
	s.setFresh(chatID, on)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "chat_id": chatID, "fresh": on})
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
	// chat baru = fresh: tanpa MEMORY.md + tanpa ringkasan konteks.
	s.setFresh(next, true)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "chat_id": next, "fresh": true})
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

// ---------- files (upload / download bridge) ----------

// SentFile is one file pushed by the agent into a web chat.
type SentFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	URL     string `json:"url"`
	Size    int64  `json:"size"`
	Caption string `json:"caption,omitempty"`
	Time    string `json:"time"`
	// Idx = jumlah bubble yang tampil saat file dikirim; dipakai buat
	// nempelin chip ke bubble jawaban yang benar (tahan reload/restart).
	Idx int `json:"idx,omitempty"`
}

func (s *Server) downloadsDir() string { return filepath.Join(s.cfg.Workspace, "downloads") }
func (s *Server) webDir() string       { return filepath.Join(s.downloadsDir(), "web") }
func (s *Server) uploadsDir() string   { return filepath.Join(s.downloadsDir(), "uploads") }

// SendFile makes telegram_sendfile work on the web channel: the file is stored
// under downloads/web/ and offered to the browser as a download chip.
func (s *Server) SendFile(ctx context.Context, chatID int64, filename string, data []byte, caption string) error {
	if s == nil || s.cfg == nil {
		return errors.New("web ui belum siap")
	}
	if len(data) > maxUploadBytes {
		return fmt.Errorf("file %s terlalu besar (maks %d MB)", filename, maxUploadBytes>>20)
	}
	name := filepath.Base(strings.TrimSpace(filename))
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "file.bin"
	}
	dir := s.webDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	stored := fmt.Sprintf("%s-%s", time.Now().Format("20060102-150405"), name)
	if err := os.WriteFile(filepath.Join(dir, stored), data, 0o644); err != nil {
		return err
	}
	rel := path.Join("web", stored)
	sf := SentFile{
		Name:    name,
		Path:    rel,
		URL:     "/api/file?path=" + url.QueryEscape(rel),
		Size:    int64(len(data)),
		Caption: caption,
		Time:    time.Now().Format("02 Jan 15:04"),
	}
	sf.Idx = s.filesIdx(chatID)
	s.filesMu.Lock()
	s.files[chatID] = append(s.files[chatID], sf)
	if n := len(s.files[chatID]); n > 200 {
		s.files[chatID] = append([]SentFile{}, s.files[chatID][n-200:]...)
	}
	s.filesMu.Unlock()
	s.saveFiles()
	log.Printf("webui sendfile: chat %d <- %s (%d bytes)", chatID, name, len(data))
	return nil
}

// GetTelegramUser is part of ai.TelegramClient; Telegram-only by nature.
func (s *Server) GetTelegramUser(ctx context.Context, userID int64) (*ai.TelegramUserInfo, error) {
	return nil, errors.New("telegram_getuser cuma jalan di chat Telegram")
}

func (s *Server) sentFiles(chatID int64) []SentFile {
	s.filesMu.Lock()
	defer s.filesMu.Unlock()
	out := make([]SentFile, len(s.files[chatID]))
	copy(out, s.files[chatID])
	return out
}

func (s *Server) newFiles(chatID int64, from int) []SentFile {
	all := s.sentFiles(chatID)
	if from < 0 || from > len(all) {
		return nil
	}
	return all[from:]
}

// resolveDownloadsFile maps a downloads/-relative path to a real path and
// refuses anything outside downloads/.
func (s *Server) resolveDownloadsFile(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", errors.New("path kosong")
	}
	rel = strings.TrimPrefix(path.Clean("/"+filepath.ToSlash(rel)), "/")
	base := s.downloadsDir()
	full := filepath.Join(base, filepath.FromSlash(rel))
	if full != base && !strings.HasPrefix(full, base+string(os.PathSeparator)) {
		return "", errors.New("path di luar folder downloads")
	}
	return full, nil
}

// handleUpload stores one uploaded file under downloads/uploads/ and returns
// its absolute path so the agent can read_file it right away.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+(1<<20))
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unggahan gagal / file terlalu besar (maks 20MB)"})
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "field 'file' tidak ada"})
		return
	}
	defer f.Close()
	name := filepath.Base(strings.TrimSpace(hdr.Filename))
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "unggahan.bin"
	}
	dir := s.uploadsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	stored := fmt.Sprintf("%s-%s", time.Now().Format("20060102-150405"), name)
	full := filepath.Join(dir, stored)
	dst, err := os.Create(full)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	n, cerr := io.Copy(dst, io.LimitReader(f, maxUploadBytes+1))
	_ = dst.Close()
	if cerr != nil {
		_ = os.Remove(full)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": cerr.Error()})
		return
	}
	if n > maxUploadBytes {
		_ = os.Remove(full)
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "file terlalu besar (maks 20MB)"})
		return
	}
	rel := path.Join("downloads", "uploads", stored)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"name": name,
		"path": full,
		"rel":  rel,
		"size": n,
		"url":  "/api/file?path=" + url.QueryEscape(path.Join("uploads", stored)),
	})
}

// fileEntry is one row of GET /api/files.
type fileEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
	Time string `json:"time"`
	Src  string `json:"src,omitempty"`

	mod time.Time `json:"-"`
}

// handleFiles lists everything under downloads/ (upload + hasil kerja agent),
// terbaru dulu.
func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	base := s.downloadsDir()
	out := []fileEntry{}
	_ = filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, rerr := filepath.Rel(base, p)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && (strings.HasPrefix(d.Name(), ".") || strings.Count(rel, "/") >= 2) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		fi, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		src := ""
		switch {
		case strings.HasPrefix(rel, "web/"):
			src = "dari Puru"
		case strings.HasPrefix(rel, "uploads/"):
			src = "dari kamu"
		}
		out = append(out, fileEntry{
			Name: filepath.Base(p),
			Path: rel,
			URL:  "/api/file?path=" + url.QueryEscape(rel),
			Size: fi.Size(),
			Time: fi.ModTime().Format("02 Jan 15:04"),
			Src:  src,
			mod:  fi.ModTime(),
		})
		if len(out) >= 500 {
			return filepath.SkipAll
		}
		return nil
	})
	sort.Slice(out, func(a, b int) bool { return out[a].mod.After(out[b].mod) })
	writeJSON(w, http.StatusOK, map[string]any{"files": out, "dir": base})
}

// handleFile serves one file from downloads/ as an attachment.
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	full, err := s.resolveDownloadsFile(r.URL.Query().Get("path"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	f, err := os.Open(full)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "file tidak ditemukan"})
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "bukan file"})
		return
	}
	name := filepath.Base(full)
	ct := mime.TypeByExtension(filepath.Ext(name))
	if ct == "" {
		ct = "application/octet-stream"
	}
	kind := "attachment"
	if r.URL.Query().Get("inline") != "" {
		// ?inline=1 -> dipakai <img>/<video> di UI biar bisa diputar langsung.
		kind = "inline"
	}
	disp := mime.FormatMediaType(kind, map[string]string{"filename": name})
	if disp == "" {
		disp = kind
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", disp)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, name, fi.ModTime(), f)
}
