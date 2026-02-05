package app

import (
	"context"
	"fmt"

	"cli-tg-chat-summary/internal/telegram"
	"cli-tg-chat-summary/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

type fetchResult struct {
	messages []telegram.Message
	err      error
}

type FetchOpts struct {
	Ctx   context.Context
	Title string
}

func (a *App) fetchWithProgress(opts FetchOpts, fetch func(context.Context, telegram.ProgressFunc) ([]telegram.Message, error)) ([]telegram.Message, error) {
	msgCh := make(chan tea.Msg, 128)
	resultCh := make(chan fetchResult, 1)
	fetchCtx, cancel := context.WithCancel(opts.Ctx)
	defer cancel()

	go func() {
		progressFn := func(update telegram.ProgressUpdate) {
			if fetchCtx.Err() != nil {
				return
			}
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

		messages, err := fetch(fetchCtx, progressFn)
		resultCh <- fetchResult{messages: messages, err: err}
		close(msgCh)
	}()

	model := tui.NewProgressModel(opts.Title, msgCh)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to run progress TUI: %w", err)
	}

	cancel()
	result := <-resultCh
	return result.messages, result.err
}
