package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

type Pane struct {
	ID         string
	Session    string
	Window     string
	WindowName string
	Cwd        string
	Command    string
	Label      string
	Mode       string
	Parent     string
	PID        string
}

const Session = "cockpit"

var mutating = []string{
	"send-keys", "paste-buffer", "split-window", "join-pane", "swap-pane", "select-pane", "kill-pane",
	"new-window", "select-window", "set-option", "respawn-pane", "switch-client",
}

func targetSession(target string) (string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "-t", target, "#{session_name}").Output()
	return strings.TrimSpace(string(out)), err
}

func guard(args []string, sessionOf func(string) (string, error)) error {
	if args[0] == "kill-server" || args[0] == "kill-session" {
		return fmt.Errorf("cockpit never runs tmux %s", args[0])
	}
	if !slices.Contains(mutating, args[0]) {
		return nil
	}
	if !slices.Contains(args, "-t") {
		return fmt.Errorf("tmux %s needs an explicit target in the %q session", args[0], Session)
	}
	for i, a := range args[:len(args)-1] {
		if a != "-t" && a != "-s" {
			continue
		}
		s, err := sessionOf(args[i+1])
		if err != nil {
			return fmt.Errorf("tmux %s: no target %s", args[0], args[i+1])
		}
		if s != Session {
			return fmt.Errorf("%s is in your %q tmux session; cockpit only changes its own %q session", args[i+1], s, Session)
		}
	}
	return nil
}

func run(args ...string) (string, error) {
	if err := guard(args, targetSession); err != nil {
		return "", err
	}
	out, err := exec.Command("tmux", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tmux %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

func ListPanes() ([]Pane, error) {
	out, err := run("list-panes", "-a", "-F",
		"#{pane_id}\t#{session_name}\t#{window_index}\t#{window_name}\t#{pane_current_path}\t#{pane_current_command}\t#{@cockpit_label}\t#{@cockpit_mode}\t#{@cockpit_parent}\t#{pane_pid}")
	if err != nil && noServer(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parsePanes(out), nil
}

func noServer(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "no server running") || strings.Contains(msg, "error connecting to")
}

func parsePanes(out string) []Pane {
	var panes []Pane
	seen := map[string]bool{}
	for line := range strings.SplitSeq(strings.TrimRight(out, "\n"), "\n") {
		f := strings.Split(line, "\t")
		if len(f) != 10 || seen[f[0]] {
			continue
		}
		seen[f[0]] = true
		panes = append(panes, Pane{f[0], f[1], f[2], f[3], f[4], f[5], f[6], f[7], f[8], f[9]})
	}
	return panes
}

func SendText(pane, text string) error {
	load := exec.Command("tmux", "load-buffer", "-b", "cockpit", "-")
	load.Stdin = strings.NewReader(text)
	if out, err := load.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux load-buffer: %w: %s", err, out)
	}
	if _, err := run("paste-buffer", "-p", "-d", "-b", "cockpit", "-t", pane); err != nil {
		return err
	}
	time.Sleep(150 * time.Millisecond)
	_, err := run("send-keys", "-t", pane, "Enter")
	return err
}

func Capture(pane string, lines int) (string, error) {
	return run("capture-pane", "-p", "-J", "-t", pane, "-S", fmt.Sprintf("-%d", lines))
}

func Focus(pane string) error {
	if _, err := run("switch-client", "-t", pane); err != nil {
		return err
	}
	if _, err := run("select-window", "-t", pane); err != nil {
		return err
	}
	_, err := run("select-pane", "-t", pane)
	return err
}

type Client struct {
	Name string
	Pane string
}

func Clients() ([]Client, error) {
	out, err := run("list-clients", "-F", "#{client_name}\t#{pane_id}")
	if err != nil {
		return nil, err
	}
	var clients []Client
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if name, pane, ok := strings.Cut(line, "\t"); ok {
			clients = append(clients, Client{name, pane})
		}
	}
	return clients, nil
}

func Display(client, msg string) error {
	_, err := run("display-message", "-l", "-c", client, msg)
	return err
}

