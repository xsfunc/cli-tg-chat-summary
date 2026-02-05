package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"cli-tg-chat-summary/internal/config"
	"cli-tg-chat-summary/internal/telegram"
	"cli-tg-chat-summary/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

type App struct {
	cfg      *config.Config
	tgClient *telegram.Client
	exporter Exporter
}

func New(cfg *config.Config, tgClient *telegram.Client) *App {
	return NewWithExporter(cfg, tgClient, NewDefaultExporter())
}

func NewWithExporter(cfg *config.Config, tgClient *telegram.Client, exporter Exporter) *App {
	if exporter == nil {
		exporter = NewDefaultExporter()
	}
	return &App{
		cfg:      cfg,
		tgClient: tgClient,
		exporter: exporter,
	}
}

type RunOptions struct {
	Since        time.Time
	Until        time.Time
	UseDateRange bool
}

func (a *App) Run(ctx context.Context, opts RunOptions) error {
	// Login
	fmt.Println("Authenticating with Telegram...")
	if err := a.tgClient.Login(ctx, os.Stdin); err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}

	fmt.Println("Fetching chats (this might take a moment)...")
	chats, err := a.tgClient.GetDialogs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get dialogs: %w", err)
	}

	if len(chats) == 0 {
		fmt.Println("No chats found.")
		return nil
	}

	// Start TUI
	markReadFunc := func(chat telegram.Chat) error {
		if chat.IsForum {
			return fmt.Errorf("marking forums as read is not supported yet")
		}
		// Use TopMessageID to mark everything up to that message as read
		if chat.TopMessageID == 0 {
			return fmt.Errorf("no top message id found")
		}
		return a.tgClient.MarkAsRead(ctx, chat, chat.TopMessageID)
	}

	model := tui.NewModel(chats, markReadFunc)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	m, ok := finalModel.(tui.Model)
	if !ok {
		return fmt.Errorf("internal error: invalid model type")
	}

	selectedChat := m.GetSelected()
	if selectedChat == nil {
		fmt.Println("No chat selected.")
		return nil
	}

	var messages []telegram.Message
	var exportTitle string
	var selectedTopic *telegram.Topic

	// Handle forum topic selection
	if selectedChat.IsForum {
		fmt.Printf("Fetching topics for forum %s...\n", selectedChat.Title)
		topics, err := a.tgClient.GetForumTopics(ctx, selectedChat.ID)
		if err != nil {
			return fmt.Errorf("failed to get forum topics: %w", err)
		}

		if len(topics) == 0 {
			fmt.Println("No topics found in forum.")
			return nil
		}

		// Show topic selection TUI
		topicModel := tui.NewTopicModel(topics)
		tp := tea.NewProgram(topicModel)
		finalTopicModel, err := tp.Run()
		if err != nil {
			return fmt.Errorf("failed to run topic TUI: %w", err)
		}

		tm, ok := finalTopicModel.(tui.TopicModel)
		if !ok {
			return fmt.Errorf("internal error: invalid topic model type")
		}

		selectedTopic = tm.GetSelected()
		if selectedTopic == nil {
			fmt.Println("No topic selected.")
			return nil
		}

		if opts.UseDateRange {
			progressTitle := fmt.Sprintf("%s / %s (%s to %s)", selectedChat.Title, selectedTopic.Title, opts.Since.Format("2006-01-02"), opts.Until.Format("2006-01-02"))
			messages, err = a.fetchWithProgress(progressTitle, func(progress telegram.ProgressFunc) ([]telegram.Message, error) {
				return a.tgClient.GetTopicMessagesByDate(ctx, selectedChat.ID, selectedTopic.ID, opts.Since, opts.Until, progress)
			})
		} else {
			progressTitle := fmt.Sprintf("%s / %s (unread)", selectedChat.Title, selectedTopic.Title)
			messages, err = a.fetchWithProgress(progressTitle, func(progress telegram.ProgressFunc) ([]telegram.Message, error) {
				return a.tgClient.GetTopicMessages(ctx, selectedChat.ID, selectedTopic.ID, selectedTopic.LastReadID, progress)
			})
		}
		if err != nil {
			return fmt.Errorf("failed to get topic messages: %w", err)
		}
		exportTitle = selectedChat.Title + " - " + selectedTopic.Title
	} else {
		if opts.UseDateRange {
			progressTitle := fmt.Sprintf("%s (%s to %s)", selectedChat.Title, opts.Since.Format("2006-01-02"), opts.Until.Format("2006-01-02"))
			messages, err = a.fetchWithProgress(progressTitle, func(progress telegram.ProgressFunc) ([]telegram.Message, error) {
				return a.tgClient.GetMessagesByDate(ctx, selectedChat.ID, opts.Since, opts.Until, progress)
			})
		} else {
			progressTitle := fmt.Sprintf("%s (unread)", selectedChat.Title)
			messages, err = a.fetchWithProgress(progressTitle, func(progress telegram.ProgressFunc) ([]telegram.Message, error) {
				return a.tgClient.GetUnreadMessages(ctx, selectedChat.ID, selectedChat.LastReadID, progress)
			})
		}
		if err != nil {
			return fmt.Errorf("failed to get messages: %w", err)
		}
		exportTitle = selectedChat.Title
	}

	if len(messages) == 0 {
		fmt.Println("No text messages found to export.")
		return nil
	}

	// Export to file
	// format: ChatName_Date.txt or ChatName_TopicName_Date.txt
	// date range format: ChatName_YYYY-MM-DD_to_YYYY-MM-DD.txt
	filename, err := a.exporter.Export(exportTitle, messages, opts)
	if err != nil {
		return fmt.Errorf("failed to export: %w", err)
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

	fmt.Printf("Successfully exported %d messages to %s\n", len(messages), filename)

	// Mark as read
	maxID := 0
	for _, msg := range messages {
		if msg.ID > maxID {
			maxID = msg.ID
		}
	}

	if maxID > 0 && !opts.UseDateRange {
		fmt.Println("Marking messages as read...")
		var err error
		if selectedTopic != nil {
			err = a.tgClient.MarkTopicAsRead(ctx, selectedChat.ID, selectedTopic.ID, maxID)
		} else {
			err = a.tgClient.MarkAsRead(ctx, *selectedChat, maxID)
		}
		if err != nil {
			fmt.Printf("Warning: failed to mark messages as read: %v\n", err)
		} else {
			fmt.Println("Messages marked as read.")
		}
	}

	return nil
}

func sanitizeFilename(name string) string {
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	res := name
	for _, char := range invalid {
		res = strings.ReplaceAll(res, char, "_")
	}
	return strings.TrimSpace(res)
}
