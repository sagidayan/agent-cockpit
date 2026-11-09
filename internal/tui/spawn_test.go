package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/modes"
	"github.com/sagidayan/agent-cockpit/internal/projects"
)

func TestMatchDirs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, d := range []string{"api", "app", ".hidden"} {
		require.NoError(t, os.Mkdir(filepath.Join(root, d), 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "file"), nil, 0o644))
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"children of dir", root + "/", []string{root + "/api/", root + "/app/"}},
		{"filters by prefix", root + "/api", []string{root + "/api/"}},
		{"case insensitive", root + "/AP", []string{root + "/api/", root + "/app/"}},
		{"hidden when asked", root + "/.h", []string{root + "/.hidden/"}},
		{"missing dir", root + "/nope/x", nil},
		{"no slash", "api", nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, matchDirs(tc.input))
		})
	}
}

func TestGauge(t *testing.T) {
	t.Parallel()
	cases := []struct {
		pct  int
		want string
	}{
		{0, "▱▱▱▱▱▱▱▱▱▱"},
		{42, "▰▰▰▰▱▱▱▱▱▱"},
		{100, "▰▰▰▰▰▰▰▰▰▰"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, ansi.Strip(gauge(tc.pct, 10)))
	}
}

func TestMatchProjects(t *testing.T) {
	t.Parallel()
	list := []projects.Project{{Name: "payments", Dir: "~/src/pay"}, {Name: "platform", Dir: "~/src/plat"}, {Name: "web", Dir: "~/src/web"}}
	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"empty lists all", "", []string{"payments", "platform", "web"}},
		{"substring", "form", []string{"platform"}},
		{"case insensitive", "PA", []string{"payments"}},
		{"no match", "zzz", nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, s := range matchProjects(list, tc.query) {
				got = append(got, s.value)
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func testForm(kinds []string, list []projects.Project, dir string) spawnForm {
	f := spawnForm{
		modes: modes.All(nil), kinds: kinds, projects: list,
		filter: newField(80), dir: newField(80), url: newField(80), ticket: newField(80), task: newField(80), name: newField(80),
	}
	f.dir.SetValue(dir)
	return f.applyMode()
}

func TestSpawnSteps(t *testing.T) {
	t.Parallel()
	two, one := []string{"claude", "codex"}, []string{"claude"}
	list := []projects.Project{{Name: "api", Dir: "/src/api"}}
	cases := []struct {
		name    string
		mode    string
		kinds   []string
		list    []projects.Project
		project string
		repo    bool
		want    []spawnStep
	}{
		{"coding without projects", modes.Code, two, nil, "", false, []spawnStep{stepMode, stepKind, stepDir, stepDetails}},
		{"coding with a project", modes.Code, one, list, "api", false, []spawnStep{stepMode, stepProject, stepDetails}},
		{"coding in another folder", modes.Code, one, list, "", false, []spawnStep{stepMode, stepProject, stepDir, stepDetails}},
		{"coding in a git repo", modes.Code, one, list, "api", true, []spawnStep{stepMode, stepProject, stepWorktree, stepDetails}},
		{"review", modes.Review, two, list, "", false, []spawnStep{stepMode, stepKind, stepURL, stepDetails}},
		{"bug in a project repo", modes.Bug, one, list, "api", true, []spawnStep{stepMode, stepTicket, stepProject, stepWorktree, stepDetails}},
		{"bug without projects", modes.Bug, two, nil, "", false, []spawnStep{stepMode, stepKind, stepTicket, stepDir, stepDetails}},
		{"chat with one agent", modes.Chat, one, list, "", false, []spawnStep{stepMode, stepDetails}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := testForm(tc.kinds, tc.list, "/")
			f.mode = slices.IndexFunc(f.modes, func(m config.Mode) bool { return m.Name == tc.mode })
			f.project, f.repo = tc.project, tc.repo
			require.Equal(t, tc.want, f.steps())
		})
	}
}

func TestSpawnWizard(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "api"), 0o755))
	press := func(f spawnForm, code rune) spawnForm {
		f, _ = f.update(tea.KeyPressMsg{Code: code})
		return f
	}
	list := []projects.Project{{Name: "api", Dir: root + "/api", Agent: "codex"}, {Name: "web", Dir: root + "/web"}}

	f := testForm([]string{"claude", "codex"}, list, root+"/")
	f, _ = f.next()
	require.Equal(t, stepKind, f.step)
	f, _ = f.back()
	require.Equal(t, stepMode, f.step)
	f, _ = f.next()
	f, _ = f.next()
	require.Equal(t, stepProject, f.step)
	require.Len(t, f.matches, 3)

	f, _ = f.update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	require.Equal(t, []string{"web", ""}, []string{f.matches[0].value, f.matches[1].value})
	f = press(f, tea.KeyDown)
	f, _ = f.next()
	require.Equal(t, stepDir, f.step)

	f.dir.SetValue(root + "/nope")
	f, _ = f.next()
	require.Equal(t, stepDir, f.step)
	require.Contains(t, f.err, "not a directory")

	f, _ = f.back()
	f.filter.SetValue("api")
	f = f.suggest()
	f, _ = f.next()
	require.Equal(t, stepDetails, f.step)
	require.Equal(t, "defaults to api", f.name.Placeholder)

	r, err := f.request()
	require.NoError(t, err)
	require.Equal(t, []string{"claude", root + "/api", "api"}, []string{r.Kind, r.Dir, r.Name})
}

