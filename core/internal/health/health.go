package health

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Checker verifies a component is healthy
type Checker interface {
	HealthCheck(ctx context.Context) error
}

// Server exposes a /health endpoint
type Server struct {
	checkers []Checker
	server   *http.Server
}

// New creates a health server on the given port
func New(port int) *Server {
	s := &Server{}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	return s
}

// AddChecker adds a component to check during health requests
func (s *Server) AddChecker(c Checker) {
	s.checkers = append(s.checkers, c)
}

// Start begins listening (non-blocking)
func (s *Server) Start() error {
	go s.server.ListenAndServe()
	return nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	for _, c := range s.checkers {
		if err := c.HealthCheck(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "unhealthy: %v", err)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
