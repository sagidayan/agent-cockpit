package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/sagidayan/agent-cockpit/internal/state"
)

var opencodeStatuses = []state.Status{state.Working, state.NeedsInput, state.Done, state.Idle}

func parseOpencode(args []string, _ io.Reader) (event, error) {
	if len(args) > 2 && args[0] == "session" {
		ev, err := parseOpencode(args[2:], nil)
		ev.session = args[1]
		return ev, err
	}
	if len(args) == 2 && args[0] == "stats" {
		return opencodeStats(args[1])
	}
	if len(args) == 0 || !slices.Contains(opencodeStatuses, state.Status(args[0])) {
		return event{}, fmt.Errorf("opencode: want stats or one of %v, got %v", opencodeStatuses, args)
	}
	return event{status: state.Status(args[0]), message: strings.Join(args[1:], " ")}, nil
}

func opencodeStats(payload string) (event, error) {
	var s state.Stats
	if err := json.Unmarshal([]byte(payload), &s); err != nil {
		return event{}, err
	}
	return event{stats: &s}, nil
}
