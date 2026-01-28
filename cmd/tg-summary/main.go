package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"cli-tg-chat-summary/internal/config"
	"cli-tg-chat-summary/internal/telegram"
	"cli-tg-chat-summary/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var sinceStr, untilStr string
	flag.StringVar(&sinceStr, "since", "", "Start date (YYYY-MM-DD)")
	flag.StringVar(&untilStr, "until", "", "End date (YYYY-MM-DD)")
	flag.Parse()

	var sinceTime, untilTime time.Time
	var useDateRange bool
	var err error

	if sinceStr != "" {
		useDateRange = true
		sinceTime, err = time.Parse("2006-01-02", sinceStr)
		if err != nil {
			fmt.Printf("Error: Invalid date format for --since: %s\n", sinceStr)
			fmt.Println("Please use the format YYYY-MM-DD (e.g., 2024-01-20)")
			os.Exit(1)
		}
		if untilStr != "" {
			untilTime, err = time.Parse("2006-01-02", untilStr)
			if err != nil {
				fmt.Printf("Error: Invalid date format for --until: %s\n", untilStr)
				fmt.Println("Please use the format YYYY-MM-DD (e.g., 2024-01-20)")
				os.Exit(1)
			}
			// set until to end of that day
			untilTime = untilTime.Add(24 * time.Hour).Add(-time.Nanosecond)
		} else {
			untilTime = time.Now()
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

	// Login
	fmt.Println("Authenticating with Telegram...")
	if err := tgClient.Login(ctx, os.Stdin); err != nil {
		log.Fatalf("failed to login: %v", err)
	}

	fmt.Println("Fetching chats (this might take a moment)...")
	chats, err := tgClient.GetDialogs(ctx)
	if err != nil {
		log.Fatalf("failed to get dialogs: %v", err)
	}

	if len(chats) == 0 {
		fmt.Println("No chats found.")
		return
	}

	// Start TUI
	model := tui.NewModel(chats)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		log.Fatalf("failed to run TUI: %v", err)
	}

	m, ok := finalModel.(tui.Model)
	if !ok {
		log.Fatalf("internal error: invalid model type")
	}

	selectedChat := m.GetSelected()
	if selectedChat == nil {
		fmt.Println("No chat selected.")
		return
	}

	var messages []telegram.Message
	var exportTitle string
	var selectedTopic *telegram.Topic

	// Handle forum topic selection
	if selectedChat.IsForum {
		fmt.Printf("Fetching topics for forum %s...\n", selectedChat.Title)
		topics, err := tgClient.GetForumTopics(ctx, selectedChat.ID)
		if err != nil {
			log.Fatalf("failed to get forum topics: %v", err)
		}

		if len(topics) == 0 {
			fmt.Println("No topics found in forum.")
			return
		}

		// Show topic selection TUI
		topicModel := tui.NewTopicModel(topics)
		tp := tea.NewProgram(topicModel)
		finalTopicModel, err := tp.Run()
		if err != nil {
			log.Fatalf("failed to run topic TUI: %v", err)
		}

		tm, ok := finalTopicModel.(tui.TopicModel)
		if !ok {
			log.Fatalf("internal error: invalid topic model type")
		}

		selectedTopic = tm.GetSelected()
		if selectedTopic == nil {
			fmt.Println("No topic selected.")
			return
		}

		if useDateRange {
			fmt.Printf("Fetching messages for topic %s from %s to %s...\n", selectedTopic.Title, sinceTime.Format("2006-01-02"), untilTime.Format("2006-01-02"))
			messages, err = tgClient.GetTopicMessagesByDate(ctx, selectedChat.ID, selectedTopic.ID, sinceTime, untilTime)
		} else {
			fmt.Printf("Fetching unread messages for topic %s...\n", selectedTopic.Title)
			messages, err = tgClient.GetTopicMessages(ctx, selectedChat.ID, selectedTopic.ID, selectedTopic.LastReadID)
		}
		if err != nil {
			log.Fatalf("failed to get topic messages: %v", err)
		}
		exportTitle = selectedChat.Title + " - " + selectedTopic.Title
	} else {
		if useDateRange {
			fmt.Printf("Fetching messages for %s from %s to %s...\n", selectedChat.Title, sinceTime.Format("2006-01-02"), untilTime.Format("2006-01-02"))
			messages, err = tgClient.GetMessagesByDate(ctx, selectedChat.ID, sinceTime, untilTime)
		} else {
			fmt.Printf("Fetching unread messages for %s...\n", selectedChat.Title)
			messages, err = tgClient.GetUnreadMessages(ctx, selectedChat.ID, selectedChat.LastReadID)
		}
		if err != nil {
			log.Fatalf("failed to get messages: %v", err)
		}
		exportTitle = selectedChat.Title
	}

	if len(messages) == 0 {
		fmt.Println("No text messages found to export.")
		return
	}

	// Export to file
	// format: ChatName_Date.txt or ChatName_TopicName_Date.txt
	cleanName := sanitizeFilename(exportTitle)
	dateStr := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("exports/%s_%s.txt", cleanName, dateStr)

	// Ensure exports directory exists
	cwd, _ := os.Getwd()
	if err := os.MkdirAll(filepath.Join(cwd, "exports"), 0755); err != nil {
		log.Fatalf("failed to create exports directory: %v", err)
	}

	// Ensure filename is absolute or relative to cur dir
	fullPath := filepath.Join(cwd, filename)

	f, err := os.Create(fullPath)
	if err != nil {
		log.Fatalf("failed to create file: %v", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("failed to close file: %v", err)
		}
	}()

	if _, err := fmt.Fprintf(f, "Chat Summary: %s\n", exportTitle); err != nil {
		log.Printf("failed to write header: %v", err)
	}
	if _, err := fmt.Fprintf(f, "Export Date: %s\n", time.Now().Format(time.RFC1123)); err != nil {
		log.Printf("failed to write date: %v", err)
	}
	if _, err := fmt.Fprintf(f, "Total Messages: %d\n\n", len(messages)); err != nil {
		log.Printf("failed to write count: %v", err)
	}

	// Sort messages by date (oldest first)
	// fetched messages are usually newest first from history?
	// `GetUnreadMessages` implementation appended them as they came.
	// If we used `MessagesGetHistory` without offset loop, we got newest first.
	// Let's reverse to have chronological order for reading.
	for i := len(messages)/2 - 1; i >= 0; i-- {
		opp := len(messages) - 1 - i
		messages[i], messages[opp] = messages[opp], messages[i]
	}

	for _, msg := range messages {
		if _, err := fmt.Fprintf(f, "[%s] %s: %s\n", msg.Date.Format("15:04"), msg.Sender, strings.TrimSpace(msg.Text)); err != nil {
			log.Printf("failed to write message: %v", err)
		}
	}

	fmt.Printf("Successfully exported %d messages to %s\n", len(messages), filename)

	// Mark as read
	maxID := 0
	for _, msg := range messages {
		if msg.ID > maxID {
			maxID = msg.ID
		}
	}

	if maxID > 0 && !useDateRange {
		fmt.Println("Marking messages as read...")
		var err error
		if selectedTopic != nil {
			err = tgClient.MarkTopicAsRead(ctx, selectedChat.ID, selectedTopic.ID, maxID)
		} else {
			err = tgClient.MarkAsRead(ctx, *selectedChat, maxID)
		}
		if err != nil {
			fmt.Printf("Warning: failed to mark messages as read: %v\n", err)
		} else {
			fmt.Println("Messages marked as read.")
		}
	}
}

func sanitizeFilename(name string) string {
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	res := name
	for _, char := range invalid {
		res = strings.ReplaceAll(res, char, "_")
	}
	// Remove emojis or other complex chars?
	// Basic sanitization should be enough for linux.
	return strings.TrimSpace(res)
}
