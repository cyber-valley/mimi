package main

import (
	"context"
	"log"

	"mimi/internal/provider/telegram"
)

func main() {
	ctx := context.Background()
	err := telegram.StartClient(ctx, func(s telegram.ClientState) error {
		log.Println("client started")
		return nil
	})
	if err != nil {
		log.Fatalf("failed to start client with %#v", err)
	}
}
