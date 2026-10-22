package dns

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/miekg/dns"
)

// Server coordinates UDP and TCP DNS listeners.
type Server struct {
	addr      string
	handler   *Handler
	udpServer *dns.Server
	tcpServer *dns.Server
	mu        sync.RWMutex
	running   bool
}

// NewServer creates a new Server instance.
func NewServer(addr string, handler *Handler) *Server {
	return &Server{
		addr:    addr,
		handler: handler,
	}
}

// Start boots the DNS listeners on UDP and TCP in separate goroutines.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return errors.New("server is already running")
	}

	udpConn, err := net.ListenPacket("udp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to bind UDP listener on %s: %w", s.addr, err)
	}

	tcpListener, err := net.Listen("tcp", s.addr)
	if err != nil {
		_ = udpConn.Close()
		return fmt.Errorf("failed to bind TCP listener on %s: %w", s.addr, err)
	}

	s.udpServer = &dns.Server{
		PacketConn: udpConn,
		Handler:    s.handler,
	}

	s.tcpServer = &dns.Server{
		Listener: tcpListener,
		Handler:  s.handler,
	}

	go func() {
		_ = s.udpServer.ActivateAndServe()
	}()

	go func() {
		_ = s.tcpServer.ActivateAndServe()
	}()

	s.running = true
	return nil
}

// Stop gracefully shuts down both UDP and TCP listeners.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false
	var errs []error

	if s.udpServer != nil {
		if err := s.udpServer.Shutdown(); err != nil {
			errs = append(errs, err)
		}
	}
	if s.tcpServer != nil {
		if err := s.tcpServer.Shutdown(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}
	return nil
}

// IsRunning reports whether the server is currently active.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *Server) isRunning() bool {
	return s.running
}

// Addr returns the configured listener address.
func (s *Server) Addr() string {
	return s.addr
}
