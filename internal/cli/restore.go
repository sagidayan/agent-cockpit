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
	if n, err := resume(missing); err != nil {
		return fmt.Errorf("restored %d of %d:\n%w", n, len(missing), err)
	}
	return nil
}

func resume(entries []snapshot.Entry) (int, error) {
	panes, err := control.Restore(entries)
	for _, p := range panes {
		fmt.Println(p)
	}
	return len(panes), err
}
