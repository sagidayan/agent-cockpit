package tui

import (
	"cmp"
	"fmt"
	"image/color"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/modes"
	"github.com/sagidayan/agent-cockpit/internal/paths"
	"github.com/sagidayan/agent-cockpit/internal/projects"
	"github.com/sagidayan/agent-cockpit/internal/state"
)

const (
	cardGap        = 1
	maxSuggestions = 5
	minMetaWidth   = 24
	tldrLimit      = 90
)

var (
	listMarker = regexp.MustCompile(`^([-*+>]|\d+[.)])\s+`)
	emphasis   = strings.NewReplacer("**", "", "__", "", "`", "")
)

var (
	accent  = lipgloss.Color("#8B7CF6")
	muted   = lipgloss.Color("#7A808C")
	subtle  = lipgloss.Color("#4B5060")
	danger  = lipgloss.Color("#EF6B73")
	success = lipgloss.Color("#4FBF85")
	warning = lipgloss.Color("#E5B341")

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(accent)
	nameStyle     = lipgloss.NewStyle().Bold(true)
	mutedStyle    = lipgloss.NewStyle().Foreground(muted)
	subtleStyle   = lipgloss.NewStyle().Foreground(subtle)
	barStyle      = lipgloss.NewStyle().Foreground(accent)
	errStyle      = lipgloss.NewStyle().Foreground(danger)
	noticeStyle   = lipgloss.NewStyle().Foreground(success)
	keyStyle      = lipgloss.NewStyle().Foreground(accent).Bold(true)
	favoriteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F2C94C"))
	selectedBg    = lipgloss.Color("#2E2A45")
	blurredBg     = lipgloss.Color("#25262E")
	selectedChip  = lipgloss.NewStyle().Foreground(lipgloss.Color("#1E1E2E")).Background(accent).Bold(true).Padding(0, 1)
	boxStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(0, 1)
)

type badge struct {
	label string
	color lipgloss.Style
}

var (
	attention     = lipgloss.Color("#E06C9F")
	attentionDim  = lipgloss.Color("#8A4A66")
	attentionLoud = lipgloss.NewStyle().Foreground(attention).Bold(true)
	attentionSoft = lipgloss.NewStyle().Foreground(attentionDim)
)

var badges = map[state.Status]badge{
	state.Working:    {"working", lipgloss.NewStyle().Foreground(warning)},
	state.Background: {"background", lipgloss.NewStyle().Foreground(lipgloss.Color("#6CB6FF"))},
	state.NeedsInput: {"needs input", attentionLoud},
	state.Done:       {"done", lipgloss.NewStyle().Foreground(success)},
	state.Idle:       {"idle", mutedStyle},
}

func badgeFor(s state.Status) badge {
	if b, ok := badges[s]; ok {
		return b
	}
	return badge{"unknown", subtleStyle}
}

func (m model) statusIcon(s state.Status) string {
	switch s {
	case state.Working:
		return spinner[m.frame%len(spinner)]
	case state.Background:
		return backgroundSpinner[(m.frame/3)%len(backgroundSpinner)]
	}
	if i, ok := icons.status[s]; ok {
		return i
	}
	return "·"
}

func (m model) statusStyle(s state.Status) lipgloss.Style {
	if s == state.NeedsInput && (m.frame/5)%2 == 1 {
		return attentionSoft
	}
	return badgeFor(s).color
}

func (m model) statusBadge(s state.Status) string {
	return m.statusStyle(s).Render(m.statusIcon(s) + " " + badgeFor(s).label)
}

func (m model) View() tea.View {
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, m.header(), m.listView(m.listHeight()), m.footer()))
	v.AltScreen = true
	v.ReportFocus = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m model) listHeight() int {
	return m.height - lipgloss.Height(m.header()) - lipgloss.Height(m.footer())
}

func (m model) cards() []string {
	cards := make([]string, len(m.agents))
	for i, a := range m.agents {
		cards[i] = m.card(a, i == m.cursor)
	}
	return cards
}

