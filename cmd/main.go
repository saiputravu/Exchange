package main

import (
	"context"
	server "fenrir/internal"
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

	srv := server.New("0.0.0.0", 9001)
	go srv.Run(ctx)

	// Block on running the server.
	<-ctx.Done()
}
