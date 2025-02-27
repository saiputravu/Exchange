package server

import (
	"context"
	"fmt"
	"net"
	"strings"

	"fenrir/internal/protocol"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Server struct {
	protocol.UnimplementedDebugServer
	ctx    context.Context
	cancel context.CancelFunc

	srvID       uint32
	address     string
	port        uint16
	connections uint32
}

func NewServer(ctx context.Context, cancel context.CancelFunc, srvID uint32, address string, port uint16) *Server {
	return &Server{
		ctx:     ctx,
		cancel:  cancel,
		srvID:   srvID,
		address: address,
		port:    port,
	}
}

// Destroys the server context, and signals to running routines to issue a cleanup.
func (s *Server) Shutdown() {
	s.cancel()
}

func (s *Server) Run() error {
	var opts []grpc.ServerOption

	// FIXME: This should be configured to use TLS/SSL
	opts = append(opts, grpc.Creds(insecure.NewCredentials()))

	grpcServer := grpc.NewServer(opts...)
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.address, s.port))
	if err != nil {
		return err
	}

	protocol.RegisterDebugServer(grpcServer, s)

	println("Running server")
	return grpcServer.Serve(listener)
}

// ---- Utility Methods ----
func (s Server) String() (string, error) {
	var sb strings.Builder

	_, err := sb.WriteString(fmt.Sprintf("Address: %s\n", s.address))
	if err != nil {
		return "", err
	}
	_, err = sb.WriteString(fmt.Sprintf("Port:     %d\n", s.port))
	if err != nil {
		return "", err
	}

	return sb.String(), nil
}

// ---- Debug Server Implementations ----
func (s Server) QueryServer(context.Context, *protocol.Empty) (*protocol.ServerInfo, error) {
	srvInfo := protocol.ServerInfo{
		Type:        0,
		Id:          s.srvID,
		Port:        (uint32)(s.port),
		Connections: s.connections,
	}

	return &srvInfo, nil
}
