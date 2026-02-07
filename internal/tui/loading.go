package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	loadingTitleStyle = lipgloss.NewStyle().MarginLeft(2).Bold(true)
	loadingInfoStyle  = lipgloss.NewStyle().MarginLeft(4)
)

type LoadingModel struct {
	message string
	spinner spinner.Model
}

func NewLoadingModel(message string) LoadingModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return LoadingModel{message: message, spinner: s}
}

func (m LoadingModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m LoadingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m LoadingModel) View() string {
	header := loadingTitleStyle.Render(m.message)
	status := loadingInfoStyle.Render(fmt.Sprintf("%s working...", m.spinner.View()))
	return lipgloss.JoinVertical(lipgloss.Left, header, status)
}
