package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cli-tg-chat-summary/internal/config"
	"cli-tg-chat-summary/internal/telegram"
	"cli-tg-chat-summary/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
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
	fmt.Println("Authenticating with Telegram...")
	if err := tgClient.Login(os.Stdin); err != nil {
		log.Fatalf("failed to login: %v", err)
	}

	fmt.Println("Fetching chats (this might take a moment)...")
	chats, err := tgClient.GetDialogs()
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

	fmt.Printf("Fetching unread messages for %s...\n", selectedChat.Title)
	messages, err := tgClient.GetUnreadMessages(selectedChat.ID, selectedChat.LastReadID)
	if err != nil {
		log.Fatalf("failed to get messages: %v", err)
	}

	if len(messages) == 0 {
		fmt.Println("No text messages found to export.")
		return
	}

	// Export to file
	// format: ChatName_Date.txt
	cleanName := sanitizeFilename(selectedChat.Title)
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
	defer f.Close()

	fmt.Fprintf(f, "Chat Summary: %s\n", selectedChat.Title)
	fmt.Fprintf(f, "Export Date: %s\n", time.Now().Format(time.RFC1123))
	fmt.Fprintf(f, "Total Messages: %d\n\n", len(messages))

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
		fmt.Fprintf(f, "[%s] %s: %s\n", msg.Date.Format("15:04"), msg.Sender, strings.TrimSpace(msg.Text))
	}

	fmt.Printf("Successfully exported %d messages to %s\n", len(messages), filename)

	// Mark as read
	maxID := 0
	for _, msg := range messages {
		if msg.ID > maxID {
			maxID = msg.ID
		}
	}

	if maxID > 0 {
		fmt.Println("Marking messages as read...")
		if err := tgClient.MarkAsRead(*selectedChat, maxID); err != nil {
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
