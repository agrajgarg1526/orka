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
	Delete  key.Binding
	Search  key.Binding
	Help    key.Binding
	Quit    key.Binding
	Back    key.Binding
}

var BoardKeys = BoardKeyMap{
	Up:      key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
	Down:    key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
	Left:    key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "prev col")),
	Right:   key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "next col")),
	Advance: key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "advance phase")),
	Retreat: key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "retreat phase")),
	New:     key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new task")),
	Open:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
	Delete:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete task")),
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
	Diff    key.Binding
	Back    key.Binding
}

var TaskKeys = TaskKeyMap{
	Up:      key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "scroll up")),
	Down:    key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "scroll down")),
	Advance: key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "advance phase")),
	Retreat: key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "retreat phase")),
	Restart: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "launch/resume")),
	Stop:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "stop agent")),
	Edit:    key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit notes")),
	Diff:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "diff viewer")),
	Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
}
