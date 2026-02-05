package app

import (
	"fmt"

	"cli-tg-chat-summary/internal/telegram"
	"cli-tg-chat-summary/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

type fetchResult struct {
	messages []telegram.Message
	err      error
}

func (a *App) fetchWithProgress(title string, fetch func(telegram.ProgressFunc) ([]telegram.Message, error)) ([]telegram.Message, error) {
	msgCh := make(chan tea.Msg, 128)
	resultCh := make(chan fetchResult, 1)

	go func() {
		progressFn := func(update telegram.ProgressUpdate) {
			select {
			case msgCh <- tui.ProgressMsg{
				Phase:   update.Phase,
				Parsed:  update.Parsed,
				Scanned: update.Scanned,
				Batch:   update.Batch,
			}:
			default:
			}
		}

		messages, err := fetch(progressFn)
		resultCh <- fetchResult{messages: messages, err: err}
		close(msgCh)
	}()

	model := tui.NewProgressModel(title, msgCh)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return nil, fmt.Errorf("failed to run progress TUI: %w", err)
	}

	result := <-resultCh
	return result.messages, result.err
}
