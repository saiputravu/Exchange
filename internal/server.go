package server

import (
	"context"
	"fenrir/internal/protocol"
	"fmt"
	"time"
)

type Server struct {
	*protocol.UnimplementedDebugServer
}

func New() *Server {
	return &Server{}
}

func (s *Server) Shutdown() {
}

func (s *Server) Run(ctx context.Context) {
	i := 0
	for {
		select {
		case <-ctx.Done():
			s.Shutdown()
		default:
			fmt.Println("Fenrir server alive for", i, "seconds")
			time.Sleep(time.Second)
			i++
		}
	}
}