func SplitRight(target, percent, command string) (string, error) {
	out, err := run("split-window", "-h", "-d", "-P", "-F", "#{pane_id}", "-l", percent, "-t", target, command)
	return strings.TrimSpace(out), err
}

func JoinRight(src, target, percent string) error {
	_, err := run("join-pane", "-h", "-d", "-l", percent, "-s", src, "-t", target)
	return err
}

func Swap(src, dst string) error {
	_, err := run("swap-pane", "-d", "-s", src, "-t", dst)
	return err
}

func Select(pane string) error {
	_, err := run("select-pane", "-t", pane)
	return err
}

func Kill(pane string) error {
	_, err := run("kill-pane", "-t", pane)
	return err
}

func NewWindow(session, name, dir, command string) (string, error) {
	out, err := run("new-window", "-d", "-P", "-F", "#{pane_id}", "-t", session+":", "-n", name, "-c", dir, command)
	return strings.TrimSpace(out), err
}

func SessionOf(pane string) (string, error) {
	out, err := run("display-message", "-p", "-t", pane, "#{session_name}")
	return strings.TrimSpace(out), err
}

func SendKeys(pane string, keys ...string) error {
	_, err := run(append([]string{"send-keys", "-t", pane}, keys...)...)
	return err
}

func SetLabel(pane, label string) error {
	if label == "" {
		_, err := run("set-option", "-p", "-u", "-t", pane, "@cockpit_label")
		return err
	}
	_, err := run("set-option", "-p", "-t", pane, "@cockpit_label", label)
	return err
}

func Popup(dir, title, command string) error {
	_, err := run("display-popup", "-E", "-w", "90%", "-h", "85%", "-d", dir, "-T", " "+title+" ", command)
	return err
}

func ServerPID() string {
	if parts := strings.Split(os.Getenv("TMUX"), ","); len(parts) >= 2 {
		return parts[1]
	}
	out, err := run("display-message", "-p", "#{pid}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func SplitLeft(target, percent, dir, command string) (string, error) {
	out, err := run("split-window", "-h", "-b", "-d", "-P", "-F", "#{pane_id}", "-l", percent, "-c", dir, "-t", target, command)
	return strings.TrimSpace(out), err
}

func SetPaneOption(pane, name, value string) error {
	_, err := run("set-option", "-p", "-t", pane, name, value)
	return err
}

func UnsetPaneOption(pane, name string) error {
	_, err := run("set-option", "-p", "-u", "-t", pane, name)
	return err
}

func PaneOptions(pane string, names ...string) ([]string, error) {
	out, err := run("display-message", "-p", "-t", pane, "#{"+strings.Join(names, "}\t#{")+"}")
	if err != nil {
		return nil, err
	}
	values := strings.SplitN(strings.TrimRight(out, "\n"), "\t", len(names))
	if len(values) != len(names) {
		return nil, fmt.Errorf("tmux: want %d options for %s, got %q", len(names), pane, out)
	}
	return values, nil
}

func PanesWithOption(name, value string) ([]string, error) {
	out, err := run("list-panes", "-a", "-F", "#{pane_id}\t#{"+name+"}")
	if err != nil {
		return nil, err
	}
	var ids []string
	for line := range strings.SplitSeq(strings.TrimRight(out, "\n"), "\n") {
		if id, v, ok := strings.Cut(line, "\t"); ok && v == value {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func EnsureSession() error {
	if _, err := run("has-session", "-t", "="+Session); err == nil {
		return nil
	}
	_, err := run("new-session", "-d", "-s", Session)
	return err
}

func Names(pane string) (label, window string, err error) {
	out, err := run("display-message", "-p", "-t", pane, "#{@cockpit_label}\t#{window_name}")
	if err != nil {
		return "", "", err
	}
	label, window, _ = strings.Cut(strings.TrimRight(out, "\n"), "\t")
	return label, window, nil
}

func Respawn(pane, dir, command string) error {
	_, err := run("respawn-pane", "-k", "-t", pane, "-c", dir, command)
	return err
}
