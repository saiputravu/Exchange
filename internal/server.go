package server

import (
	"context"
	"fmt"
	"time"
)

type Server struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func Create(ctx context.Context, cancel context.CancelFunc) Server {
	return Server{ctx, cancel}
}

// Destroys the server context, and signals to running routines to issue a cleanup.
func (s *Server) Shutdown() {
	s.cancel()
}

func (s *Server) Run() {
	i := 0
	for {
		fmt.Println("Fenrir server alive for", i, "seconds")
		time.Sleep(time.Second)
		i++
	}
}
