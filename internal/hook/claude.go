package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/transcript"
)

type claudeInput struct {
	Event                string `json:"hook_event_name"`
	SessionID            string `json:"session_id"`
	Cwd                  string `json:"cwd"`
	Prompt               string `json:"prompt"`
	Message              string `json:"message"`
	LastAssistantMessage string `json:"last_assistant_message"`
	TranscriptPath       string `json:"transcript_path"`
	BackgroundTasks      []struct {
		Description string `json:"description"`
	} `json:"background_tasks"`
	SessionCrons []struct {
		Schedule string `json:"schedule"`
	} `json:"session_crons"`
}

func parseClaude(_ []string, stdin io.Reader) (event, error) {
	var in claudeInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return event{}, err
	}
	ev := event{cwd: in.Cwd, session: in.SessionID, transcript: in.TranscriptPath}
	switch in.Event {
	case "SessionStart":
		ev.status = state.Idle
	case "UserPromptSubmit":
		ev.status = state.Working
		if transcript.IsPrompt(in.Prompt) {
			ev.prompt = in.Prompt
		}
	case "Notification":
		ev.status, ev.message = state.NeedsInput, in.Message
	case "Stop":
		ev.status, ev.message = stopStatus(in)
	case "SessionEnd":
		ev.remove = true
	}
	return ev, nil
}

func stopStatus(in claudeInput) (state.Status, string) {
	last := lastMessage(in)
	if n := len(in.BackgroundTasks); n > 0 {
		var what []string
		for _, t := range in.BackgroundTasks {
			what = append(what, t.Description)
		}
		return state.Background, joinNonEmpty(fmt.Sprintf("%d in background (%s)", n, strings.Join(what, ", ")), last)
	}
	if len(in.SessionCrons) > 0 {
		return state.Background, joinNonEmpty("scheduled to wake ("+in.SessionCrons[0].Schedule+")", last)
	}
	return state.Done, last
}

func joinNonEmpty(head, tail string) string {
	if tail == "" {
		return head
	}
	return head + " · " + tail
}

func lastMessage(in claudeInput) string {
	if in.LastAssistantMessage != "" {
		return in.LastAssistantMessage
	}
	t, _ := transcript.Read(in.TranscriptPath)
	return t.Reply
}
