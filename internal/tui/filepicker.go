package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type FilePickedMsg struct{ Path string }

type filePicker struct {
	dir     string
	entries []string // directory names only
	idx     int
	scroll  int
	width   int
	height  int
	err     string
}

func newFilePicker(startDir string, w, h int) filePicker {
	fp := filePicker{width: w, height: h}
	fp.cd(startDir)
	return fp
}

func (fp *filePicker) cd(dir string) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		fp.err = err.Error()
		return
	}
	infos, err := os.ReadDir(abs)
	if err != nil {
		fp.err = err.Error()
		return
	}
	fp.err = ""
	fp.dir = abs
	fp.entries = []string{".."}
	var dirs []string
	for _, e := range infos {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	fp.entries = append(fp.entries, dirs...)
	fp.idx = 0
	fp.scroll = 0
}

func (fp filePicker) visibleH() int {
	h := fp.height - 8
	if h < 4 {
		h = 4
	}
	return h
}

func (fp filePicker) Update(msg tea.Msg) (filePicker, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "up", "k":
			if fp.idx > 0 {
				fp.idx--
				fp.clampScroll()
			}
		case "down", "j":
			if fp.idx < len(fp.entries)-1 {
				fp.idx++
				fp.clampScroll()
			}
		case "right", "l":
			if fp.idx < len(fp.entries) {
				name := fp.entries[fp.idx]
				var next string
				if name == ".." {
					next = filepath.Dir(fp.dir)
				} else {
					next = filepath.Join(fp.dir, name)
				}
				fp.cd(next)
			}
		case "enter":
			if fp.idx < len(fp.entries) {
				name := fp.entries[fp.idx]
				var selected string
				if name == ".." {
					selected = filepath.Dir(fp.dir)
				} else {
					selected = filepath.Join(fp.dir, name)
				}
				return fp, func() tea.Msg { return FilePickedMsg{Path: selected} }
			}
			return fp, func() tea.Msg { return FilePickedMsg{Path: fp.dir} }
		}
	}
	return fp, nil
}

func (fp *filePicker) clampScroll() {
	vh := fp.visibleH()
	if fp.idx < fp.scroll {
		fp.scroll = fp.idx
	}
	if fp.idx >= fp.scroll+vh {
		fp.scroll = fp.idx - vh + 1
	}
}

func (fp filePicker) View(cardW int) string {
	innerW := cardW - 6

	dirLine := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Width(innerW).Render(fp.dir)

	vh := fp.visibleH()
	var rows []string
	for i := fp.scroll; i < len(fp.entries) && i < fp.scroll+vh; i++ {
		name := fp.entries[i]
		if i == fp.idx {
			rows = append(rows, lipgloss.NewStyle().
				Foreground(lipgloss.Color("#111827")).
				Background(colorPrimary).
				Bold(true).
				Width(innerW).
				Render("  "+name+"/"))
		} else {
			rows = append(rows, lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E5E7EB")).
				Width(innerW).
				Render("  "+name+"/"))
		}
	}

	var hintOrErr string
	if fp.err != "" {
		hintOrErr = StyleStatusError.Render(fp.err)
	} else {
		hintOrErr = StyleHelp.Render("↑/↓ navigate   → open dir   enter select   esc back")
	}

	selectPath := fp.dir
	if fp.idx < len(fp.entries) {
		name := fp.entries[fp.idx]
		if name == ".." {
			selectPath = filepath.Dir(fp.dir)
		} else {
			selectPath = filepath.Join(fp.dir, name)
		}
	}
	selectLine := lipgloss.NewStyle().
		Foreground(colorSuccess).Bold(true).
		Render("→ " + selectPath)

	return lipgloss.JoinVertical(lipgloss.Left,
		hintOrErr,
		"",
		selectLine,
		StyleStatusMuted.Render(strings.Repeat("─", innerW)),
		"",
		dirLine,
		strings.Join(rows, "\n"),
	)
}
