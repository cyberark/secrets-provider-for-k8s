package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

const DefaultAddress = ":8080"

type Server struct {
	listener   net.Listener
	httpServer *http.Server
	isHealthy  atomic.Bool
	isReady    atomic.Bool
}

func NewServer(address string) (*Server, error) {
	if address == "" {
		address = DefaultAddress
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	server := &Server{listener: listener}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.healthHandler)
	mux.HandleFunc("/readyz", server.readyHandler)
	server.httpServer = &http.Server{Handler: mux}

	return server, nil
}

func (s *Server) Start() {
	log.Info(fmt.Sprintf(messages.CSPFK038I, s.Address()))
	go func() {
		err := s.httpServer.Serve(s.listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return
		}
	}()

	s.SetHealthy(true)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Address() string {
	return s.listener.Addr().String()
}

func (s *Server) SetHealthy(healthy bool) {
	s.isHealthy.Store(healthy)
	log.Info(messages.CSPFK039I, healthy)
}

func (s *Server) healthHandler(w http.ResponseWriter, _ *http.Request) {
	if s.isHealthy.Load() {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

func (s *Server) SetReady(ready bool) {
	s.isReady.Store(ready)
	log.Info(messages.CSPFK040I, ready)
}

func (s *Server) readyHandler(w http.ResponseWriter, _ *http.Request) {
	if s.isReady.Load() {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}
