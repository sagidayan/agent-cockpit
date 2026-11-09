package tui

import "github.com/sagidayan/agent-cockpit/internal/state"

type iconSet struct {
	title, project, folder, cost, model, diff, clock, label, search, restore, plus, star string
	kinds, modes                                                                         map[string]string
	status                                                                               map[state.Status]string
}

var nerdIcons = iconSet{
	title:   "\U000F08CF",
	project: "\uF502",
	folder:  "\U000F024B",
	cost:    "\U000F01C1",
	model:   "\U000F061A",
	diff:    "\uF440",
	clock:   "\U000F0150",
	label:   "\U000F04F9",
	search:  "\U000F0349",
	restore: "\U000F006F",
	plus:    "\uF067",
	star:    "\U000F04CE",
	kinds:   map[string]string{"claude": "\uEC10", "codex": "\U000F09D1", "opencode": "\uEA85"},
	modes:   map[string]string{"code": "\uF44F", "review": "\uF407", "bug": "\uF188", "chat": "\uF086"},
	status: map[state.Status]string{
		state.NeedsInput: "\U000F009E",
		state.Done:       "\U000F05E0",
		state.Idle:       "\U000F04B2",
		state.Paused:     "\U000F03E4",
	},
}

var plainIcons = iconSet{
	title:   "◇",
	project: "◆",
	restore: "↺",
	plus:    "+",
	star:    "★",
	search:  "/",
	kinds:   map[string]string{},
	modes:   map[string]string{},
	status: map[state.Status]string{
		state.NeedsInput: "◆",
		state.Done:       "✓",
		state.Idle:       "○",
		state.Paused:     "‖",
	},
}

var icons = nerdIcons

var backgroundSpinner = []string{"◴", "◷", "◶", "◵"}

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func withIcon(icon, text string) string {
	if icon == "" {
		return text
	}
	return icon + " " + text
}
