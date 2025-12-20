package main

import (
	"context"
	server "fenrir/internal"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := server.New()

	// Block on running the server.
	srv.Run(ctx)
}
