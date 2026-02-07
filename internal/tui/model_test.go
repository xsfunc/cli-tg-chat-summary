package tui

import (
	"testing"

	"cli-tg-chat-summary/internal/telegram"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	chats := []telegram.Chat{
		{ID: 1, Title: "Chat 1", UnreadCount: 5},
		{ID: 2, Title: "Chat 2", UnreadCount: 10},
		{ID: 3, Title: "Chat 3", UnreadCount: 0},
	}

	model := NewModel(chats, nil, ModelOptions{})

	if model.selected != nil {
		t.Error("expected selected to be nil initially")
	}
	if model.quitting {
		t.Error("expected quitting to be false initially")
	}
	if model.list.Title != "Select Chat to Summarize" {
		t.Errorf("unexpected list title: %s", model.list.Title)
	}
}

func TestNewModel_EmptyChats(t *testing.T) {
	model := NewModel([]telegram.Chat{}, nil, ModelOptions{})

	if model.selected != nil {
		t.Error("expected selected to be nil")
	}
}

func TestModel_Init(t *testing.T) {
	model := NewModel([]telegram.Chat{}, nil, ModelOptions{})
	cmd := model.Init()

	if cmd != nil {
		t.Error("expected Init to return nil")
	}
}

func TestModel_Update_Quit_CtrlC(t *testing.T) {
	model := NewModel([]telegram.Chat{{ID: 1, Title: "Test"}}, nil, ModelOptions{})

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	newModel, cmd := model.Update(msg)

	m := newModel.(Model)
	if !m.quitting {
		t.Error("expected quitting to be true after Ctrl+C")
	}
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestModel_Update_Quit_Esc(t *testing.T) {
	model := NewModel([]telegram.Chat{{ID: 1, Title: "Test"}}, nil, ModelOptions{})

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, cmd := model.Update(msg)

	m := newModel.(Model)
	if !m.quitting {
		t.Error("expected quitting to be true after Esc")
	}
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestModel_Update_Quit_Q(t *testing.T) {
	model := NewModel([]telegram.Chat{{ID: 1, Title: "Test"}}, nil, ModelOptions{})

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newModel, cmd := model.Update(msg)

	m := newModel.(Model)
	if !m.quitting {
		t.Error("expected quitting to be true after 'q'")
	}
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestModel_Update_Enter(t *testing.T) {
	chats := []telegram.Chat{
		{ID: 1, Title: "First Chat", UnreadCount: 5},
		{ID: 2, Title: "Second Chat", UnreadCount: 10},
	}
	model := NewModel(chats, nil, ModelOptions{})

	// Press Enter to select the first item
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, cmd := model.Update(msg)

	m := newModel.(Model)
	if m.selected == nil {
		t.Error("expected selected to not be nil after Enter")
	} else if m.selected.ID != 1 {
		t.Errorf("expected selected chat ID 1, got %d", m.selected.ID)
	}
	if cmd == nil {
		t.Error("expected quit command after selection")
	}
}

func TestModel_Update_WindowSize(t *testing.T) {
	model := NewModel([]telegram.Chat{{ID: 1, Title: "Test"}}, nil, ModelOptions{})

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newModel, cmd := model.Update(msg)

	m := newModel.(Model)
	// Model should handle window resize without crashing
	if m.quitting {
		t.Error("model should not be quitting after window resize")
	}
	if cmd != nil {
		t.Error("expected no command from window resize")
	}
}

func TestModel_View(t *testing.T) {
	model := NewModel([]telegram.Chat{{ID: 1, Title: "Test"}}, nil, ModelOptions{})
	view := model.View()

	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestModel_View_Quitting(t *testing.T) {
	model := NewModel([]telegram.Chat{{ID: 1, Title: "Test"}}, nil, ModelOptions{})
	model.quitting = true

	view := model.View()

	if view == "" {
		t.Error("expected non-empty view when quitting")
	}
}

func TestModel_View_Selected(t *testing.T) {
	model := NewModel([]telegram.Chat{{ID: 1, Title: "Test Chat"}}, nil, ModelOptions{})
	model.selected = &telegram.Chat{ID: 1, Title: "Test Chat"}

	view := model.View()

	if view == "" {
		t.Error("expected non-empty view when selected")
	}
}

func TestModel_GetSelected(t *testing.T) {
	model := NewModel([]telegram.Chat{}, nil, ModelOptions{})

	if model.GetSelected() != nil {
		t.Error("expected nil when nothing selected")
	}

	expectedChat := &telegram.Chat{ID: 42, Title: "Selected"}
	model.selected = expectedChat

	if model.GetSelected() != expectedChat {
		t.Error("GetSelected returned wrong chat")
	}
}

func TestItem_FilterValue(t *testing.T) {
	it := item{chat: telegram.Chat{Title: "Test Chat"}}

	if it.FilterValue() != "Test Chat" {
		t.Errorf("expected FilterValue 'Test Chat', got '%s'", it.FilterValue())
	}
}

func TestItemDelegate_Height(t *testing.T) {
	d := itemDelegate{}
	if d.Height() != 1 {
		t.Errorf("expected Height 1, got %d", d.Height())
	}
}

func TestItemDelegate_Spacing(t *testing.T) {
	d := itemDelegate{}
	if d.Spacing() != 0 {
		t.Errorf("expected Spacing 0, got %d", d.Spacing())
	}
}

func TestItemDelegate_Update(t *testing.T) {
	d := itemDelegate{}
	cmd := d.Update(nil, nil)
	if cmd != nil {
		t.Error("expected nil command from Update")
	}
}

func TestTruncateWithEllipsis(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{name: "no-truncate", in: "short", max: 10, want: "short"},
		{name: "truncate", in: "this is long", max: 8, want: "this ..."},
		{name: "small-max", in: "abcdef", max: 3, want: "abc"},
		{name: "zero", in: "abcdef", max: 0, want: ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := truncateWithEllipsis(tc.in, tc.max); got != tc.want {
				t.Errorf("truncateWithEllipsis(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

func TestModel_Update_CtrlR(t *testing.T) {
	chats := []telegram.Chat{{ID: 1, Title: "Test Chat", UnreadCount: 5}}
	called := false
	markRead := func(c telegram.Chat) error {
		called = true
		if c.ID != 1 {
			t.Errorf("expected chat ID 1, got %d", c.ID)
		}
		return nil
	}

	model := NewModel(chats, markRead, ModelOptions{})

	msg := tea.KeyMsg{Type: tea.KeyCtrlR}
	newModel, _ := model.Update(msg)

	m := newModel.(Model)
	if !called {
		t.Error("expected markReadFunc to be called")
	}
	if m.statusMsg == "" {
		t.Error("expected status message")
	}
}

// TopicModel tests

func TestNewTopicModel(t *testing.T) {
	topics := []telegram.Topic{
		{ID: 1, Title: "General", UnreadCount: 5},
		{ID: 2, Title: "Off-topic", UnreadCount: 10},
	}

	model := NewTopicModel(topics)

	if model.selected != nil {
		t.Error("expected selected to be nil initially")
	}
	if model.quitting {
		t.Error("expected quitting to be false initially")
	}
	if model.list.Title != "Select Topic to Summarize" {
		t.Errorf("unexpected list title: %s", model.list.Title)
	}
}

func TestTopicModel_Init(t *testing.T) {
	model := NewTopicModel([]telegram.Topic{})
	cmd := model.Init()

	if cmd != nil {
		t.Error("expected Init to return nil")
	}
}

func TestTopicModel_Update_Quit(t *testing.T) {
	model := NewTopicModel([]telegram.Topic{{ID: 1, Title: "Test"}})

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	newModel, cmd := model.Update(msg)

	m := newModel.(TopicModel)
	if !m.quitting {
		t.Error("expected quitting to be true after Ctrl+C")
	}
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestTopicModel_Update_Enter(t *testing.T) {
	topics := []telegram.Topic{
		{ID: 1, Title: "General", UnreadCount: 5},
	}
	model := NewTopicModel(topics)

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, cmd := model.Update(msg)

	m := newModel.(TopicModel)
	if m.selected == nil {
		t.Error("expected selected to not be nil after Enter")
	}
	if cmd == nil {
		t.Error("expected quit command after selection")
	}
}

func TestTopicModel_Update_WindowSize(t *testing.T) {
	model := NewTopicModel([]telegram.Topic{{ID: 1, Title: "Test"}})

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newModel, cmd := model.Update(msg)

	m := newModel.(TopicModel)
	if m.quitting {
		t.Error("model should not be quitting after window resize")
	}
	if cmd != nil {
		t.Error("expected no command from window resize")
	}
}

func TestTopicModel_View(t *testing.T) {
	model := NewTopicModel([]telegram.Topic{{ID: 1, Title: "Test"}})
	view := model.View()

	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestTopicModel_GetSelected(t *testing.T) {
	model := NewTopicModel([]telegram.Topic{})

	if model.GetSelected() != nil {
		t.Error("expected nil when nothing selected")
	}

	expectedTopic := &telegram.Topic{ID: 42, Title: "Selected"}
	model.selected = expectedTopic

	if model.GetSelected() != expectedTopic {
		t.Error("GetSelected returned wrong topic")
	}
}

func TestTopicItem_FilterValue(t *testing.T) {
	it := topicItem{topic: telegram.Topic{Title: "Test Topic"}}

	if it.FilterValue() != "Test Topic" {
		t.Errorf("expected FilterValue 'Test Topic', got '%s'", it.FilterValue())
	}
}

func TestTopicItemDelegate_Height(t *testing.T) {
	d := topicItemDelegate{}
	if d.Height() != 1 {
		t.Errorf("expected Height 1, got %d", d.Height())
	}
}

func TestTopicItemDelegate_Spacing(t *testing.T) {
	d := topicItemDelegate{}
	if d.Spacing() != 0 {
		t.Errorf("expected Spacing 0, got %d", d.Spacing())
	}
}

func TestTopicItemDelegate_Update(t *testing.T) {
	d := topicItemDelegate{}
	cmd := d.Update(nil, nil)
	if cmd != nil {
		t.Error("expected nil command from Update")
	}
}
