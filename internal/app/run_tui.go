package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) runInteractiveTUI(ctx context.Context, opts RunOptions) error {
	model := newAppModel(a, ctx, opts)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}
	m, ok := finalModel.(appModel)
	if !ok {
		return fmt.Errorf("internal error: invalid model type")
	}
	if m.err != nil {
		return m.err
	}
	return nil
}