func (m model) header() string {
	chips, totals := m.statusChips(!m.blurred), m.totals()
	if lipgloss.Width(chips)+lipgloss.Width(totals)+4 > m.width {
		chips = m.statusChips(false)
	}
	paint := func(s string) string { return gradient(s, true, titleColors...) }
	rule := gradient(strings.Repeat("━", max(m.width, 0)), false, titleColors...)
	clock := withIcon(icons.clock, time.Now().Format("Mon 2 Jan · 15:04"))
	if m.blurred {
		paint = func(s string) string { return mutedStyle.Render(s) }
		rule = subtleStyle.Render(strings.Repeat("─", max(m.width, 0)))
		clock = "viewer has focus"
	}
	right := []string{m.population(), mutedStyle.Render(clock), ""}
	rows := banner(bigTitle)
	if lipgloss.Width(rows[0])+lipgloss.Width(right[0])+6 > m.width {
		rows, right = []string{withIcon(icons.title, spaced(bigTitle))}, right[:1]
	}
	lines := make([]string, 0, len(rows)+2)
	for i, row := range rows {
		lines = append(lines, spread(paint(row), right[i], m.width))
	}
	return strings.Join(append(lines, spread(chips, totals, m.width), rule), "\n")
}

const bigTitle = "AGENT COCKPIT"

var bigFont = map[rune][3]string{
	'A': {"╔═╗", "╠═╣", "╩ ╩"},
	'G': {"╔═╗", "║ ╦", "╚═╝"},
	'E': {"╔═╗", "║╣ ", "╚═╝"},
	'N': {"╔╗╔", "║║║", "╝╚╝"},
	'T': {"╔╦╗", " ║ ", " ╩ "},
	'C': {"╔═╗", "║  ", "╚═╝"},
	'O': {"╔═╗", "║ ║", "╚═╝"},
	'K': {"╦╔═", "╠╩╗", "╩ ╩"},
	'P': {"╔═╗", "╠═╝", "╩  "},
	'I': {"╦", "║", "╩"},
	' ': {"   ", "   ", "   "},
}

func banner(s string) []string {
	rows := make([]string, 3)
	for _, r := range s {
		for i, part := range bigFont[r] {
			rows[i] += part
		}
	}
	return rows
}

var titleColors = []color.Color{accent, attention, lipgloss.Color("#6CB6FF")}

func spaced(s string) string {
	return strings.Join(strings.Split(strings.ToUpper(s), ""), " ")
}

func gradient(s string, bold bool, stops ...color.Color) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return ""
	}
	colors := lipgloss.Blend1D(len(runes), stops...)
	var b strings.Builder
	for i, r := range runes {
		b.WriteString(lipgloss.NewStyle().Foreground(colors[i]).Bold(bold).Render(string(r)))
	}
	return b.String()
}

func spread(left, right string, width int) string {
	gap := width - 2 - lipgloss.Width(left) - lipgloss.Width(right)
	if right == "" || gap < 2 {
		return " " + left
	}
	return " " + left + strings.Repeat(" ", gap) + right
}

func (m model) statusChips(labels bool) string {
	counts := map[state.Status]int{}
	for _, a := range m.all {
		counts[a.Status]++
	}
	var parts []string
	for _, s := range []state.Status{state.NeedsInput, state.Working, state.Background, state.Done, state.Idle} {
		if counts[s] == 0 {
			continue
		}
		chip := fmt.Sprintf("%s %d", m.statusIcon(s), counts[s])
		if labels {
			chip += " " + badgeFor(s).label
		}
		parts = append(parts, m.statusStyle(s).Render(chip))
	}
	if len(parts) == 0 {
		return mutedStyle.Render("no agents yet")
	}
	return strings.Join(parts, "  ")
}

