package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/sagidayan/agent-cockpit/internal/paths"
)

var usages = map[string]string{
	"new":     "new [--mode code|review|bug|chat|NAME] [--agent claude|codex|opencode] [--project NAME | --dir DIR] [--url URL] [--ticket T] [--name N] [--label L] [--prompt TASK] [--worktree]",
	"list":    "list [--json]",
	"send":    "send TARGET TEXT... (TEXT - reads stdin)",
	"stop":    "stop TARGET",
	"kill":    "kill TARGET",
	"label":   "label TARGET [TEXT]",
	"fav":     "fav [TARGET | resume N|LABEL|NAME | rm N]",
	"restore": "restore",
	"project": "project [list | add NAME [DIR] | rm NAME]",
}

var commands = map[string]func([]string) error{
	"new":     newAgent,
	"list":    list,
	"send":    send,
	"stop":    stop,
	"kill":    kill,
	"label":   label,
	"fav":     fav,
	"restore": restore,
	"project": project,
}

func Has(cmd string) bool {
	_, ok := commands[cmd]
	return ok
}

func Run(cmd string, args []string) error {
	err := commands[cmd](args)
	if errors.Is(err, errUsage) {
		return fmt.Errorf("usage: cockpit %s", usages[cmd])
	}
	return err
}

var errUsage = errors.New("usage")

func parse(fs *flag.FlagSet, args []string) error {
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return usageError(fs, err)
	}
	if fs.NArg() > 0 {
		return usageError(fs, fmt.Errorf("unexpected argument %q", fs.Arg(0)))
	}
	return nil
}

func usageError(fs *flag.FlagSet, err error) error {
	if errors.Is(err, flag.ErrHelp) {
		return errUsage
	}
	return fmt.Errorf("%w\nusage: cockpit %s", err, usages[fs.Name()])
}

func resolveDir(input string) (string, error) {
	dir, err := filepath.Abs(paths.Expand(strings.TrimSpace(input)))
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", errors.New("not a directory: " + dir)
	}
	return dir, nil
}

func render(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	widths := make([]int, len(rows[0]))
	for _, r := range rows {
		for i, c := range r {
			widths[i] = max(widths[i], utf8.RuneCountInString(c))
		}
	}
	var b strings.Builder
	for _, r := range rows {
		var line strings.Builder
		for i, c := range r {
			line.WriteString(c + strings.Repeat(" ", widths[i]-utf8.RuneCountInString(c)+2))
		}
		b.WriteString(strings.TrimRight(line.String(), " ") + "\n")
	}
	return b.String()
}
