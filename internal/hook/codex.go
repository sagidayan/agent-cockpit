package hook

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/sagidayan/agent-cockpit/internal/state"
)

type codexPayload struct {
	Type                 string   `json:"type"`
	Cwd                  string   `json:"cwd"`
	ThreadID             string   `json:"thread-id"`
	LastAssistantMessage string   `json:"last-assistant-message"`
	InputMessages        []string `json:"input-messages"`
}

func parseCodex(args []string, _ io.Reader) (event, error) {
	if len(args) == 0 {
		return event{}, errors.New("codex: missing JSON payload argument")
	}
	var p codexPayload
	if err := json.Unmarshal([]byte(args[len(args)-1]), &p); err != nil {
		return event{}, err
	}
	if p.Type != "agent-turn-complete" {
		return event{}, nil
	}
	ev := event{status: state.Done, message: p.LastAssistantMessage, cwd: p.Cwd, session: p.ThreadID}
	if n := len(p.InputMessages); n > 0 {
		ev.prompt = p.InputMessages[n-1]
	}
	return ev, nil
}
