package main

import (
	"context"
	server "fenrir/internal"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	defer stop()

	srv, err := server.New("0.0.0.0", 9001)
	if err != nil {
		log.Fatal().Err(err).Msg("server startup failed")
	}
	log.Info().Msg("server started up")

	// Block on running the server.
	srv.Run(ctx)
}