func (m model) population() string {
	masters := 0
	for _, a := range m.all {
		if a.Mode == modes.Master {
			masters++
		}
	}
	parts := []string{plural(len(m.all), "agent")}
	if masters > 0 {
		parts = append(parts, plural(masters, "master"))
	}
	return mutedStyle.Render(strings.Join(parts, " · "))
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func (m model) totals() string {
	var parts []string
	if cost := totalCost(m.all); cost > 0 {
		parts = append(parts, mutedStyle.Render(withIcon(icons.cost, fmt.Sprintf("$%.2f", cost))))
	}
	added, removed := 0, 0
	for _, a := range m.all {
		added, removed = added+a.Stats.LinesAdded, removed+a.Stats.LinesRemoved
	}
	if added+removed > 0 {
		parts = append(parts, badges[state.Done].color.Render(fmt.Sprintf("+%d", added))+" "+errStyle.Render(fmt.Sprintf("-%d", removed)))
	}
	if m.sortBy != "" && m.sortBy != "location" {
		parts = append(parts, mutedStyle.Render("by "+m.sortBy))
	}
	return strings.Join(parts, mutedStyle.Render(" · "))
}

func (m model) listView(height int) string {
	box := lipgloss.NewStyle().Height(height).MaxHeight(max(height, 0))
	switch {
	case m.overlay == "help":
		return box.Render(m.helpView())
	case m.overlay == "projects":
		return box.Render(m.projectsView())
	case m.overlay == "favorites":
		return box.Render(m.favoritesView())
	case m.detail.open:
		return box.Render(m.detailView())
	case len(m.agents) == 0 && m.filter != "":
		return box.Render(mutedStyle.Render(" No agents match \"" + m.filter + "\"."))
	case len(m.agents) == 0:
		return box.Render(mutedStyle.Render(" No agents running.\n Press n to start one."))
	}
	cards := m.cards()
	lines := strings.Split(strings.Join(cards, strings.Repeat("\n", cardGap+1)), "\n")
	start := m.scrollStart(cards, height)
	lines = lines[start:min(len(lines), start+max(height, 0))]
	return lipgloss.NewStyle().Height(height).Render(strings.Join(lines, "\n"))
}

func (m model) scrollStart(cards []string, height int) int {
	end := 0
	for i := 0; i <= m.cursor && i < len(cards); i++ {
		end += lipgloss.Height(cards[i]) + cardGap
	}
	return max(0, end-cardGap-height)
}

func (m model) card(a agents.Live, selected bool) string {
	depth := m.depth[a.ID]
	inner := max(m.width-4-2*depth, 10)
	lines := append(m.cardHead(a, inner),
		ansi.Truncate(keyStyle.Render("❯")+" "+orPlaceholder(tldr(a.Prompt), "no prompt yet"), inner, "…"),
		ansi.Truncate(m.reply(a), inner, "…"),
	)
	bar := subtleStyle.Render("│")
	_, flashing := m.changed[a.ID]
	switch {
	case flashing && (m.frame/3)%2 == 0:
		bar = m.statusStyle(a.Status).Render("┃")
	case selected && m.blurred:
		bar = mutedStyle.Render("┃")
	case selected:
		bar = barStyle.Render("┃")
	}
	indent := strings.Repeat("  ", depth)
	for i, l := range lines {
		lines[i] = " " + indent + bar + " " + l
		if selected {
			lines[i] = m.highlight(lines[i])
		}
	}
	return strings.Join(lines, "\n")
}

func (m model) cardHead(a agents.Live, inner int) []string {
	status := m.statusBadge(a.Status)
	label := withIcon(icons.kinds[a.Kind], name(a))
	if m.depth[a.ID] > 0 {
		label = "↳ " + label
	}
	if n := m.childCount(a); n > 0 {
		label = m.chevron(a) + " " + label + " " + keyStyle.Render(fmt.Sprintf("[%d]", n))
	}
	if m.isFavorite(a) {
		label += " " + favoriteStyle.Render(icons.star)
	}
	title := nameStyle.Render(ansi.Truncate(label, inner-lipgloss.Width(status)-1, "…"))
	gap := max(1, inner-lipgloss.Width(title)-lipgloss.Width(status))
	return []string{
		title + strings.Repeat(" ", gap) + status,
		metaLine(m.meta(a), statsLine(a), inner),
	}
}

func (m model) chevron(a agents.Live) string {
	if m.isExpanded(a.ID) {
		return "▾"
	}
	return "▸"
}

func (m model) highlight(line string) string {
	bg := selectedBg
	if m.blurred {
		bg = blurredBg
	}
	on := ansi.NewStyle().BackgroundColor(bg).String()
	line = strings.ReplaceAll(line, ansi.ResetStyle, ansi.ResetStyle+on)
	pad := strings.Repeat(" ", max(0, m.width-ansi.StringWidth(line)))
	return on + line + pad + ansi.ResetStyle
}

func metaLine(meta, stats string, width int) string {
	room := width - lipgloss.Width(stats) - 2
	if stats == "" || room < minMetaWidth {
		return mutedStyle.Render(ansi.Truncate(meta, width, "…"))
	}
	meta = ansi.Truncate(meta, room, "…")
	return mutedStyle.Render(meta) + strings.Repeat(" ", width-lipgloss.Width(meta)-lipgloss.Width(stats)) + stats
}

func (m model) reply(a agents.Live) string {
	marker := m.statusStyle(a.Status).Render("⏺") + " "
	text := tldr(a.Message)
	switch {
	case a.Status == state.NeedsInput && text != "":
		return marker + m.statusStyle(a.Status).Render(text)
	case a.Status == state.Working && text == "":
		return marker + subtleStyle.Render("thinking…")
	}
	return marker + orPlaceholder(text, "no reply yet")
}

func orPlaceholder(text, placeholder string) string {
	if text == "" {
		return subtleStyle.Render(placeholder)
	}
	return text
}

func tldr(s string) string {
	fenced := false
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
			continue
		}
		if fenced || line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "|") || strings.HasSuffix(line, ":") {
			continue
		}
		return clip(firstSentence(emphasis.Replace(listMarker.ReplaceAllString(line, ""))), tldrLimit)
	}
	return ""
}

