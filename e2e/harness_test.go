//go:build e2e

package e2e

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/state"
)

const config = `[notify]
desktop = false
tmux = false

[editor]
command = ""
`

var bin string

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("tmux"); err != nil {
		os.Exit(m.Run())
	}
	dir, err := os.MkdirTemp("", "cockpit-e2e-")
	if err != nil {
		panic(err)
	}
	bin = dir
	if err := build(dir); err != nil {
		os.RemoveAll(dir)
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func build(dir string) error {
	for out, pkg := range map[string]string{"cockpit": "../cmd/cockpit", "claude": "./testdata/fakeagent"} {
		if b, err := exec.Command("go", "build", "-o", filepath.Join(dir, out), pkg).CombinedOutput(); err != nil {
			return fmt.Errorf("build %s: %w: %s", pkg, err, b)
		}
	}
	return nil
}

type env struct {
	t      *testing.T
	name   string
	socket string
	pid    string
	dir    string
	home   string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	if bin == "" {
		t.Skip("tmux is not installed")
	}
	e := &env{t: t, name: label(t), dir: t.TempDir(), home: t.TempDir()}
	expected := socketPath(e.name)
	require.Less(t, len(expected), 100)
	require.NoFileExists(t, expected)
	require.NoError(t, os.MkdirAll(filepath.Join(e.home, ".config", "agent-cockpit"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(e.home, ".config", "agent-cockpit", "config.toml"), []byte(config), 0o644))
	_, err := e.run("-f", "/dev/null", "new-session", "-d", "-s", "e2e", "-x", "200", "-y", "50", "sleep 100000")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = e.run("kill-server")
		_ = os.Remove(e.socket)
	})
	e.socket, e.pid = e.format("e2e:", "#{socket_path}"), e.format("e2e:", "#{pid}")
	requireSameSocket(t, expected, e.socket)
	e.tmux("set-option", "-g", "default-shell", "/bin/sh")
	e.tmux("set-option", "-g", "remain-on-exit", "on")
	return e
}

func label(t *testing.T) string {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return "ce" + hex.EncodeToString(b)
}

func socketPath(name string) string {
	return filepath.Join(cmp.Or(os.Getenv("TMUX_TMPDIR"), "/tmp"), fmt.Sprintf("tmux-%d", os.Getuid()), name)
}

func requireSameSocket(t *testing.T, expected, actual string) {
	t.Helper()
	require.Equal(t, filepath.Base(expected), filepath.Base(actual))
	want, err := filepath.EvalSymlinks(filepath.Dir(expected))
	require.NoError(t, err)
	got, err := filepath.EvalSymlinks(filepath.Dir(actual))
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func (e *env) run(args ...string) (string, error) {
	cmd := exec.Command("tmux", append([]string{"-L", e.name}, args...)...)
	cmd.Env = append(os.Environ(), "TMUX=", "TMUX_PANE=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tmux %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func (e *env) tmux(args ...string) string {
	e.t.Helper()
	out, err := e.run(args...)
	require.NoError(e.t, err)
	return out
}

func (e *env) format(target, format string) string {
	e.t.Helper()
	return e.tmux("display-message", "-p", "-t", target, format)
}

func (e *env) environ(pane string) []string {
	return append(os.Environ(),
		"TMUX="+e.socket+","+e.pid+",0",
		"TMUX_PANE="+pane,
		"HOME="+e.home,
		"XDG_STATE_HOME="+filepath.Join(e.home, ".local", "state"),
		"XDG_CONFIG_HOME="+filepath.Join(e.home, ".config"),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
}

func (e *env) cockpit(args ...string) (string, error) {
	cmd := exec.Command(filepath.Join(bin, "cockpit"), args...)
	cmd.Env = e.environ("")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (e *env) hook(pane, event string, fields ...string) {
	e.t.Helper()
	in := map[string]string{"hook_event_name": event, "cwd": e.dir}
	for i := 0; i+1 < len(fields); i += 2 {
		in[fields[i]] = fields[i+1]
	}
	data, err := json.Marshal(in)
	require.NoError(e.t, err)
	cmd := exec.Command(filepath.Join(bin, "cockpit"), "hook", "claude")
	cmd.Env = e.environ(pane)
	cmd.Stdin = strings.NewReader(string(data))
	out, err := cmd.CombinedOutput()
	require.NoError(e.t, err, string(out))
	require.Empty(e.t, string(out))
}

func (e *env) window(session string, command ...string) string {
	e.t.Helper()
	if _, err := e.run("has-session", "-t", "="+session); err != nil {
		return e.tmux(append([]string{"new-session", "-d", "-P", "-F", "#{pane_id}", "-s", session, "-x", "200", "-y", "50", "-c", e.dir}, command...)...)
	}
	return e.tmux(append([]string{"new-window", "-d", "-P", "-F", "#{pane_id}", "-t", session + ":", "-c", e.dir}, command...)...)
}

func (e *env) agent(session string) string {
	e.t.Helper()
	pane := e.window(session, "exec "+filepath.Join(bin, "claude"))
	e.eventually(func() bool { return e.format(pane, "#{pane_current_command}") == "claude" })
	return pane
}

func (e *env) dashboard() string {
	e.t.Helper()
	return e.window("work", "-e", "HOME="+e.home,
		"-e", "XDG_STATE_HOME="+filepath.Join(e.home, ".local", "state"),
		"-e", "XDG_CONFIG_HOME="+filepath.Join(e.home, ".config"),
		"exec "+filepath.Join(bin, "cockpit"))
}

func (e *env) keys(pane string, keys ...string) {
	e.t.Helper()
	e.tmux(append([]string{"send-keys", "-t", pane}, keys...)...)
}

func (e *env) panes(target string) []string {
	e.t.Helper()
	return strings.Fields(e.tmux("list-panes", "-t", target, "-F", "#{pane_id}"))
}

func (e *env) windows(session string) []string {
	e.t.Helper()
	return strings.Fields(e.tmux("list-windows", "-t", "="+session, "-F", "#{window_id}"))
}

func (e *env) capture(pane string) string {
	e.t.Helper()
	return e.tmux("capture-pane", "-p", "-J", "-t", pane)
}

func (e *env) state(pane string) state.Agent {
	e.t.Helper()
	var a state.Agent
	data, err := os.ReadFile(filepath.Join(e.home, ".local", "state", "agent-cockpit", "agents", strings.TrimPrefix(pane, "%")+".json"))
	require.NoError(e.t, err)
	require.NoError(e.t, json.Unmarshal(data, &a))
	return a
}

func (e *env) viewing(dashboard, pane string) {
	e.t.Helper()
	e.eventually(func() bool { return slices.Contains(e.panes(dashboard), pane) })
}

func (e *env) showing(pane, text string) {
	e.t.Helper()
	e.eventually(func() bool { return strings.Contains(e.capture(pane), text) })
}

func (e *env) eventually(cond func() bool) {
	e.t.Helper()
	for deadline := time.Now().Add(15 * time.Second); !cond(); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			e.t.Fatal("timed out waiting for tmux state")
		}
	}
}
