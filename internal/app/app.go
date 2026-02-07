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
	Since          time.Time
	Until          time.Time
	UseDateRange   bool
	ChatID         int64
	ChatIDRaw      int64
	TopicID        int
	TopicTitle     string
	NonInteractive bool
}

func (a *App) Run(ctx context.Context, opts RunOptions) error {
	// Login
	fmt.Println("Authenticating with Telegram...")
	if err := a.tgClient.Login(ctx, os.Stdin); err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}

	if opts.NonInteractive {
		return a.runNonInteractive(ctx, opts)
	}

	for {
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

		modelOpts := tui.ModelOptions{}
		if opts.UseDateRange {
			modelOpts.Mode = tui.ModeDateRange
			modelOpts.Since = opts.Since
			modelOpts.Until = opts.Until
		}
		model := tui.NewModel(chats, markReadFunc, modelOpts)
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

		mode := m.GetExportMode()
		if mode == tui.ModeDateRange {
			since, until, ok := m.GetDateRange()
			if ok {
				opts.UseDateRange = true
				opts.Since = since
				opts.Until = until
			}
		} else {
			opts.UseDateRange = false
		}

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
				continue
			}

			selectedTopic, err = a.chooseForumTopic(topics, opts)
			if err != nil {
				return err
			}
			if selectedTopic == nil {
				fmt.Println("No topic selected.")
				continue
			}
		}

		messages, exportTitle, err := a.fetchSelectedMessages(ctx, *selectedChat, selectedTopic, opts)
		if err != nil {
			return err
		}

		if len(messages) == 0 {
			fmt.Println("No text messages found to export.")
			continue
		}

		filename, err := a.exportMessages(exportTitle, messages, opts)
		if err != nil {
			return err
		}

		fmt.Printf("Successfully exported %d messages to %s\n", len(messages), filename)
		_ = a.markMessagesAsRead(ctx, *selectedChat, selectedTopic, messages, opts)
	}

}

func (a *App) runNonInteractive(ctx context.Context, opts RunOptions) error {
	fmt.Println("Fetching chats (this might take a moment)...")
	chats, err := a.tgClient.GetDialogs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get dialogs: %w", err)
	}

	if len(chats) == 0 {
		fmt.Println("No chats found.")
		return nil
	}

	selectedChat := findChatByID(chats, opts.ChatID)
	if selectedChat == nil {
		chatID := opts.ChatID
		if opts.ChatIDRaw != 0 {
			chatID = opts.ChatIDRaw
		}
		return fmt.Errorf("chat with id %d not found; accepts raw ID or -100... format", chatID)
	}

	var selectedTopic *telegram.Topic
	if selectedChat.IsForum {
		if opts.TopicID == 0 && opts.TopicTitle == "" {
			return fmt.Errorf("forum chat requires --topic-id or --topic")
		}
		fmt.Printf("Fetching topics for forum %s...\n", selectedChat.Title)
		topics, err := a.tgClient.GetForumTopics(ctx, selectedChat.ID)
		if err != nil {
			return fmt.Errorf("failed to get forum topics: %w", err)
		}
		selectedTopic, err = a.chooseForumTopic(topics, opts)
		if err != nil {
			return err
		}
		if selectedTopic == nil {
			return fmt.Errorf("forum chat requires --topic-id or --topic")
		}
	}

	messages, exportTitle, err := a.fetchSelectedMessages(ctx, *selectedChat, selectedTopic, opts)
	if err != nil {
		return err
	}

	if len(messages) == 0 {
		fmt.Println("No text messages found to export.")
		return nil
	}

	filename, err := a.exportMessages(exportTitle, messages, opts)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully exported %d messages to %s\n", len(messages), filename)
	return a.markMessagesAsRead(ctx, *selectedChat, selectedTopic, messages, opts)
}

func findChatByID(chats []telegram.Chat, chatID int64) *telegram.Chat {
	for i := range chats {
		if chats[i].ID == chatID {
			return &chats[i]
		}
	}
	return nil
}

func selectForumTopic(topics []telegram.Topic, topicID int, topicTitle string) (*telegram.Topic, error) {
	if topicID != 0 {
		for i := range topics {
			if topics[i].ID == topicID {
				return &topics[i], nil
			}
		}
		return nil, fmt.Errorf("forum topic id %d not found", topicID)
	}

	title := strings.TrimSpace(topicTitle)
	if title == "" {
		return nil, fmt.Errorf("forum chat requires --topic-id or --topic")
	}

	lowerTitle := strings.ToLower(title)
	var exactMatches []telegram.Topic
	for _, topic := range topics {
		if strings.EqualFold(topic.Title, title) {
			exactMatches = append(exactMatches, topic)
		}
	}
	if len(exactMatches) == 1 {
		return &exactMatches[0], nil
	}
	if len(exactMatches) > 1 {
		return nil, fmt.Errorf("forum topic title %q matched multiple topics: %s", title, formatTopicCandidates(exactMatches))
	}

	var containsMatches []telegram.Topic
	for _, topic := range topics {
		if strings.Contains(strings.ToLower(topic.Title), lowerTitle) {
			containsMatches = append(containsMatches, topic)
		}
	}
	if len(containsMatches) == 1 {
		return &containsMatches[0], nil
	}
	if len(containsMatches) > 1 {
		return nil, fmt.Errorf("forum topic title %q matched multiple topics: %s", title, formatTopicCandidates(containsMatches))
	}
	return nil, fmt.Errorf("forum topic title %q not found", title)
}