func firstSentence(s string) string {
	for i := 0; i+1 < len(s); i++ {
		if strings.ContainsRune(".?!", rune(s[i])) && s[i+1] == ' ' {
			return s[:i+1]
		}
	}
	return s
}

func clip(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	cut := string(r[:limit])
	if i := strings.LastIndex(cut, " "); i > limit/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, ",.;: ") + "…"
}

func (m model) footer() string {
	var parts []string
	if m.loadErr != nil {
		parts = append(parts, m.wrap(errStyle, "✗ "+m.loadErr.Error()))
	}
	if m.actionErr != nil {
		parts = append(parts, m.wrap(errStyle, "✗ "+m.actionErr.Error()))
	}
	if m.notice != "" {
		style := noticeStyle
		if time.Since(m.noticeAt) > noticeFor-noticeFade {
			style = subtleStyle
		}
		parts = append(parts, " "+style.Render(m.notice))
	}
	if m.sending {
		parts = append(parts, m.composerView())
	}
	if m.spawning {
		parts = append(parts, m.spawnView())
	}
	if n := len(m.restorable); n > 0 {
		banner := fmt.Sprintf("%d session(s) from last time", n)
		parts = append(parts, " "+keyStyle.Render(withIcon(icons.restore, banner))+"  "+keyStyle.Render("R")+mutedStyle.Render(" restore  ")+keyStyle.Render("x")+mutedStyle.Render(" dismiss"))
	}
	if m.filter != "" && m.prompt != '/' {
		parts = append(parts, " "+keyStyle.Render(withIcon(icons.search, m.filter))+mutedStyle.Render(fmt.Sprintf("  %d of %d", len(m.agents), len(m.all))))
	}
	switch {
	case m.confirm != nil:
		parts = append(parts, " "+errStyle.Render(m.confirm.prompt))
	case m.prompt != 0:
		parts = append(parts, m.promptView())
	default:
		parts = append(parts, m.help())
	}
	return strings.Join(parts, "\n")
}

func (m model) promptView() string {
	var rows []string
	if m.prompt == ':' {
		for _, c := range completions(m.line.Value()) {
			rows = append(rows, "   "+keyStyle.Render(c.name)+" "+mutedStyle.Render(c.args))
		}
	}
	prefix := string(m.prompt)
	if m.prompt == 'r' {
		prefix = "label: "
	}
	rows = append(rows, " "+keyStyle.Render(prefix)+m.line.View())
	return strings.Join(rows, "\n")
}

func (m model) projectsView() string {
	rows := []string{" " + titleStyle.Render("projects") + mutedStyle.Render("  "+paths.Contract(projects.Path()))}
	if len(m.projects) == 0 {
		rows = append(rows, "", mutedStyle.Render("   No projects yet."), mutedStyle.Render("   :project add <name> [dir]"))
	}
	for _, p := range m.projects {
		agent := ""
		if p.Agent != "" {
			agent = subtleStyle.Render(" · " + p.Agent)
		}
		rows = append(rows, ansi.Truncate("   "+keyStyle.Render(p.Name)+"  "+mutedStyle.Render(p.Dir)+agent, m.width, "…"))
	}
	return strings.Join(rows, "\n")
}

