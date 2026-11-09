package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

type Status string

const (
	Working    Status = "working"
	Background Status = "background"
	NeedsInput Status = "needs_input"
	Done       Status = "done"
	Idle       Status = "idle"
)

type Agent struct {
	Pane         string    `json:"pane"`
	Kind         string    `json:"kind"`
	Cwd          string    `json:"cwd"`
	Status       Status    `json:"status"`
	Message      string    `json:"message"`
	Prompt       string    `json:"prompt,omitempty"`
	Transcript   string    `json:"transcript,omitempty"`
	Touched      []string  `json:"touched,omitempty"`
	ClosedCost   float64   `json:"closed_cost_usd,omitempty"`
	BudgetWarned bool      `json:"budget_warned,omitempty"`
	Server       string    `json:"server,omitempty"`
	SessionID    string    `json:"session_id,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Stats struct {
	Model        string    `json:"model"`
	ContextUsed  int       `json:"context_used"`
	ContextMax   int       `json:"context_max"`
	CostUSD      float64   `json:"cost_usd"`
	LinesAdded   int       `json:"lines_added"`
	LinesRemoved int       `json:"lines_removed"`
	Server       string    `json:"server,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Closed struct {
	Name      string    `json:"name"`
	Label     string    `json:"label,omitempty"`
	Kind      string    `json:"kind"`
	Dir       string    `json:"dir"`
	Mode      string    `json:"mode,omitempty"`
	Parent    string    `json:"parent,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	Message   string    `json:"message,omitempty"`
	Diff      string    `json:"diff,omitempty"`
	CostUSD   float64   `json:"cost_usd,omitempty"`
	ClosedAt  time.Time `json:"closed_at"`
}

const closedLimit = 20

func root() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "agent-cockpit")
}

func MCPLog() string {
	return filepath.Join(root(), "mcp.log")
}

func LoadOrder() []string {
	var ids []string
	_ = read(filepath.Join(root(), "order.json"), &ids)
	return ids
}

func SaveOrder(ids []string) error {
	return write(filepath.Join(root(), "order.json"), ids)
}

func LoadGrouped() bool {
	var on bool
	_ = read(filepath.Join(root(), "group.json"), &on)
	return on
}

func SaveGrouped(on bool) error {
	return write(filepath.Join(root(), "group.json"), on)
}

func closedPath() string {
	return filepath.Join(root(), "closed.json")
}

func LoadClosed() []Closed {
	var list []Closed
	_ = read(closedPath(), &list)
	return list
}

func AddClosed(c Closed) error {
	unlock, err := lock()
	if err != nil {
		return err
	}
	defer unlock()
	list := append([]Closed{c}, LoadClosed()...)
	return write(closedPath(), list[:min(len(list), closedLimit)])
}

func RemoveClosed(c Closed) error {
	unlock, err := lock()
	if err != nil {
		return err
	}
	defer unlock()
	list := slices.DeleteFunc(LoadClosed(), func(x Closed) bool { return x.ClosedAt.Equal(c.ClosedAt) })
	return write(closedPath(), list)
}

func Dir() string {
	return filepath.Join(root(), "agents")
}

func statsDir() string {
	return filepath.Join(root(), "stats")
}

func file(dir, pane string) string {
	return filepath.Join(dir, strings.TrimPrefix(pane, "%")+".json")
}

func path(pane string) string {
	return file(Dir(), pane)
}

func write(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func read(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func Save(a Agent) error {
	a.UpdatedAt = time.Now()
	a.Server = tmux.ServerPID()
	return write(path(a.Pane), a)
}

func Load(pane string) (Agent, error) {
	var a Agent
	return a, read(path(pane), &a)
}

func Update(pane string, change func(prev Agent) (Agent, bool)) (prev, cur Agent, saved bool, err error) {
	unlock, err := lock()
	if err != nil {
		return prev, cur, false, err
	}
	defer unlock()
	prev, _ = Load(pane)
	cur, saved = change(prev)
	if !saved {
		return prev, cur, false, nil
	}
	return prev, cur, true, Save(cur)
}

func lock() (func(), error) {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(Dir(), ".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { f.Close() }, nil
}

func SaveStats(pane string, s Stats) error {
	s.UpdatedAt = time.Now()
	s.Server = tmux.ServerPID()
	return write(file(statsDir(), pane), s)
}

func LoadStats(pane string) (Stats, bool) {
	var s Stats
	return s, read(file(statsDir(), pane), &s) == nil
}

func List() ([]Agent, error) {
	files, err := filepath.Glob(filepath.Join(Dir(), "*.json"))
	if err != nil {
		return nil, err
	}
	agents := make([]Agent, 0, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var a Agent
		if json.Unmarshal(data, &a) == nil {
			agents = append(agents, a)
		}
	}
	return agents, nil
}

func Remove(pane string) error {
	_ = os.Remove(file(statsDir(), pane))
	return os.Remove(path(pane))
}
