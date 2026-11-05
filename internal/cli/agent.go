package cli

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/control"
)

func one(args []string) (agents.Live, error) {
	if len(args) != 1 {
		return agents.Live{}, errUsage
	}
	return control.Find(args[0])
}

func stop(args []string) error {
	a, err := one(args)
	if err != nil {
		return err
	}
	return control.Stop(a)
}

func kill(args []string) error {
	a, err := one(args)
	if err != nil {
		return err
	}
	return control.Kill(a)
}

func label(args []string) error {
	if len(args) == 0 {
		return errUsage
	}
	a, err := control.Find(args[0])
	if err != nil {
		return err
	}
	return control.SetLabel(a, strings.Join(args[1:], " "))
}

func send(args []string) error {
	if len(args) < 2 {
		return errUsage
	}
	text, err := message(args[1:], os.Stdin)
	if err != nil {
		return err
	}
	a, err := control.Find(args[0])
	if err != nil {
		return err
	}
	return control.Send(a, text)
}

func message(words []string, stdin io.Reader) (string, error) {
	text := strings.Join(words, " ")
	if text == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", err
		}
		text = string(data)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("empty message")
	}
	return text, nil
}
