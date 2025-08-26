package tsnet

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"tailscale.com/client/local"
	"tailscale.com/tsnet"
)

type Server struct {
	srv      *tsnet.Server
	listener net.Listener
}

type Config struct {
	Hostname string
	AuthKey  string
	StateDir string
	UseHTTPS bool
	Port     string
}

func NewServer(config Config) (*Server, error) {
	srv := &tsnet.Server{}

	if config.Hostname != "" {
		srv.Hostname = config.Hostname
	}

	if config.AuthKey != "" {
		srv.AuthKey = config.AuthKey
	}

	if config.StateDir != "" {
		srv.Dir = config.StateDir
	}

	return &Server{
		srv: srv,
	}, nil
}

func (s *Server) Listen(config Config) error {
	port := config.Port
	if port == "" {
		if config.UseHTTPS {
			port = ":443"
		} else {
			port = ":80"
		}
	}

	ln, err := s.srv.Listen("tcp", port)
	if err != nil {
		return fmt.Errorf("failed to listen on Tailscale network: %w", err)
	}

	if config.UseHTTPS {
		lc, err := s.srv.LocalClient()
		if err != nil {
			return fmt.Errorf("failed to get LocalClient: %w", err)
		}

		ln = tls.NewListener(ln, &tls.Config{
			GetCertificate: lc.GetCertificate,
		})
	}

	s.listener = ln
	return nil
}

func (s *Server) Serve(handler http.Handler) error {
	if s.listener == nil {
		return fmt.Errorf("server not listening - call Listen() first")
	}

	return http.Serve(s.listener, handler)
}

func (s *Server) HTTPClient() *http.Client {
	return s.srv.HTTPClient()
}

func (s *Server) LocalClient() (*local.Client, error) {
	return s.srv.LocalClient()
}

func (s *Server) Close() error {
	if s.listener != nil {
		s.listener.Close()
	}
	return s.srv.Close()
}

func (s *Server) Addr() net.Addr {
	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}
