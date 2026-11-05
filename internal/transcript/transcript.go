package transcript

import (
	"cmp"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sagidayan/agent-cockpit/internal/state"
)

const (
	tailBytes = 256 << 10
	maxHops   = 5
	slack     = 5 * time.Second
)

type Turn struct {
	Prompt      string
	Reply       string
	Started     time.Time
	Ended       time.Time
	Interrupted bool
	Cwd         string
}

type entry struct {
	Type        string    `json:"type"`
	Subtype     string    `json:"subtype"`
	IsMeta      bool      `json:"isMeta"`
	Timestamp   time.Time `json:"timestamp"`
	Cwd         string    `json:"cwd"`
	ContinuedIn string    `json:"continuedInSessionId"`
	Message     struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type block struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func Settle(a state.Agent) state.Agent {
	if a.Kind != "claude" {
		return a
	}
	t, ok := Read(File(a))
	if !ok {
		return a
	}
	a.Prompt = cmp.Or(a.Prompt, t.Prompt)
	a.Cwd = cmp.Or(t.Cwd, a.Cwd)
	if a.Status != state.Working && a.Status != state.NeedsInput {
		return a
	}
	if t.Ended.After(a.UpdatedAt) {
		a.Status, a.Message = state.Done, t.Reply
		if t.Interrupted {
			a.Status = state.Idle
		}
	}
	if t.Started.After(a.UpdatedAt.Add(-slack)) {
		if t.Prompt != "" {
			a.Prompt = t.Prompt
		}
		if a.Status == state.Working && t.Reply != "" {
			a.Message = t.Reply
		}
	}
	return a
}

func File(a state.Agent) string {
	if a.Transcript != "" || a.SessionID == "" {
		return a.Transcript
	}
	home, _ := os.UserHomeDir()
	slug := strings.NewReplacer("/", "-", ".", "-").Replace(a.Cwd)
	return filepath.Join(home, ".claude", "projects", slug, a.SessionID+".jsonl")
}

func Read(path string) (Turn, bool) {
	for range maxHops {
		if path == "" {
			return Turn{}, false
		}
		data, err := tail(path)
		if err != nil {
			return Turn{}, false
		}
		t, next := parse(data)
		if next == "" {
			return t, true
		}
		path = filepath.Join(filepath.Dir(path), next+".jsonl")
	}
	return Turn{}, false
}

func tail(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	start := max(0, info.Size()-tailBytes)
	data, err := io.ReadAll(io.NewSectionReader(f, start, info.Size()-start))
	if err != nil {
		return "", err
	}
	s := string(data)
	if start > 0 {
		_, s, _ = strings.Cut(s, "\n")
	}
	return s, nil
}

func parse(data string) (t Turn, continuedIn string) {
	for _, line := range strings.Split(data, "\n") {
		var e entry
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		t.Cwd = cmp.Or(e.Cwd, t.Cwd)
		switch {
		case e.Type == "continued-in":
			continuedIn = e.ContinuedIn
		case e.Type == "system" && (e.Subtype == "turn_duration" || e.Subtype == "stop_hook_summary"):
			t.Ended = e.Timestamp
		case e.Type == "assistant":
			if text := LastText(e.Message.Content); text != "" {
				t.Reply = text
			}
		case e.Type == "user" && !e.IsMeta:
			text, isText := userText(e.Message.Content)
			switch {
			case !isText:
			case strings.HasPrefix(text, "[Request interrupted by user"):
				t.Ended, t.Interrupted = e.Timestamp, true
			default:
				t = Turn{Prompt: t.Prompt, Started: e.Timestamp, Cwd: t.Cwd}
				if IsPrompt(text) {
					t.Prompt = text
				}
			}
		}
	}
	return t, continuedIn
}

func userText(raw json.RawMessage) (string, bool) {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, true
	}
	text := LastText(raw)
	return text, text != ""
}

func LastText(raw json.RawMessage) string {
	var blocks []block
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var text string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			text = b.Text
		}
	}
	return text
}

func IsPrompt(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && !strings.HasPrefix(s, "<")
}
