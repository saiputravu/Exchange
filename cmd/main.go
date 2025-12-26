package main

import (
	"context"
	"fenrir/internal/common"
	"fenrir/internal/engine"
	"fenrir/internal/net"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	defer stop()

	// Setup the TCP server and the matching engine.
	var srv *net.Server
	var eng *engine.Engine
	eng = engine.New(srv, common.Equities)
	srv = net.New("0.0.0.0", 9001, eng)

	go srv.Run(ctx)
	// Block on running the server.
	<-ctx.Done()
}