func (m model) helpView() string {
	keys := [][2]string{
		{"j/k ↑↓", "move"}, {"pgup/dn", "move by 5"}, {"home/end", "first / last (also G)"}, {"1-9", "jump to agent"},
		{"⏎ → l", "interact in viewer"}, {"i", "details: full prompt, reply, conversation (esc closes)"},
		{"g", "go to agent's window"}, {"d", "review the agent's diff in your editor"}, {"m", "move agent to the cockpit session"}, {"space", "expand / collapse children"}, {"s", "send a prompt"}, {"r", "label the agent"}, {"n", "new session"},
		{"f", "toggle favorite"}, {"F", "favorites (resume old ones)"},
		{"/", "filter"}, {"esc", "clear filter"}, {":", "command"}, {"R", "restore last sessions"},
		{"x", "dismiss restore"}, {"?", "this help"}, {"q", "quit"},
	}
	rows := []string{" " + titleStyle.Render("keys")}
	for _, k := range keys {
		rows = append(rows, fmt.Sprintf("   %s %s", keyStyle.Render(fmt.Sprintf("%-8s", k[0])), mutedStyle.Render(k[1])))
	}
	rows = append(rows, "", " "+titleStyle.Render("commands")+mutedStyle.Render("  any unique prefix works"))
	for _, c := range commands() {
		rows = append(rows, "   "+keyStyle.Render(fmt.Sprintf("%-11s", ":"+c.name))+mutedStyle.Render(c.help))
	}
	rows = append(rows, "   "+keyStyle.Render(fmt.Sprintf("%-11s", ":!<cmd>"))+mutedStyle.Render("run a command in the agent's directory"))
	for i, r := range rows {
		rows[i] = ansi.Truncate(r, m.width, "…")
	}
	return strings.Join(rows, "\n")
}

func (m model) composerView() string {
	a, _ := m.selected()
	title := keyStyle.Render("→ " + name(a))
	return boxStyle.Width(max(m.width, 10)).Render(title + "\n" + m.input.View())
}

func (m model) spawnView() string {
	f := m.form
	steps := f.steps()
	i := slices.Index(steps, f.step)
	crumbs := []string{keyStyle.Render(withIcon(icons.plus, "new session"))}
	for _, done := range steps[:i] {
		crumbs = append(crumbs, mutedStyle.Render(f.crumb(done)))
	}
	head := spread(strings.Join(crumbs, subtleStyle.Render(" › ")), subtleStyle.Render(fmt.Sprintf("step %d of %d", i+1, len(steps))), m.width-2)
	rows := []string{strings.TrimPrefix(head, " "), "", nameStyle.Render(f.question())}
	rows = append(rows, m.stepBody()...)
	if f.err != "" {
		rows = append(rows, errStyle.Render(f.err))
	}
	return boxStyle.Width(max(m.width, 10)).Render(strings.Join(rows, "\n"))
}

func (f spawnForm) crumb(step spawnStep) string {
	switch step {
	case stepMode:
		md := f.currentMode()
		return withIcon(icons.modes[md.Name], md.Title)
	case stepKind:
		return withIcon(icons.kinds[f.kinds[f.kind]], f.kinds[f.kind])
	case stepProject:
		return cmp.Or(f.project, "other folder")
	case stepDir:
		return paths.Contract(filepath.Clean(paths.Expand(f.dir.Value())))
	case stepURL:
		return modes.ReviewName(strings.TrimSpace(f.url.Value()))
	case stepWorktree:
		if f.worktree {
			return "new worktree"
		}
		return "this checkout"
	}
	return ""
}

func (f spawnForm) question() string {
	switch f.step {
	case stepMode:
		return "What kind of session?"
	case stepKind:
		return "Which agent?"
	case stepProject:
		return "Which project?"
	case stepDir:
		return "Which folder?"
	case stepURL:
		return "Which pull request?"
	case stepWorktree:
		return "Work in a separate git worktree?"
	}
	return taskQuestions[f.currentMode().Input]
}

var taskQuestions = map[string]string{
	modes.InputDir:  "What should it work on?",
	modes.InputURL:  "Anything to focus on?",
	modes.InputNone: "What's the task?",
	modes.InputTemp: "What do you want to ask?",
}