func (a *App) chooseForumTopic(topics []telegram.Topic, opts RunOptions) (*telegram.Topic, error) {
	if opts.NonInteractive {
		// Topic selection uses exact title match first, then falls back to contains.
		return selectForumTopic(topics, opts.TopicID, opts.TopicTitle)
	}

	// Show topic selection TUI
	topicModel := tui.NewTopicModel(topics)
	tp := tea.NewProgram(topicModel)
	finalTopicModel, err := tp.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run topic TUI: %w", err)
	}

	tm, ok := finalTopicModel.(tui.TopicModel)
	if !ok {
		return nil, fmt.Errorf("internal error: invalid topic model type")
	}

	return tm.GetSelected(), nil
}

func formatTopicCandidates(topics []telegram.Topic) string {
	var parts []string
	for _, topic := range topics {
		parts = append(parts, fmt.Sprintf("id=%d title=%q", topic.ID, topic.Title))
	}
	return strings.Join(parts, ", ")
}

func (a *App) fetchSelectedMessages(ctx context.Context, selectedChat telegram.Chat, selectedTopic *telegram.Topic, opts RunOptions) ([]telegram.Message, string, error) {
	var messages []telegram.Message
	var exportTitle string
	var err error

	if selectedChat.IsForum {
		if selectedTopic == nil {
			return nil, "", fmt.Errorf("forum chat requires --topic-id or --topic")
		}
		if opts.UseDateRange {
			progressTitle := fmt.Sprintf("%s / %s (%s to %s)", selectedChat.Title, selectedTopic.Title, opts.Since.Format("2006-01-02"), opts.Until.Format("2006-01-02"))
			messages, err = a.fetchWithProgress(FetchOpts{Ctx: ctx, Title: progressTitle}, func(ctx context.Context, progress telegram.ProgressFunc) ([]telegram.Message, error) {
				return a.tgClient.GetTopicMessagesByDate(ctx, selectedChat.ID, selectedTopic.ID, opts.Since, opts.Until, progress)
			})
		} else {
			progressTitle := fmt.Sprintf("%s / %s (unread)", selectedChat.Title, selectedTopic.Title)
			messages, err = a.fetchWithProgress(FetchOpts{Ctx: ctx, Title: progressTitle}, func(ctx context.Context, progress telegram.ProgressFunc) ([]telegram.Message, error) {
				return a.tgClient.GetTopicMessages(ctx, selectedChat.ID, selectedTopic.ID, selectedTopic.LastReadID, progress)
			})
		}
		if err != nil {
			return nil, "", fmt.Errorf("failed to get topic messages: %w", err)
		}
		exportTitle = selectedChat.Title + " - " + selectedTopic.Title
	} else {
		if opts.UseDateRange {
			progressTitle := fmt.Sprintf("%s (%s to %s)", selectedChat.Title, opts.Since.Format("2006-01-02"), opts.Until.Format("2006-01-02"))
			messages, err = a.fetchWithProgress(FetchOpts{Ctx: ctx, Title: progressTitle}, func(ctx context.Context, progress telegram.ProgressFunc) ([]telegram.Message, error) {
				return a.tgClient.GetMessagesByDate(ctx, selectedChat.ID, opts.Since, opts.Until, progress)
			})
		} else {
			progressTitle := fmt.Sprintf("%s (unread)", selectedChat.Title)
			messages, err = a.fetchWithProgress(FetchOpts{Ctx: ctx, Title: progressTitle}, func(ctx context.Context, progress telegram.ProgressFunc) ([]telegram.Message, error) {
				return a.tgClient.GetUnreadMessages(ctx, selectedChat.ID, selectedChat.LastReadID, progress)
			})
		}
		if err != nil {
			return nil, "", fmt.Errorf("failed to get messages: %w", err)
		}
		exportTitle = selectedChat.Title
	}

	return messages, exportTitle, nil
}

func (a *App) exportMessages(exportTitle string, messages []telegram.Message, opts RunOptions) (string, error) {
	// Sort messages by date (oldest first)
	// fetched messages are usually newest first from history?
	// `GetUnreadMessages` implementation appended them as they came.
	// If we used `MessagesGetHistory` without offset loop, we got newest first.
	// Let's reverse to have chronological order for reading.
	for i := len(messages)/2 - 1; i >= 0; i-- {
		opp := len(messages) - 1 - i
		messages[i], messages[opp] = messages[opp], messages[i]
	}

	// Export to file
	// format: ChatName_Date.txt or ChatName_TopicName_Date.txt
	// date range format: ChatName_YYYY-MM-DD_to_YYYY-MM-DD.txt
	filename, err := a.exporter.Export(exportTitle, messages, opts)
	if err != nil {
		return "", fmt.Errorf("failed to export: %w", err)
	}

	return filename, nil
}

func (a *App) markMessagesAsRead(ctx context.Context, selectedChat telegram.Chat, selectedTopic *telegram.Topic, messages []telegram.Message, opts RunOptions) error {
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
			err = a.tgClient.MarkAsRead(ctx, selectedChat, maxID)
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
