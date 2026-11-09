package main

import (
	"fmt"
	"os"

	"github.com/sagidayan/agent-cockpit/internal/cli"
	"github.com/sagidayan/agent-cockpit/internal/hook"
	"github.com/sagidayan/agent-cockpit/internal/install"
	"github.com/sagidayan/agent-cockpit/internal/mcpserver"
	"github.com/sagidayan/agent-cockpit/internal/statusline"
	"github.com/sagidayan/agent-cockpit/internal/tui"
)

const usage = `usage: cockpit <command>

  init [-y]            set up hooks, MCP and config for installed agents
  tui                  show the agent dashboard (default)
  hook <agent> [args]  record an agent event (claude|codex|opencode)
  setup <agent>        print hook config for an agent
  statusline           record claude statusline stats, then run your own statusline
  mcp                  run the MCP server (stdio)
  version              print the cockpit version

  new [--mode M] [--agent A] [--project P | --dir D] [--url U] [--ticket K]
      [--name N] [--label L] [--prompt T] [--worktree]
                       start an agent, print its pane id
  list [--json]        list running agents
  send TARGET TEXT...  send a prompt (TEXT - reads stdin)
  stop|kill TARGET     interrupt / close an agent
  restart TARGET       relaunch an agent in its pane, resuming its session
  label TARGET [TEXT]  label an agent (no TEXT clears)
  fav [TARGET | resume N|LABEL|NAME | rm N]
                       list, toggle, resume or remove favorites
  restore              restart agents from the last snapshot
  project [list | add NAME [DIR] | rm NAME]

  TARGET is a pane id, label or window name.`

var version = "dev"

func main() {
	cmd := "tui"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	if err := run(cmd, os.Args[min(2, len(os.Args)):]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd string, args []string) error {
	switch cmd {
	case "init":
		return install.Run(args)
	case "tui":
		return tui.Run()
	case "hook":
		if len(args) == 0 {
			return fmt.Errorf(usage)
		}
		return hook.Run(args[0], args[1:])
	case "setup":
		if len(args) == 0 {
			return fmt.Errorf(usage)
		}
		return install.Print(args[0])
	case "statusline":
		return statusline.Run()
	case "mcp":
		return mcpserver.Run()
	case "version", "--version", "-v":
		fmt.Println(version)
		return nil
	default:
		if cli.Has(cmd) {
			return cli.Run(cmd, args)
		}
		return fmt.Errorf(usage)
	}
}
