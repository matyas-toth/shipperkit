package httpapi

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/websocket"
	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/store"
)

type terminalSession struct {
	record     store.TerminalSession
	ticket     string
	connectBy  time.Time
	mu         sync.Mutex
	connected  bool
	connecting bool
	cancel     context.CancelFunc
}

func (s *Server) terminalRoutes(r chi.Router) {
	r.With(s.csrf, s.limit).Post("/applications/{id}/terminal", s.startTerminal)
	r.Get("/applications/{id}/terminal/audit", s.terminalAudit)
	r.Get("/applications/{id}/terminal/{terminal}", s.connectTerminal)
	r.With(s.csrf, s.limit).Post("/applications/{id}/terminal/{terminal}/revoke", s.revokeTerminal)
}
func (s *Server) terminalSource() (deploy.TerminalSource, bool) {
	if s.cfg.Worker == nil {
		return nil, false
	}
	source, ok := s.cfg.Worker.Runtime.(deploy.TerminalSource)
	return source, ok
}
func (s *Server) terminalBrowser(w http.ResponseWriter, r *http.Request) (store.Session, string, bool) {
	// Interactive sessions require the browser's revocable cookie and CSRF protection.
	if principal(r) != nil {
		s.problem(w, 403, "terminal_denied", "Sign in to open a terminal")
		return store.Session{}, "", false
	}
	session, err := s.session(r)
	if err != nil {
		s.problem(w, 401, "unauthenticated", "Sign in to open a terminal")
		return session, "", false
	}
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		s.problem(w, 401, "unauthenticated", "Sign in to open a terminal")
		return session, "", false
	}
	return session, auth.Hash(cookie.Value), true
}
func (s *Server) terminalOwned(ctx context.Context, v store.TerminalSession) error {
	session, err := s.db.Session(ctx, v.AuthHash)
	if err != nil || session.User.ID != v.UserID || session.AuthStep() != "" {
		return errors.New("authentication_revoked")
	}
	current, err := s.db.Terminal(ctx, v.ID)
	if err != nil || current.EndedAt != 0 || current.ExpiresAt <= time.Now().Unix() {
		return errors.New("session_expired")
	}
	app, err := s.db.Application(ctx, v.ApplicationID)
	if err != nil {
		return errors.New("application_unavailable")
	}
	found := false
	for _, c := range app.Containers {
		if c.ID == v.ContainerID && c.Managed && c.ApplicationID == app.ID && c.State == "running" {
			found = true
		}
	}
	if !found {
		return errors.New("container_unavailable")
	}
	source, ok := s.terminalSource()
	if !ok {
		return errors.New("terminal_unavailable")
	}
	if err = source.CheckTerminal(ctx, v.ApplicationID, v.ContainerID); err != nil {
		return errors.New("container_unavailable")
	}
	return nil
}
func (s *Server) startTerminal(w http.ResponseWriter, r *http.Request) {
	session, hash, ok := s.terminalBrowser(w, r)
	if !ok {
		return
	}
	var input struct {
		ContainerID string `json:"containerId"`
		Minutes     int    `json:"minutes"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	if input.Minutes == 0 {
		input.Minutes = 15
	}
	if input.Minutes < 1 || input.Minutes > 30 {
		s.problem(w, 422, "invalid_timeout", "Choose a timeout between 1 and 30 minutes")
		return
	}
	app, err := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		s.resourceError(w, err)
		return
	}
	found := false
	for _, c := range app.Containers {
		if c.ID == input.ContainerID && c.Managed && c.ApplicationID == app.ID && c.State == "running" {
			found = true
		}
	}
	if !found {
		s.problem(w, 403, "container_denied", "Choose a running container in this application")
		return
	}
	source, ok := s.terminalSource()
	if !ok {
		s.problem(w, 503, "terminal_unavailable", "The terminal is unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	if err = source.CheckTerminal(ctx, app.ID, input.ContainerID); err != nil {
		s.problem(w, 409, "container_unavailable", "The container is unavailable")
		return
	}
	expiry := min(time.Now().Add(time.Duration(input.Minutes)*time.Minute).Unix(), session.ExpiresAt)
	v := store.TerminalSession{ID: auth.Token()[:24], UserID: session.User.ID, AuthHash: hash, ApplicationID: app.ID, ContainerID: input.ContainerID, ExpiresAt: expiry}
	if err = s.db.CreateTerminal(r.Context(), v); err != nil {
		if errors.Is(err, store.ErrTerminalLimit) {
			s.problem(w, 429, "terminal_limit", "Close an existing terminal first")
		} else {
			s.internal(w, err)
		}
		return
	}
	t := &terminalSession{record: v, ticket: auth.Token(), connectBy: time.Now().Add(30 * time.Second)}
	s.terminalMu.Lock()
	if s.terminals == nil {
		s.terminals = map[string]*terminalSession{}
	}
	s.terminals[v.ID] = t
	s.terminalMu.Unlock()
	// Pending tickets expire without leaving a session slot or an unaudited shell.
	time.AfterFunc(30*time.Second, func() {
		t.mu.Lock()
		connected := t.connected
		t.mu.Unlock()
		if !connected {
			s.finishTerminal(t, "end", "ticket_expired")
		}
	})
	s.json(w, 201, map[string]any{"id": v.ID, "ticket": t.ticket, "expiresAt": expiry})
}
func (s *Server) findTerminal(w http.ResponseWriter, r *http.Request) (*terminalSession, bool) {
	session, hash, ok := s.terminalBrowser(w, r)
	if !ok {
		return nil, false
	}
	s.terminalMu.Lock()
	t := s.terminals[chi.URLParam(r, "terminal")]
	s.terminalMu.Unlock()
	if t == nil || t.record.ApplicationID != chi.URLParam(r, "id") || t.record.UserID != session.User.ID || t.record.AuthHash != hash {
		s.problem(w, 404, "not_found", "The terminal was not found")
		return nil, false
	}
	return t, true
}
func (s *Server) finishTerminal(t *terminalSession, event, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.db.EndTerminal(ctx, t.record.ID, event, reason)
	t.mu.Lock()
	if t.cancel != nil {
		t.cancel()
	}
	t.mu.Unlock()
	s.terminalMu.Lock()
	if s.terminals[t.record.ID] == t {
		delete(s.terminals, t.record.ID)
	}
	s.terminalMu.Unlock()
	return err
}
func (s *Server) revokeTerminal(w http.ResponseWriter, r *http.Request) {
	t, ok := s.findTerminal(w, r)
	if !ok {
		return
	}
	if err := s.finishTerminal(t, "revoke", "user_revoked"); err != nil {
		s.internal(w, err)
		return
	}
	s.json(w, 200, map[string]bool{"revoked": true})
}
func (s *Server) terminalAudit(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.terminalBrowser(w, r); !ok {
		return
	}
	if _, err := s.db.Application(r.Context(), chi.URLParam(r, "id")); err != nil {
		s.resourceError(w, err)
		return
	}
	items, err := s.db.TerminalAudit(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		s.internal(w, err)
		return
	}
	s.json(w, 200, map[string]any{"items": items})
}

type terminalMessage struct {
	Type   string `json:"type"`
	Ticket string `json:"ticket,omitempty"`
	Data   string `json:"data,omitempty"`
	Cols   uint   `json:"cols,omitempty"`
	Rows   uint   `json:"rows,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func (s *Server) connectTerminal(w http.ResponseWriter, r *http.Request) {
	t, ok := s.findTerminal(w, r)
	if !ok {
		return
	}
	origin := r.Header.Get("Origin")
	if origin == "" || (origin != requestOrigin(r) && origin != s.publicURL()) {
		s.problem(w, 403, "origin_denied", "The request origin is not allowed")
		return
	}
	t.mu.Lock()
	if t.connected || t.connecting {
		t.mu.Unlock()
		s.problem(w, 409, "terminal_connected", "This terminal is already connected")
		return
	}
	t.connecting = true
	t.mu.Unlock()
	defer func() { t.mu.Lock(); t.connecting = false; t.mu.Unlock() }()
	websocket.Server{Handshake: func(config *websocket.Config, req *http.Request) error { return nil }, Handler: func(ws *websocket.Conn) {
		defer ws.Close()
		ws.MaxPayloadBytes = 16 << 10
		_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
		var first terminalMessage
		if websocket.JSON.Receive(ws, &first) != nil {
			return
		}
		t.mu.Lock()
		allowed := first.Type == "connect" && auth.Equal(first.Ticket, t.ticket) && !t.connected && time.Now().Before(t.connectBy)
		if allowed {
			t.connected = true
			t.ticket = ""
		}
		t.mu.Unlock()
		if !allowed {
			return
		}
		ctx, cancel := context.WithDeadline(context.Background(), time.Unix(t.record.ExpiresAt, 0))
		defer cancel()
		t.mu.Lock()
		t.cancel = cancel
		t.mu.Unlock()
		reason := "disconnected"
		defer func() { s.finishTerminal(t, "end", reason) }()
		if err := s.terminalOwned(ctx, t.record); err != nil {
			reason = err.Error()
			return
		}
		source, _ := s.terminalSource()
		cols, rows := first.Cols, first.Rows
		if cols < 2 || cols > 300 {
			cols = 80
		}
		if rows < 2 || rows > 120 {
			rows = 24
		}
		terminal, err := source.OpenTerminal(ctx, t.record.ApplicationID, t.record.ContainerID, t.record.ID, cols, rows)
		if err != nil {
			reason = "shell_unavailable"
			_ = websocket.JSON.Send(ws, terminalMessage{Type: "end", Reason: "The container shell is unavailable"})
			return
		}
		defer terminal.Close()
		stopped := make(chan struct{})
		defer close(stopped)
		go func() {
			select {
			case <-ctx.Done():
				ws.Close()
				terminal.Close()
			case <-stopped:
			}
		}()
		// Output is sent directly with a deadline so a slow peer cannot accumulate memory.
		outputDone := make(chan error, 1)
		var sendMu sync.Mutex
		send := func(m terminalMessage) error {
			sendMu.Lock()
			defer sendMu.Unlock()
			_ = ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
			return websocket.JSON.Send(ws, m)
		}
		if send(terminalMessage{Type: "ready"}) != nil {
			return
		}
		go func() {
			buffer := make([]byte, 8192)
			total := 0
			for {
				if ctx.Err() != nil {
					outputDone <- ctx.Err()
					return
				}
				n, e := terminal.Read(buffer)
				if n > 0 {
					total += n
					if total > 32<<20 {
						outputDone <- errors.New("output_limit")
						return
					}
					if err := send(terminalMessage{Type: "output", Data: base64.StdEncoding.EncodeToString(buffer[:n])}); err != nil {
						outputDone <- err
						return
					}
				}
				if e != nil {
					outputDone <- e
					return
				}
			}
		}()
		incoming := make(chan terminalMessage, 8)
		readDone := make(chan error, 1)
		_ = ws.SetReadDeadline(time.Time{})
		go func() {
			for {
				var m terminalMessage
				if err := websocket.JSON.Receive(ws, &m); err != nil {
					readDone <- err
					return
				}
				select {
				case incoming <- m:
				case <-ctx.Done():
					return
				}
			}
		}()
		verify := time.NewTicker(time.Second)
		defer verify.Stop()
		idle := time.NewTimer(5 * time.Minute)
		defer idle.Stop()
		inputBytes := 0
		window := time.Now()
		windowBytes := 0
		for {
			select {
			case <-ctx.Done():
				reason = "session_expired"
				return
			case <-idle.C:
				reason = "idle_timeout"
				_ = send(terminalMessage{Type: "end", Reason: "The terminal timed out"})
				return
			case <-verify.C:
				check, stop := context.WithTimeout(ctx, 3*time.Second)
				err := s.terminalOwned(check, t.record)
				stop()
				if err != nil {
					reason = err.Error()
					_ = send(terminalMessage{Type: "end", Reason: "Terminal access has ended"})
					return
				}
			case err := <-outputDone:
				if !errors.Is(err, io.EOF) {
					reason = "output_ended"
				} else {
					reason = "shell_exited"
				}
				_ = send(terminalMessage{Type: "end", Reason: "The shell has exited"})
				return
			case <-readDone:
				return
			case m := <-incoming:
				if ctx.Err() != nil {
					return
				}
				if time.Since(window) >= time.Second {
					window = time.Now()
					windowBytes = 0
				}
				windowBytes += len(m.Data)
				inputBytes += len(m.Data)
				if len(m.Data) > 8192 || windowBytes > 64<<10 || inputBytes > 4<<20 {
					reason = "input_limit"
					return
				}
				switch m.Type {
				case "input":
					if len(m.Data) > 0 {
						if _, err = terminal.Write([]byte(m.Data)); err != nil {
							return
						}
						if !idle.Stop() {
							select {
							case <-idle.C:
							default:
							}
						}
						idle.Reset(5 * time.Minute)
					}
				case "resize":
					resize, stop := context.WithTimeout(ctx, 2*time.Second)
					err = terminal.Resize(resize, m.Cols, m.Rows)
					stop()
					if err != nil {
						reason = "invalid_size"
						return
					}
				default:
					reason = "invalid_message"
					return
				}
			}
		}
	}}.ServeHTTP(w, r)
}
