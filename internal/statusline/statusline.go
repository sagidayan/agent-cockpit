package statusline

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/state"
)

type input struct {
	Model struct {
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Cost struct {
		TotalCostUSD      float64 `json:"total_cost_usd"`
		TotalLinesAdded   int     `json:"total_lines_added"`
		TotalLinesRemoved int     `json:"total_lines_removed"`
	} `json:"cost"`
	ContextWindow struct {
		TotalInputTokens  int `json:"total_input_tokens"`
		ContextWindowSize int `json:"context_window_size"`
	} `json:"context_window"`
}

func Run() error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		logErr(err)
	}
	if pane := os.Getenv("TMUX_PANE"); pane != "" {
		logErr(save(pane, data))
	}
	return passthrough(data)
}

func save(pane string, data []byte) error {
	s, err := toStats(data)
	if err != nil {
		return err
	}
	return state.SaveStats(pane, s)
}

func toStats(data []byte) (state.Stats, error) {
	var in input
	if err := json.Unmarshal(data, &in); err != nil {
		return state.Stats{}, err
	}
	return state.Stats{
		Model:        in.Model.DisplayName,
		ContextUsed:  in.ContextWindow.TotalInputTokens,
		ContextMax:   in.ContextWindow.ContextWindowSize,
		CostUSD:      in.Cost.TotalCostUSD,
		LinesAdded:   in.Cost.TotalLinesAdded,
		LinesRemoved: in.Cost.TotalLinesRemoved,
	}, nil
}

func passthrough(data []byte) error {
	cfg, err := config.Load()
	if err != nil {
		logErr(err)
	}
	if cfg.Claude.Statusline == "" {
		return nil
	}
	cmd := exec.Command("sh", "-c", cfg.Claude.Statusline)
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	var exit *exec.ExitError
	if err := cmd.Run(); errors.As(err, &exit) {
		os.Exit(exit.ExitCode())
	} else if err != nil {
		return err
	}
	return nil
}

func logErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "cockpit statusline:", err)
	}
}
