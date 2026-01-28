package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"cli-tg-chat-summary/internal/app"
	"cli-tg-chat-summary/internal/config"
	"cli-tg-chat-summary/internal/telegram"
)

func main() {
	var sinceStr, untilStr string
	flag.StringVar(&sinceStr, "since", "", "Start date (YYYY-MM-DD)")
	flag.StringVar(&untilStr, "until", "", "End date (YYYY-MM-DD)")
	flag.Parse()

	var opts app.RunOptions
	var err error

	if sinceStr != "" {
		opts.UseDateRange = true
		opts.Since, err = time.Parse("2006-01-02", sinceStr)
		if err != nil {
			fmt.Printf("Error: Invalid date format for --since: %s\n", sinceStr)
			fmt.Println("Please use the format YYYY-MM-DD (e.g., 2024-01-20)")
			os.Exit(1)
		}
		if untilStr != "" {
			opts.Until, err = time.Parse("2006-01-02", untilStr)
			if err != nil {
				fmt.Printf("Error: Invalid date format for --until: %s\n", untilStr)
				fmt.Println("Please use the format YYYY-MM-DD (e.g., 2024-01-20)")
				os.Exit(1)
			}
			// set until to end of that day
			opts.Until = opts.Until.Add(24 * time.Hour).Add(-time.Nanosecond)
		} else {
			opts.Until = time.Now()
		}
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Setup context
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Initialize Telegram client
	tgClient, err := telegram.NewClient(cfg)
	if err != nil {
		log.Fatalf("failed to create telegram client: %v", err)
	}

	// Initialize function app
	application := app.New(cfg, tgClient)

	// Run application
	if err := application.Run(ctx, opts); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
