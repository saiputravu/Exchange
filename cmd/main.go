package main

import (
	"context"

	server "fenrir/internal"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	srv := server.Create(ctx, cancel)

	// Startup the server.
	go srv.Run()

	// Wait until the context is finished.
	<-ctx.Done()
}
