package tui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"
)

func newComposer() textarea.Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.Placeholder = "Message the agent…"
	ta.ShowLineNumbers = false
	ta.DynamicHeight = true
	ta.MinHeight = 2
	ta.MaxHeight = 10
	ta.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter", "ctrl+j"))
	s := textarea.DefaultDarkStyles()
	for _, st := range []*textarea.StyleState{&s.Focused, &s.Blurred} {
		st.CursorLine = lipgloss.NewStyle()
		st.Placeholder = mutedStyle
		st.EndOfBuffer = lipgloss.NewStyle()
	}
	s.Cursor.Color = accent
	ta.SetStyles(s)
	return ta
}