func TestSpawnWizardBug(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	f := testForm([]string{"claude"}, nil, root)
	f.mode = slices.IndexFunc(f.modes, func(m config.Mode) bool { return m.Name == modes.Bug })
	f, _ = f.next()
	require.Equal(t, stepTicket, f.step)
	f, _ = f.next()
	require.Equal(t, "a ticket is required", f.err)

	f.ticket.SetValue("SP-7")
	f, _ = f.next()
	require.Equal(t, stepDir, f.step)
	f, _ = f.next()
	require.Equal(t, stepDetails, f.step)
	require.Equal(t, "defaults to bug SP-7", f.name.Placeholder)

	r, err := f.request()
	require.NoError(t, err)
	require.Equal(t, []string{root, "bug SP-7", modes.Bug}, []string{r.Dir, r.Name, r.Mode})
	require.Contains(t, r.Prompt, "Fix the bug described in SP-7")
}

func TestSpawnWizardDirNavigation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, d := range []string{"api", "app/src"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, d), 0o755))
	}
	press := func(f spawnForm, code rune) spawnForm {
		f, _ = f.update(tea.KeyPressMsg{Code: code})
		return f
	}
	f, _ := testForm([]string{"claude"}, nil, root+"/").enter(stepDir)
	require.Len(t, f.matches, 2)

	f = press(f, tea.KeyDown)
	f = press(f, tea.KeyRight)
	require.Equal(t, root+"/app/", f.dir.Value())
	require.Equal(t, []suggestion{{"src/", "", root + "/app/src/"}}, f.matches)

	f = press(f, tea.KeyLeft)
	require.Equal(t, root+"/", f.dir.Value())
	require.True(t, f.dismissible())
}

func TestSpawnWizardWorktree(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	repo := filepath.Join(root, "api")
	require.NoError(t, os.Mkdir(repo, 0o755))
	for _, args := range [][]string{{"init", "-q"}, {"commit", "-q", "--allow-empty", "-m", "init"}} {
		out, err := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
	}

	f, _ := testForm([]string{"claude"}, nil, repo).enter(stepDir)
	f, _ = f.next()
	require.Equal(t, stepWorktree, f.step)
	f, _ = f.update(tea.KeyPressMsg{Code: tea.KeyDown})
	require.True(t, f.worktree)
	f, _ = f.next()
	require.Equal(t, stepDetails, f.step)
	f.name.SetValue("Refunds")

	r, err := f.request()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "api-worktrees", "refunds"), r.Dir)
	require.DirExists(t, r.Dir)
}
