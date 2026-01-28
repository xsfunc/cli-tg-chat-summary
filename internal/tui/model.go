package tui

import (
	"fmt"
	"io"
	"strings"

	"time"

	"cli-tg-chat-summary/internal/telegram"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	quitTextStyle     = lipgloss.NewStyle().Margin(1, 0, 2, 4)
)

type item struct {
	chat telegram.Chat
}

func (i item) FilterValue() string { return i.chat.Title }

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%s (%d unread)", i.chat.Title, i.chat.UnreadCount)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(s, " "))
		}
	}

	_, _ = fmt.Fprint(w, fn(str))
}

type Model struct {
	list         list.Model
	selected     *telegram.Chat
	quitting     bool
	markReadFunc func(telegram.Chat) error
	statusMsg    string
}

type statusClearMsg struct{}

func NewModel(chats []telegram.Chat, markReadFunc func(telegram.Chat) error) Model {
	items := make([]list.Item, len(chats))
	for i, chat := range chats {
		items[i] = item{chat: chat}
	}

	const defaultWidth = 20
	const listHeight = 14

	l := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	l.Title = "Select Chat to Summarize"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	l.Styles.HelpStyle = helpStyle

	// Add more help keys
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("ctrl+r"),
				key.WithHelp("ctrl+r", "mark read"),
			),
		}
	}
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("ctrl+r"),
				key.WithHelp("^r", "mark read"),
			),
		}
	}

	return Model{list: l, markReadFunc: markReadFunc}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit

		case "q", "Q": // Handle both cases
			// If we are filtering, we should execute the filter logic (type the letter q)
			// unless the user specifically wants to quit.
			// However, standard intuitive behavior is "q" quits if not typing.
			if m.list.FilterState() != list.Filtering {
				m.quitting = true
				return m, tea.Quit
			}

		case "enter":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.selected = &i.chat
			}
			return m, tea.Quit

		case "ctrl+r":
			if m.list.FilterState() == list.Filtering {
				// Don't intercept if filtering (though ctrl+r is unlikely to be typed text everywhere, but better safe)
				// Actually ctrl+r is usually safe.
				// But let's keep it safe.
				break
			}
			if m.markReadFunc == nil {
				m.statusMsg = "Error: Mark as read not implemented"
				return m, nil
			}

			i, ok := m.list.SelectedItem().(item)
			if ok {
				err := m.markReadFunc(i.chat)
				if err != nil {
					m.statusMsg = fmt.Sprintf("Error: %v", err)
				} else {
					m.statusMsg = fmt.Sprintf("Marked %s as read", i.chat.Title)

					// Update the item in the list directly to show 0 unread
					idx := m.list.Index()
					newChat := i.chat
					newChat.UnreadCount = 0

					// Update the items list
					items := m.list.Items()
					items[idx] = item{chat: newChat}
					m.list.SetItems(items)
				}
				// Clear status after 2 seconds
				return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return statusClearMsg{}
				})
			}
		}

	case statusClearMsg:
		m.statusMsg = ""
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.quitting {
		return quitTextStyle.Render("Bye!")
	}
	if m.selected != nil {
		return quitTextStyle.Render(fmt.Sprintf("Selected: %s", m.selected.Title))
	}

	view := m.list.View()
	if m.statusMsg != "" {
		view += "\n" + quitTextStyle.Foreground(lipgloss.Color("62")).Render(m.statusMsg)
	}
	return view
}

func (m Model) GetSelected() *telegram.Chat {
	return m.selected
}

// TopicModel is a TUI model for selecting forum topics
type TopicModel struct {
	list     list.Model
	selected *telegram.Topic
	quitting bool
}

type topicItem struct {
	topic telegram.Topic
}

func (i topicItem) FilterValue() string { return i.topic.Title }

type topicItemDelegate struct{}

func (d topicItemDelegate) Height() int                             { return 1 }
func (d topicItemDelegate) Spacing() int                            { return 0 }
func (d topicItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d topicItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(topicItem)
	if !ok {
		return
	}

	str := fmt.Sprintf("%s (%d unread)", i.topic.Title, i.topic.UnreadCount)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(s, " "))
		}
	}

	_, _ = fmt.Fprint(w, fn(str))
}

func NewTopicModel(topics []telegram.Topic) TopicModel {
	items := make([]list.Item, len(topics))
	for i, topic := range topics {
		items[i] = topicItem{topic: topic}
	}

	const defaultWidth = 20
	const listHeight = 14

	l := list.New(items, topicItemDelegate{}, defaultWidth, listHeight)
	l.Title = "Select Topic to Summarize"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	return TopicModel{list: l}
}

func (m TopicModel) Init() tea.Cmd {
	return nil
}

func (m TopicModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit

		case "q", "Q":
			if m.list.FilterState() != list.Filtering {
				m.quitting = true
				return m, tea.Quit
			}

		case "enter":
			i, ok := m.list.SelectedItem().(topicItem)
			if ok {
				m.selected = &i.topic
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m TopicModel) View() string {
	if m.quitting {
		return quitTextStyle.Render("Bye!")
	}
	if m.selected != nil {
		return quitTextStyle.Render(fmt.Sprintf("Selected topic: %s", m.selected.Title))
	}
	return m.list.View()
}

func (m TopicModel) GetSelected() *telegram.Topic {
	return m.selected
}
