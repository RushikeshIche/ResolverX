package redirects

import (
	"context"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"sync"
	"time"
)

// Server is an HTTP server that listens for incoming web requests and redirects them to their target URLs.
type Server struct {
	addr       string
	service    *Service
	httpServer *http.Server
	listener   net.Listener
	mu         sync.RWMutex
	running    bool
}

// NewServer creates a new HTTP redirect server.
func NewServer(addr string, svc *Service) *Server {
	return &Server{
		addr:    addr,
		service: svc,
	}
}

// Start binds the HTTP listener and starts serving in a background goroutine.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to bind HTTP redirect listener on %s: %w", s.addr, err)
	}
	s.listener = ln

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRedirect)

	s.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		_ = s.httpServer.Serve(ln)
	}()

	s.running = true
	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// IsRunning reports whether the redirect server is listening.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Addr returns the configured address of the HTTP redirect listener.
func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	// 1. Try matching by Host header (e.g. "gitrush.test" or "gitrush.test:8080")
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = CleanShortcut(host)

	if redir, err := s.service.FindByShortcut(ctx, host); err == nil && redir != nil {
		http.Redirect(w, r, redir.TargetURL, redir.StatusCode)
		return
	}

	// 2. Try matching by URL path (e.g. "/gitrush" or "/gitrush.com")
	path := CleanShortcut(r.URL.Path)
	if path != "" {
		if redir, err := s.service.FindByShortcut(ctx, path); err == nil && redir != nil {
			http.Redirect(w, r, redir.TargetURL, redir.StatusCode)
			return
		}
	}

	// 3. Fallback: Display an informative directory page
	allRedirects, _ := s.service.List(ctx)
	renderDirectoryPage(w, host, path, allRedirects)
}

const dirTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>ResolverX — URL Shortcuts</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; margin: 0; padding: 40px 20px; display: flex; justify-content: center; }
    .card { background: #1e293b; border-radius: 12px; padding: 32px; max-width: 650px; width: 100%; box-shadow: 0 10px 25px rgba(0,0,0,0.5); }
    h1 { margin-top: 0; color: #38bdf8; font-size: 24px; }
    p { color: #94a3b8; font-size: 14px; line-height: 1.6; }
    table { width: 100%; border-collapse: collapse; margin-top: 20px; }
    th, td { text-align: left; padding: 12px 8px; border-bottom: 1px solid #334155; font-size: 14px; }
    th { color: #cbd5e1; }
    .code { background: #0f172a; padding: 3px 8px; border-radius: 6px; font-family: monospace; color: #a5f3fc; }
    a { color: #38bdf8; text-decoration: none; }
    a:hover { text-decoration: underline; }
  </style>
</head>
<body>
  <div class="card">
    <h1>ResolverX URL Shortcuts</h1>
    <p>No redirect found for <code>{{.Requested}}</code>. Below is the list of active URL shortcuts configured on your machine:</p>
    {{if .Redirects}}
    <table>
      <thead>
        <tr>
          <th>Shortcut</th>
          <th>Destination</th>
        </tr>
      </thead>
      <tbody>
        {{range .Redirects}}
        <tr>
          <td><span class="code">{{.Shortcut}}</span></td>
          <td><a href="{{.TargetURL}}">{{.TargetURL}}</a></td>
        </tr>
        {{end}}
      </tbody>
    </table>
    {{else}}
    <p style="font-style: italic; color: #64748b;">No URL shortcuts have been added yet. Use the ResolverX desktop application to create one!</p>
    {{end}}
  </div>
</body>
</html>`

type pageData struct {
	Requested string
	Redirects []*Redirect
}

func renderDirectoryPage(w http.ResponseWriter, host, path string, list []*Redirect) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)

	req := host
	if path != "" {
		req += "/" + path
	}

	tmpl, err := template.New("dir").Parse(dirTemplate)
	if err == nil {
		_ = tmpl.Execute(w, pageData{Requested: req, Redirects: list})
	}
}
