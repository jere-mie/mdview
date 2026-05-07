package server

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jere-mie/mdview/internal/assets"
	"github.com/jere-mie/mdview/internal/renderer"
	"github.com/jere-mie/mdview/internal/watcher"
)

type Config struct {
	Path     string
	Port     int
	Host     string
	Version  string
	Renderer *renderer.Renderer
}

type Server struct {
	rootPath   string
	targetFile string
	fileMode   bool
	version    string
	renderer   *renderer.Renderer
	indexTmpl  *template.Template
	listener   net.Listener
	httpServer *http.Server
	hub        *hub
	watcher    *watcher.Watcher
}

type indexData struct {
	Title    string
	Heading  string
	Subtitle string
	Entries  []indexEntry
}

type indexEntry struct {
	Name string
	URL  string
	Icon string
	Kind string
}

var websocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

func New(cfg Config) (*Server, error) {
	if cfg.Renderer == nil {
		return nil, errors.New("renderer is required")
	}

	info, err := os.Stat(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("stat path: %w", err)
	}

	s := &Server{
		version:   cfg.Version,
		renderer:  cfg.Renderer,
		indexTmpl: template.Must(template.New("index").Parse(assets.IndexTemplate())),
		hub:       newHub(),
	}

	if info.IsDir() {
		s.rootPath = cfg.Path
	} else {
		s.fileMode = true
		s.targetFile = cfg.Path
		s.rootPath = filepath.Dir(cfg.Path)
	}

	fileWatcher, err := watcher.New(cfg.Path, func(string) {
		s.hub.BroadcastReload()
	})
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}
	s.watcher = fileWatcher

	listener, actualPort, err := listenWithFallback(cfg.Host, cfg.Port, 1000)
	if err != nil {
		_ = s.watcher.Close()
		return nil, err
	}
	s.listener = listener

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/", s.handleRequest)

	s.httpServer = &http.Server{
		Addr:              listenAddr(cfg.Host, actualPort),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s, nil
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		err := s.httpServer.Serve(s.listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.watcher != nil {
		if err := s.watcher.Close(); err != nil {
			return err
		}
	}

	s.hub.Close()
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.hub.Add(conn)
	defer func() {
		s.hub.Remove(conn)
		_ = conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	if s.fileMode {
		s.handleFileMode(w, r)
		return
	}

	s.handleDirectoryMode(w, r)
}

func (s *Server) handleFileMode(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	s.renderMarkdownFile(w, r, s.targetFile, filepath.Base(s.targetFile))
}

func (s *Server) handleDirectoryMode(w http.ResponseWriter, r *http.Request) {
	resolvedPath, err := resolvePath(s.rootPath, r.URL.Path)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to read path", http.StatusInternalServerError)
		return
	}

	if info.IsDir() {
		if !strings.HasSuffix(r.URL.Path, "/") {
			target := r.URL.Path + "/"
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}

		s.renderDirectoryIndex(w, r, resolvedPath)
		return
	}

	if isMarkdownFile(resolvedPath) {
		s.renderMarkdownFile(w, r, resolvedPath, filepath.Base(resolvedPath))
		return
	}

	http.ServeFile(w, r, resolvedPath)
}

func (s *Server) renderMarkdownFile(w http.ResponseWriter, r *http.Request, filePath string, title string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}

	rendered, err := s.renderer.Render(content, renderer.Options{
		Title:          title,
		LiveReload:     true,
		ReloadEndpoint: "/ws",
	})
	if err != nil {
		http.Error(w, "failed to render markdown", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(rendered)
}

func (s *Server) renderDirectoryIndex(w http.ResponseWriter, r *http.Request, directoryPath string) {
	entries, err := os.ReadDir(directoryPath)
	if err != nil {
		http.Error(w, "failed to read directory", http.StatusInternalServerError)
		return
	}

	currentURLPath := strings.TrimSuffix(r.URL.Path, "/")
	listing := make([]indexEntry, 0, len(entries)+1)
	if directoryPath != s.rootPath {
		parentURL := path.Dir(currentURLPath)
		if parentURL == "." {
			parentURL = "/"
		}
		if !strings.HasSuffix(parentURL, "/") {
			parentURL += "/"
		}

		listing = append(listing, indexEntry{
			Name: "..",
			URL:  parentURL,
			Icon: "↩",
			Kind: "parent",
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	for _, entry := range entries {
		entryURL := path.Join(r.URL.Path, entry.Name())
		icon := "📄"
		kind := "file"
		if entry.IsDir() {
			entryURL += "/"
			icon = "📁"
			kind = "folder"
		}

		listing = append(listing, indexEntry{
			Name: entry.Name(),
			URL:  entryURL,
			Icon: icon,
			Kind: kind,
		})
	}

	relativeLabel, err := filepath.Rel(s.rootPath, directoryPath)
	if err != nil {
		relativeLabel = "."
	}
	if relativeLabel == "." {
		relativeLabel = "/"
	} else {
		relativeLabel = "/" + filepath.ToSlash(relativeLabel)
	}

	data := indexData{
		Title:    "mdview",
		Heading:  relativeLabel,
		Subtitle: fmt.Sprintf("Browsing %s with mdview %s", s.rootPath, s.version),
		Entries:  listing,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.indexTmpl.Execute(w, data); err != nil {
		http.Error(w, "failed to render directory", http.StatusInternalServerError)
	}
}

func resolvePath(root string, requestPath string) (string, error) {
	for _, segment := range strings.Split(requestPath, "/") {
		if segment == ".." {
			return "", fmt.Errorf("path escapes root: %s", requestPath)
		}
	}

	relative := strings.TrimPrefix(path.Clean("/"+requestPath), "/")
	resolved := filepath.Join(root, filepath.FromSlash(relative))

	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes root: %s", requestPath)
	}

	return resolved, nil
}

func isMarkdownFile(filePath string) bool {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".md", ".markdown", ".mdown", ".mkd":
		return true
	default:
		return false
	}
}

func listenAddr(host string, port int) string {
	if host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}

func listenWithFallback(host string, startPort int, maxAttempts int) (net.Listener, int, error) {
	if host == "" {
		host = "127.0.0.1"
	}
	if startPort < 1 || startPort > 65535 {
		return nil, 0, fmt.Errorf("invalid server port %d", startPort)
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts && startPort+attempt <= 65535; attempt++ {
		candidatePort := startPort + attempt
		listener, err := net.Listen("tcp", listenAddr(host, candidatePort))
		if err == nil {
			return listener, candidatePort, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = errors.New("no ports available")
	}
	return nil, 0, fmt.Errorf("listen on %s starting at %d: %w", host, startPort, lastErr)
}

func (s *Server) ListenAddr() string {
	if s.httpServer == nil {
		return ""
	}
	return s.httpServer.Addr
}