var modeDescriptions = map[string]string{
	modes.Code:   "work in a project folder",
	modes.Review: "review a pull request",
	modes.Master: "coordinate agents across projects",
	modes.Chat:   "ask questions in a disposable folder",
}

var stepKeys = map[spawnStep][][2]string{
	stepMode:     {{"↑↓", "choose"}, {"⏎", "next"}, {"esc", "cancel"}},
	stepKind:     {{"↑↓", "choose"}, {"⏎", "next"}, {"esc", "back"}},
	stepProject:  {{"type", "filter"}, {"↑↓", "choose"}, {"⏎", "next"}, {"esc", "back"}},
	stepDir:      {{"type", "filter"}, {"↑↓", "choose"}, {"→", "open"}, {"←", "up"}, {"⏎", "next"}, {"esc", "back"}},
	stepURL:      {{"⏎", "next"}, {"esc", "back"}},
	stepWorktree: {{"↑↓", "choose"}, {"⏎", "next"}, {"esc", "back"}},
	stepDetails:  {{"tab", "task / name"}, {"⏎", "start"}, {"esc", "back"}},
}

func (m model) stepBody() []string {
	f := m.form
	switch f.step {
	case stepMode:
		var items []suggestion
		for _, md := range f.modes {
			items = append(items, suggestion{withIcon(icons.modes[md.Name], md.Title), cmp.Or(modeDescriptions[md.Name], "custom mode"), ""})
		}
		return m.choices(items, f.mode)
	case stepKind:
		var items []suggestion
		for _, k := range f.kinds {
			items = append(items, suggestion{withIcon(icons.kinds[k], k), "", ""})
		}
		return m.choices(items, f.kind)
	case stepProject:
		return append([]string{m.formRow("filter", f.filter.View(), true)}, m.suggestionRows()...)
	case stepDir:
		return append([]string{m.formRow("dir", f.dir.View(), true)}, m.suggestionRows()...)
	case stepURL:
		return []string{m.formRow("PR url", f.url.View(), true)}
	case stepWorktree:
		items := []suggestion{
			{"no", "work in this checkout", ""},
			{"yes", "new cockpit/<name> branch from HEAD, in a sibling <repo>-worktrees folder", ""},
		}
		if f.worktree {
			return m.choices(items, 1)
		}
		return m.choices(items, 0)
	}
	label := map[string]string{modes.InputURL: "notes", modes.InputTemp: "question"}[f.currentMode().Input]
	return []string{
		m.formRow(cmp.Or(label, "task"), f.task.View(), !f.onName),
		m.formRow("name", f.name.View(), f.onName),
	}
}

func (m model) choices(items []suggestion, current int) []string {
	width := 0
	for _, it := range items {
		width = max(width, lipgloss.Width(it.label))
	}
	var rows []string
	for i, it := range items {
		label := it.label + strings.Repeat(" ", width-lipgloss.Width(it.label))
		row := "  " + mutedStyle.Render(label)
		if i == current {
			row = keyStyle.Render("› " + label)
		}
		if it.hint != "" {
			row += "   " + subtleStyle.Render(it.hint)
		}
		rows = append(rows, ansi.Truncate(row, m.width-4, "…"))
	}
	return rows
}

func (m model) suggestionRows() []string {
	matches, current := m.form.matches, m.form.pick
	start := min(max(0, current-maxSuggestions+1), max(0, len(matches)-maxSuggestions))
	var rows []string
	for i := start; i < min(len(matches), start+maxSuggestions); i++ {
		label, hint := matches[i].label, ""
		if matches[i].hint != "" {
			hint = "  " + subtleStyle.Render(matches[i].hint)
		}
		row := "            " + mutedStyle.Render(label)
		if i == current {
			row = "          " + keyStyle.Render("› "+label)
		}
		rows = append(rows, ansi.Truncate(row+hint, m.width-4, "…"))
	}
	if more := len(matches) - maxSuggestions; more > 0 {
		rows = append(rows, "            "+subtleStyle.Render(fmt.Sprintf("%d of %d", current+1, len(matches))))
	}
	return rows
}

func (m model) formRow(label, value string, focused bool) string {
	if focused {
		return keyStyle.Render(fmt.Sprintf("▸ %-8s", label)) + value
	}
	return mutedStyle.Render(fmt.Sprintf("  %-8s", label)) + value
}

