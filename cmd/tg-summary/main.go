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
	var formatName string
	var chatIDRaw int64
	var topicID int
	var topicTitle string
	flag.StringVar(&sinceStr, "since", "", "Start date (YYYY-MM-DD)")
	flag.StringVar(&untilStr, "until", "", "End date (YYYY-MM-DD)")
	flag.StringVar(&formatName, "format", "text", "Export format (text, xml, xml-compact)")
	flag.Int64Var(&chatIDRaw, "id", 0, "Chat ID (raw or -100... format) to export without TUI")
	flag.IntVar(&topicID, "topic-id", 0, "Forum topic ID (required for forum chats in non-interactive mode)")
	flag.StringVar(&topicTitle, "topic", "", "Forum topic title (alternative to --topic-id)")
	flag.Parse()

	var opts app.RunOptions
	var err error
	opts.ExportFormat = formatName

	if chatIDRaw != 0 {
		opts.NonInteractive = true
		opts.ChatIDRaw = chatIDRaw
		// Telegram Bot API uses -100... IDs for channels/supergroups.
		// Dialogs return raw ChannelID, so normalize by stripping the -100 prefix.
		opts.ChatID = normalizeChatID(chatIDRaw)
		opts.TopicID = topicID
		opts.TopicTitle = topicTitle
	}

	if (topicID != 0 || topicTitle != "") && chatIDRaw == 0 {
		fmt.Fprintln(os.Stderr, "Error: --topic-id/--topic requires --id")
		os.Exit(1)
	}

	if sinceStr != "" {
		opts.UseDateRange = true
		opts.Since, err = time.Parse("2006-01-02", sinceStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid date format for --since: %s\n", sinceStr)
			fmt.Fprintln(os.Stderr, "Please use the format YYYY-MM-DD (e.g., 2024-01-20)")
			os.Exit(1)
		}
		if untilStr != "" {
			opts.Until, err = time.Parse("2006-01-02", untilStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Invalid date format for --until: %s\n", untilStr)
				fmt.Fprintln(os.Stderr, "Please use the format YYYY-MM-DD (e.g., 2024-01-20)")
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

func normalizeChatID(id int64) int64 {
	if id <= -1000000000000 {
		return -id - 1000000000000
	}
	return id
}
