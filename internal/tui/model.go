package tui

import (
	"fmt"
	"io"
	"strings"

	"time"

	"cli-tg-chat-summary/internal/telegram"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
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
	errorStyle        = lipgloss.NewStyle().MarginLeft(2).Foreground(lipgloss.Color("160"))
	statusBarStyle    = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("62"))
)

const (
	minListWidth      = 20
	minListHeight     = 6
	defaultListWidth  = 60
	defaultListHeight = 14
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

type ExportMode int

const (
	ModeUnread ExportMode = iota
	ModeDateRange
)

type viewState int

const (
	stateChatList viewState = iota
	stateModeList
	stateSinceInput
	stateUntilInput
)

type ModelOptions struct {
	Mode  ExportMode
	Since time.Time
	Until time.Time
}

type Model struct {
	list         list.Model
	modeList     list.Model
	selected     *telegram.Chat
	quitting     bool
	done         bool
	canceled     bool
	markReadFunc func(telegram.Chat) error
	statusMsg    string
	errorMsg     string
	mode         ExportMode
	state        viewState
	sinceInput   textinput.Model
	untilInput   textinput.Model
	since        time.Time
	until        time.Time
}

type statusClearMsg struct{}

type modeItem struct {
	mode  ExportMode
	label string
}

func (i modeItem) FilterValue() string { return i.label }

func (i modeItem) Title() string { return i.label }

func (i modeItem) Description() string { return "" }

func NewModel(chats []telegram.Chat, markReadFunc func(telegram.Chat) error, opts ModelOptions) Model {
	items := make([]list.Item, len(chats))
	for i, chat := range chats {
		items[i] = item{chat: chat}
	}

	l := list.New(items, itemDelegate{}, defaultListWidth, defaultListHeight)
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
			key.NewBinding(
				key.WithKeys("m"),
				key.WithHelp("m", "mode"),
			),
		}
	}
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("ctrl+r"),
				key.WithHelp("^r", "mark read"),
			),
			key.NewBinding(
				key.WithKeys("m"),
				key.WithHelp("m", "mode"),
			),
		}
	}

	modeItems := []list.Item{
		modeItem{mode: ModeUnread, label: "Unread"},
		modeItem{mode: ModeDateRange, label: "Date range"},
	}
	modeList := list.New(modeItems, list.NewDefaultDelegate(), defaultListWidth, defaultListHeight)
	modeList.Title = "Select Export Mode"
	modeList.SetShowStatusBar(false)
	modeList.SetFilteringEnabled(false)
	modeList.Styles.Title = titleStyle
	modeList.Styles.PaginationStyle = paginationStyle
	modeList.Styles.HelpStyle = helpStyle

	sinceInput := textinput.New()
	sinceInput.Placeholder = "YYYY-MM-DD"
	sinceInput.CharLimit = 10
	sinceInput.Width = 12

	untilInput := textinput.New()
	untilInput.Placeholder = "YYYY-MM-DD"
	untilInput.CharLimit = 10
	untilInput.Width = 12

	mode := opts.Mode
	if mode != ModeDateRange {
		mode = ModeUnread
	}

	if !opts.Since.IsZero() {
		sinceInput.SetValue(opts.Since.Format("2006-01-02"))
	}
	if !opts.Until.IsZero() {
		untilInput.SetValue(opts.Until.Format("2006-01-02"))
	}

	return Model{
		list:         l,
		modeList:     modeList,
		markReadFunc: markReadFunc,
		mode:         mode,
		state:        stateChatList,
		sinceInput:   sinceInput,
		untilInput:   untilInput,
		since:        opts.Since,
		until:        opts.Until,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		applyChatListSize(&m.list, msg)
		applyModeListSize(&m.modeList, msg, 2)
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case stateModeList:
			switch keypress := msg.String(); keypress {
			case "ctrl+c", "esc":
				m.state = stateChatList
				return m, nil
			case "enter":
				i, ok := m.modeList.SelectedItem().(modeItem)
				if ok {
					if i.mode == ModeDateRange {
						m.state = stateSinceInput
						m.errorMsg = ""
						m.sinceInput.Focus()
						m.untilInput.Blur()
						return m, textinput.Blink
					}
					m.mode = ModeUnread
					m.state = stateChatList
				}
				return m, nil
			}
		case stateSinceInput:
			switch keypress := msg.String(); keypress {
			case "ctrl+c":
				m.quitting = true
				m.done = true
				m.canceled = true
				return m, nil
			case "esc":
				m.errorMsg = ""
				m.state = stateChatList
				m.sinceInput.Blur()
				return m, nil
			case "enter":
				value := strings.TrimSpace(m.sinceInput.Value())
				if value == "" {
					m.errorMsg = "Start date is required (YYYY-MM-DD)"
					return m, nil
				}
				parsed, err := time.Parse("2006-01-02", value)
				if err != nil {
					m.errorMsg = "Invalid date format. Use YYYY-MM-DD"
					return m, nil
				}
				m.since = parsed
				m.errorMsg = ""
				m.state = stateUntilInput
				m.sinceInput.Blur()
				m.untilInput.Focus()
				return m, textinput.Blink
			}
		case stateUntilInput:
			switch keypress := msg.String(); keypress {
			case "ctrl+c":
				m.quitting = true
				m.done = true
				m.canceled = true
				return m, nil
			case "esc":
				m.errorMsg = ""
				m.state = stateChatList
				m.untilInput.Blur()
				return m, nil
			case "enter":
				value := strings.TrimSpace(m.untilInput.Value())
				if value == "" {
					m.until = time.Now()
				} else {
					parsed, err := time.Parse("2006-01-02", value)
					if err != nil {
						m.errorMsg = "Invalid date format. Use YYYY-MM-DD"
						return m, nil
					}
					if parsed.Before(m.since) {
						m.errorMsg = "End date cannot be before start date"
						return m, nil
					}
					m.until = parsed.Add(24 * time.Hour).Add(-time.Nanosecond)
				}
				m.mode = ModeDateRange
				m.errorMsg = ""
				m.state = stateChatList
				m.untilInput.Blur()
				return m, nil
			}
		default:
			switch keypress := msg.String(); keypress {
			case "ctrl+c", "esc":
				m.quitting = true
				m.done = true
				m.canceled = true
				return m, nil

			case "q", "Q": // Handle both cases
				// If we are filtering, we should execute the filter logic (type the letter q)
				// unless the user specifically wants to quit.
				// However, standard intuitive behavior is "q" quits if not typing.
				if m.list.FilterState() != list.Filtering {
					m.quitting = true
					m.done = true
					m.canceled = true
					return m, nil
				}

			case "m":
				if m.list.FilterState() != list.Filtering {
					m.state = stateModeList
					m.errorMsg = ""
					return m, nil
				}

			case "enter":
				i, ok := m.list.SelectedItem().(item)
				if ok {
					m.selected = &i.chat
				}
				m.done = true
				return m, nil

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
		}

	case statusClearMsg:
		m.statusMsg = ""
		return m, nil
	}

	var cmd tea.Cmd
	switch m.state {
	case stateModeList:
		m.modeList, cmd = m.modeList.Update(msg)
	case stateSinceInput:
		m.sinceInput, cmd = m.sinceInput.Update(msg)
	case stateUntilInput:
		m.untilInput, cmd = m.untilInput.Update(msg)
	default:
		m.list, cmd = m.list.Update(msg)
	}
	return m, cmd
}

