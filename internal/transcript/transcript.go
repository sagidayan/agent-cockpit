package transcript

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
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

type Message struct {
	Role string
	Text string
}

const (
	User      = "user"
	Assistant = "assistant"
	Tool      = "tool"
)

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
	Type  string `json:"type"`
	Text  string `json:"text"`
	Name  string `json:"name"`
	Input struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
	} `json:"input"`
}

var editTools = []string{"Edit", "MultiEdit", "Write", "NotebookEdit"}

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
	entries, ok := load(path)
	if !ok {
		return Turn{}, false
	}
	return turn(entries), true
}

func Conversation(path string, n int) ([]Message, bool) {
	entries, ok := load(path)
	if !ok {
		return nil, false
	}
	var msgs []Message
	for _, e := range entries {
		msgs = append(msgs, messages(e)...)
	}
	return msgs[max(0, len(msgs)-n):], true
}

func Edited(a state.Agent) []string {
	if a.Kind != "claude" {
		return nil
	}
	var files []string
	for _, path := range sessionFiles(File(a)) {
		files = editsIn(path, files)
	}
	return files
}

func sessionFiles(path string) []string {
	var files []string
	for range maxHops {
		if path == "" {
			break
		}
		subagents, _ := filepath.Glob(filepath.Join(strings.TrimSuffix(path, ".jsonl"), "subagents", "*.jsonl"))
		files = append(append(files, path), subagents...)
		next, ok := continuation(path)
		if !ok {
			break
		}
		path = filepath.Join(filepath.Dir(path), next+".jsonl")
	}
	return files
}

func continuation(path string) (string, bool) {
	data, err := tail(path)
	if err != nil {
		return "", false
	}
	_, next := decode(data)
	return next, next != ""
}

func editsIn(path string, files []string) []string {
	f, err := os.Open(path)
	if err != nil {
		return files
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		line, err := r.ReadBytes('\n')
		if bytes.Contains(line, []byte(`"tool_use"`)) {
			files = editedFiles(line, files)
		}
		if err != nil {
			return files
		}
	}
}

func editedFiles(line []byte, files []string) []string {
	var e entry
	var blocks []block
	if json.Unmarshal(line, &e) != nil || e.Type != "assistant" || json.Unmarshal(e.Message.Content, &blocks) != nil {
		return files
	}
	for _, b := range blocks {
		f := cmp.Or(b.Input.FilePath, b.Input.NotebookPath)
		if b.Type == "tool_use" && slices.Contains(editTools, b.Name) && f != "" && !slices.Contains(files, f) {
			files = append(files, f)
		}
	}
	return files
}

func load(path string) ([]entry, bool) {
	for range maxHops {
		if path == "" {
			return nil, false
		}
		data, err := tail(path)
		if err != nil {
			return nil, false
		}
		entries, next := decode(data)
		if next == "" {
			return entries, true
		}
		path = filepath.Join(filepath.Dir(path), next+".jsonl")
	}
	return nil, false
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

func decode(data string) (entries []entry, continuedIn string) {
	for _, line := range strings.Split(data, "\n") {
		var e entry
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		if e.Type == "continued-in" {
			continuedIn = e.ContinuedIn
		}
		entries = append(entries, e)
	}
	return entries, continuedIn
}

func turn(entries []entry) (t Turn) {
	for _, e := range entries {
		t.Cwd = cmp.Or(e.Cwd, t.Cwd)
		switch {
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
			case isInterrupt(text):
				t.Ended, t.Interrupted = e.Timestamp, true
			default:
				t = Turn{Prompt: t.Prompt, Started: e.Timestamp, Cwd: t.Cwd}
				if IsPrompt(text) {
					t.Prompt = text
				}
			}
		}
	}
	return t
}

func messages(e entry) []Message {
	switch {
	case e.Type == "assistant":
		return assistantMessages(e.Message.Content)
	case e.Type == "user" && !e.IsMeta:
		if text, _ := userText(e.Message.Content); IsPrompt(text) && !isInterrupt(text) {
			return []Message{{User, text}}
		}
	}
	return nil
}

func assistantMessages(raw json.RawMessage) []Message {
	var blocks []block
	if json.Unmarshal(raw, &blocks) != nil {
		return nil
	}
	var msgs []Message
	for _, b := range blocks {
		switch {
		case b.Type == "text" && b.Text != "":
			msgs = append(msgs, Message{Assistant, b.Text})
		case b.Type == "tool_use":
			msgs = append(msgs, Message{Tool, b.Name})
		}
	}
	return msgs
}

func isInterrupt(text string) bool {
	return strings.HasPrefix(text, "[Request interrupted by user")
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
