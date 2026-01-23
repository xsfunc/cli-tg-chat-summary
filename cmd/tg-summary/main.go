package main

import (
	"fmt"
	"log"
	"os"

	"cli-tg-chat-summary/internal/config"
	"cli-tg-chat-summary/internal/telegram"
)

func main() {
	fmt.Println("Starting Telegram Chat Summary CLI...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Initialize Telegram client
	tgClient, err := telegram.NewClient(cfg)
	if err != nil {
		log.Fatalf("failed to create telegram client: %v", err)
	}

	// Login
	if err := tgClient.Login(os.Stdin); err != nil {
		log.Fatalf("failed to login: %v", err)
	}

	fmt.Println("Successfully authorized!")

	// TODO: Fetch chats with unread messages
	// TODO: Summarize using LLM
}
