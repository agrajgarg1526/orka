package tui

import "github.com/charmbracelet/bubbles/key"

type BoardKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Advance key.Binding
	Retreat key.Binding
	New     key.Binding
	Open    key.Binding
	Search  key.Binding
	Help    key.Binding
	Quit    key.Binding
	Back    key.Binding
}

var BoardKeys = BoardKeyMap{
	Up:      key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
	Down:    key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
	Left:    key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h/←", "prev col")),
	Right:   key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/→", "next col")),
	Advance: key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "advance phase")),
	Retreat: key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "retreat phase")),
	New:     key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new task")),
	Open:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
	Search:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "projects")),
}

type TaskKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Advance key.Binding
	Retreat key.Binding
	Restart key.Binding
	Stop    key.Binding
	Edit    key.Binding
	Back    key.Binding
}

var TaskKeys = TaskKeyMap{
	Up:      key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "scroll up")),
	Down:    key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "scroll down")),
	Advance: key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "advance phase")),
	Retreat: key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "retreat phase")),
	Restart: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart agent")),
	Stop:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "stop agent")),
	Edit:    key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit notes")),
	Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
}
