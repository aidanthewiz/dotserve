package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := &App{}
	if err := app.initConfig(); err != nil {
		log.Fatalf("Failed to validate config: %v", err)
	}

	if err := app.startServer(ctx); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	sig := waitForShutdownSignal()
	log.Printf("Signal (%v) received, shutting down gracefully...\n", sig.String())
	app.shutdownGracefully(ctx)
}

func waitForShutdownSignal() os.Signal {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	return <-stop
}