func statsLine(a agents.Live) string {
	if !a.HasStats {
		return ""
	}
	s := a.Stats
	var parts []string
	if s.ContextMax > 0 {
		pct := min(100, s.ContextUsed*100/s.ContextMax)
		parts = append(parts, gauge(pct, 10)+" "+mutedStyle.Render(fmt.Sprintf("%d%%", pct)))
	}
	if s.CostUSD > 0 {
		parts = append(parts, mutedStyle.Render(withIcon(icons.cost, fmt.Sprintf("%.2f", s.CostUSD))))
	}
	if s.LinesAdded+s.LinesRemoved > 0 {
		diff := badges[state.Done].color.Render(fmt.Sprintf("+%d", s.LinesAdded)) + " " + errStyle.Render(fmt.Sprintf("-%d", s.LinesRemoved))
		if icons.diff != "" {
			diff = mutedStyle.Render(icons.diff) + " " + diff
		}
		parts = append(parts, diff)
	}
	if s.Model != "" {
		parts = append(parts, mutedStyle.Render(withIcon(icons.model, s.Model)))
	}
	return strings.Join(parts, mutedStyle.Render(" · "))
}

func gauge(pct, width int) string {
	filled := (pct*width + 50) / 100
	color := success
	switch {
	case pct >= 85:
		color = danger
	case pct >= 60:
		color = warning
	}
	return lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("▰", filled)) + subtleStyle.Render(strings.Repeat("▱", width-filled))
}

func totalCost(live []agents.Live) float64 {
	total := 0.0
	for _, a := range live {
		total += a.Stats.CostUSD
	}
	return total
}

func (m model) help() string {
	keys := [][2]string{{"↑↓", "move"}, {"⏎/→", "interact"}, {"i", "details"}, {"g", "go-to"}, {"d", "diff"}, {"s", "send"}, {"n", "new"}, {"f", "fav"}, {"F", "favorites"}, {"/", "filter"}, {":", "cmd"}, {"?", "help"}}
	switch {
	case m.sending:
		keys = [][2]string{{"⏎", "send"}, {"alt+⏎", "newline"}, {"esc", "cancel"}}
	case m.spawning:
		keys = stepKeys[m.form.step]
	case m.detail.open:
		keys = [][2]string{{"↑↓", "scroll"}, {"pgup/dn", "page"}, {"home/end", "top / bottom"}, {"⏎/→", "interact"}, {"s", "send"}, {"esc", "close"}}
	}
	var lines []string
	line := ""
	for _, k := range keys {
		item := keyStyle.Render(k[0]) + " " + mutedStyle.Render(k[1])
		if line != "" && lipgloss.Width(line+"  "+item) > m.width-2 {
			lines, line = append(lines, line), ""
		}
		if line != "" {
			line += "  "
		}
		line += item
	}
	return " " + strings.Join(append(lines, line), "\n ")
}

func (m model) wrap(style lipgloss.Style, s string) string {
	return style.Width(max(m.width, 10)).Padding(0, 1).Render(s)
}

func name(a agents.Live) string {
	if a.Label != "" {
		return a.Label
	}
	if a.WindowName != "" {
		return a.WindowName
	}
	return a.Session + ":" + a.Window
}

func (m model) childCount(a agents.Live) int {
	n := 0
	for _, c := range m.all {
		if c.Parent == a.ID && c.ID != a.ID {
			n++
		}
	}
	return n
}

func (m model) meta(a agents.Live) string {
	place := withIcon(icons.folder, filepath.Base(a.Pane.Cwd))
	if p := m.projectName(a); p != "" {
		place = withIcon(icons.project, p)
	}
	where := a.Session + ":" + a.Window
	if a.Label != "" {
		where += " " + a.WindowName
	}
	parts := []string{a.Kind}
	if a.Mode != "" && a.Mode != modes.Code {
		parts = append(parts, withIcon(icons.modes[a.Mode], a.Mode))
	}
	parts = append(parts, place, where)
	if s := since(a.UpdatedAt); s != "" {
		parts = append(parts, withIcon(icons.clock, s))
	}
	return strings.Join(parts, " · ")
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func since(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return age(time.Since(t))
}

func age(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
