package main

import (
	"context"
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

	srv := net.New("0.0.0.0", 9001)
	go srv.Run(ctx)

	// Block on running the server.
	<-ctx.Done()
}
