package cli

import (
	"fmt"
	"os"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/control"
	"github.com/sagidayan/agent-cockpit/internal/snapshot"
)

func restore(args []string) error {
	if len(args) > 0 {
		return errUsage
	}
	saved, err := snapshot.Load()
	if err != nil {
		return err
	}
	live, err := agents.List()
	if err != nil {
		return err
	}
	missing := snapshot.Missing(saved, live)
	if len(missing) == 0 {
		fmt.Fprintln(os.Stderr, "nothing to restore")
		return nil
	}
	for _, e := range missing {
		if _, err := os.Stat(e.Dir); err != nil {
			fmt.Fprintln(os.Stderr, "skipping:", err)
			continue
		}
		if err := resume(e, live); err != nil {
			return err
		}
	}
	return nil
}

func resume(e snapshot.Entry, live []agents.Live) error {
	session, err := control.Session()
	if err != nil {
		return err
	}
	pane, err := control.Resume(session, e, "", live)
	if pane != "" {
		fmt.Println(pane)
	}
	return err
}