func (m Model) View() string {
	if m.quitting {
		return quitTextStyle.Render("Bye!")
	}
	if m.selected != nil {
		return quitTextStyle.Render(fmt.Sprintf("Selected: %s", m.selected.Title))
	}

	switch m.state {
	case stateModeList:
		return m.modeList.View() + "\n" + helpStyle.Render("enter: select  esc: back")
	case stateSinceInput:
		return renderDateInput("Start date (YYYY-MM-DD)", m.sinceInput, m.errorMsg)
	case stateUntilInput:
		return renderDateInput("End date (YYYY-MM-DD, optional)", m.untilInput, m.errorMsg)
	default:
		view := m.list.View()
		view += "\n" + renderStatusBar(m.mode, m.statusMsg, m.currentChat())
		return view
	}
}

func (m Model) GetSelected() *telegram.Chat {
	return m.selected
}

func (m Model) Done() bool {
	return m.done
}

func (m Model) Canceled() bool {
	return m.canceled
}

func (m Model) GetExportMode() ExportMode {
	return m.mode
}

func (m Model) GetDateRange() (time.Time, time.Time, bool) {
	if m.mode != ModeDateRange {
		return time.Time{}, time.Time{}, false
	}
	return m.since, m.until, true
}

func modeLabel(mode ExportMode) string {
	switch mode {
	case ModeDateRange:
		return "Date range"
	default:
		return "Unread"
	}
}

func renderDateInput(title string, input textinput.Model, errMsg string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n  ")
	b.WriteString(input.View())
	if errMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render(errMsg))
	}
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("enter: confirm  esc: cancel"))
	return b.String()
}

func renderStatusBar(mode ExportMode, statusMsg string, chat *telegram.Chat) string {
	parts := []string{"Mode: " + modeLabel(mode)}
	if chat != nil {
		parts = append(parts, fmt.Sprintf("ID: %d", chat.ID))
	}
	if statusMsg != "" {
		parts = append(parts, statusMsg)
	}
	return statusBarStyle.Render(strings.Join(parts, " | "))
}

func (m Model) currentChat() *telegram.Chat {
	i, ok := m.list.SelectedItem().(item)
	if !ok {
		return nil
	}
	return &i.chat
}

func listWidthForWindow(width int) int {
	adjusted := width - 2
	if adjusted < minListWidth {
		return minListWidth
	}
	return adjusted
}

func listHeightForWindow(height, extraLines int) int {
	adjusted := height - extraLines
	if adjusted < minListHeight {
		return minListHeight
	}
	return adjusted
}

func applyChatListSize(l *list.Model, msg tea.WindowSizeMsg) {
	width := listWidthForWindow(msg.Width)
	l.SetWidth(width)
	l.SetHeight(listHeightForWindow(msg.Height, 2))
}

func applyModeListSize(l *list.Model, msg tea.WindowSizeMsg, extraLines int) {
	width := listWidthForWindow(msg.Width)
	l.SetWidth(width)
	l.SetHeight(listHeightForWindow(msg.Height, extraLines))
}

// TopicModel is a TUI model for selecting forum topics
type TopicModel struct {
	list     list.Model
	selected *telegram.Topic
	quitting bool
	done     bool
	canceled bool
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

	l := list.New(items, topicItemDelegate{}, defaultListWidth, defaultListHeight)
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
		applyModeListSize(&m.list, msg, 0)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "esc":
			m.quitting = true
			m.done = true
			m.canceled = true
			return m, nil

		case "q", "Q":
			if m.list.FilterState() != list.Filtering {
				m.quitting = true
				m.done = true
				m.canceled = true
				return m, nil
			}

		case "enter":
			i, ok := m.list.SelectedItem().(topicItem)
			if ok {
				m.selected = &i.topic
			}
			m.done = true
			return m, nil
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

func (m TopicModel) Done() bool {
	return m.done
}

func (m TopicModel) Canceled() bool {
	return m.canceled
}
