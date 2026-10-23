package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("shelly-agent starting")
	if err := run(ctx); err != nil {
		log.Printf("shelly-agent failed: %v", err)
		os.Exit(1)
	}
}
