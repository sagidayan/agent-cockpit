package hook

import (
	"cmp"
	"fmt"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/sagidayan/agent-cockpit/internal/launch"
)

const redirect = "\x01"

const leadEditReason = "cockpit blocked this: you are a lead session, so you do not edit files yourself. " +
	"Spawn a child with spawn_agent and give it the context it needs."

const leadShellReason = "cockpit blocked this command: you are a lead session, and your shell only runs read-only commands (`%s` is not one). " +
	"Spawn a child with spawn_agent to make changes, run builds or tests, or monitor something, and give it the context it needs. " +
	"If the user wants to run it here, ask them to run it themselves or start a hands-on session."

var editTools = []string{"Edit", "Write", "NotebookEdit", "edit", "write", "patch"}

var shellTools = []string{"Bash", "bash", "shell"}

func isLead() bool {
	return os.Getenv(launch.LeadEnv) == "1"
}

func guard(tool, command string, lead bool) string {
	switch {
	case lead && slices.Contains(editTools, tool):
		return leadEditReason
	case !slices.Contains(shellTools, tool):
		return ""
	case tmuxGuard(command) != "":
		return tmuxGuard(command)
	case lead:
		return leadShellGuard(command)
	}
	return ""
}

func leadShellGuard(command string) string {
	for _, words := range simpleCommands(command) {
		if !readOnly(words) {
			return fmt.Sprintf(leadShellReason, strings.ReplaceAll(strings.Join(words, " "), redirect, ">"))
		}
	}
	return ""
}

var assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

func readOnly(words []string) bool {
	words, ok := withoutRedirects(words)
	for ok && len(words) > 0 && assignment.MatchString(words[0]) {
		words = words[1:]
	}
	if !ok || len(words) == 0 {
		return ok
	}
	if strings.Contains(words[0], dynamic) {
		return false
	}
	switch name, args := path.Base(words[0]), words[1:]; name {
	case "xargs":
		return readOnly(afterFlags(args, "-n", "-I", "-P", "-L", "-d", "-E", "-s", "-a"))
	case "env":
		return readOnly(afterFlags(args, "-u", "-C", "-S"))
	default:
		check, known := readOnlyCommands[name]
		return known && (check == nil || check(args))
	}
}

var harmlessTargets = []string{"/dev/null", "/dev/stdout", "/dev/stderr"}

func withoutRedirects(words []string) ([]string, bool) {
	var kept []string
	for i := 0; i < len(words); i++ {
		if words[i] != redirect {
			kept = append(kept, words[i])
			continue
		}
		if i+1 >= len(words) || !slices.Contains(harmlessTargets, words[i+1]) {
			return nil, false
		}
		i++
	}
	return kept, true
}

var readOnlyCommands = map[string]func(args []string) bool{
	"cat": nil, "head": nil, "tail": nil, "ls": nil, "tree": nil, "pwd": nil, "cd": nil, "echo": nil, "printf": nil,
	"grep": nil, "egrep": nil, "fgrep": nil, "rg": nil, "ag": nil, "wc": nil, "uniq": nil, "cut": nil, "tr": nil,
	"jq": nil, "yq": nil, "diff": nil, "cmp": nil, "comm": nil, "file": nil, "stat": nil, "du": nil, "df": nil,
	"which": nil, "whereis": nil, "type": nil, "date": nil, "basename": nil, "dirname": nil, "realpath": nil,
	"readlink": nil, "column": nil, "nl": nil, "fold": nil, "true": nil, "false": nil, "test": nil, "[": nil,
	"xxd": nil, "od": nil, "strings": nil, "shasum": nil, "sha256sum": nil, "md5": nil, "md5sum": nil,
	"uname": nil, "whoami": nil, "id": nil, "hostname": nil, "ps": nil,
	"sort":    func(args []string) bool { return !hasFlag(args, "-o", "--output") },
	"sed":     func(args []string) bool { return !hasShortFlag(args, 'i') && !hasFlag(args, "--in-place") },
	"awk":     func(args []string) bool { return !slices.ContainsFunc(args, writesFromAwk) },
	"find":    func(args []string) bool { return !hasFlag(args, findWrites...) },
	"curl":    readOnlyCurl,
	"git":     readOnlyGit,
	"gh":      readOnlyGh,
	"jira":    readOnlyJira,
	"kubectl": readOnlyKubectl,
	"kubectx": func(args []string) bool { return len(args) == 0 || hasFlag(args, "-c", "--current") },
	"kubens":  func(args []string) bool { return len(args) == 0 || hasFlag(args, "-c", "--current") },
	"gcloud":  readOnlyGcloud,
	"bq":      readOnlyBq,
	"go": func(args []string) bool {
		return len(args) > 0 && slices.Contains([]string{"doc", "list", "env", "version"}, args[0])
	},
	"tmux":    readOnlyTmux,
	"cockpit": readOnlyCockpit,
}

var findWrites = []string{"-delete", "-exec", "-execdir", "-ok", "-okdir", "-fprint", "-fprint0", "-fprintf", "-fls"}

func hasFlag(args []string, flags ...string) bool {
	return slices.ContainsFunc(args, func(a string) bool {
		return slices.ContainsFunc(flags, func(f string) bool { return a == f || strings.HasPrefix(a, f+"=") })
	})
}

func hasShortFlag(args []string, f rune) bool {
	return slices.ContainsFunc(args, func(a string) bool {
		return strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsRune(a[1:], f)
	})
}

func writesFromAwk(arg string) bool {
	return strings.Contains(arg, "system") || strings.ContainsAny(arg, ">|")
}

func afterFlags(args []string, withValue ...string) []string {
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		if slices.Contains(withValue, args[0]) {
			args = args[1:]
		}
		args = args[1:]
	}
	if len(args) == 0 {
		return []string{"true"}
	}
	return args
}

func positionals(args []string, withValue ...string) []string {
	var pos []string
	for i := 0; i < len(args); i++ {
		switch {
		case slices.Contains(withValue, args[i]):
			i++
		case !strings.HasPrefix(args[i], "-"):
			pos = append(pos, args[i])
		}
	}
	return pos
}

func first(args []string, n int) string {
	if len(args) > n {
		return args[n]
	}
	return ""
}

func readMethod(method string) bool {
	return slices.Contains([]string{"GET", "HEAD"}, strings.ToUpper(method))
}

func readOnlyCurl(args []string) bool {
	for i, a := range args {
		switch {
		case hasFlag([]string{a}, "--output", "--remote-name", "--data", "--data-raw", "--data-binary", "--data-urlencode", "--form", "--upload-file", "--json"):
			return false
		case a == "--request":
			if !readMethod(first(args, i+1)) {
				return false
			}
		case strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--"):
			flags, method, _ := strings.Cut(a[1:], "X")
			if strings.ContainsAny(flags, "oOdFT") || strings.Contains(a, "X") && !readMethod(cmp.Or(method, first(args, i+1))) {
				return false
			}
		}
	}
	return true
}

var gitRead = map[string]func(args []string) bool{
	"status": nil, "log": nil, "diff": nil, "show": nil, "blame": nil, "annotate": nil, "shortlog": nil, "describe": nil,
	"rev-parse": nil, "rev-list": nil, "ls-files": nil, "ls-tree": nil, "ls-remote": nil, "grep": nil, "cat-file": nil,
	"merge-base": nil, "name-rev": nil, "show-ref": nil, "for-each-ref": nil, "range-diff": nil, "cherry": nil,
	"check-ignore": nil, "count-objects": nil, "fetch": nil, "version": nil, "help": nil,
	"reflog": func(args []string) bool { return first(args, 0) == "" || first(args, 0) == "show" },
	"remote": func(args []string) bool {
		return slices.Contains([]string{"", "-v", "--verbose", "show", "get-url"}, first(args, 0))
	},
	"stash":    func(args []string) bool { return slices.Contains([]string{"list", "show"}, first(args, 0)) },
	"worktree": func(args []string) bool { return first(args, 0) == "list" },
	"config": func(args []string) bool {
		return hasFlag(args, "--get", "--get-all", "--get-regexp", "--list", "-l", "get", "list")
	},
	"tag": func(args []string) bool {
		return len(args) == 0 || hasFlag(args, "-l", "--list", "--contains", "--points-at")
	},
	"branch": func(args []string) bool {
		if hasFlag(args, "-d", "-D", "-m", "-M", "-c", "-C", "-f", "-u", "--delete", "--move", "--copy", "--force", "--set-upstream-to", "--unset-upstream", "--edit-description") {
			return false
		}
		return len(positionals(args)) == 0 || hasFlag(args, "-l", "--list", "--contains", "--merged", "--no-merged", "--points-at")
	},
}

func readOnlyGit(args []string) bool {
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		if slices.Contains([]string{"-C", "-c", "--git-dir", "--work-tree", "--namespace"}, args[0]) {
			args = args[1:]
		}
		args = args[1:]
	}
	if len(args) == 0 || slices.ContainsFunc(args, func(a string) bool { return strings.HasPrefix(a, "--output") }) {
		return len(args) == 0
	}
	check, ok := gitRead[args[0]]
	return ok && (check == nil || check(args[1:]))
}

var ghRead = map[string][]string{
	"pr":       {"view", "list", "diff", "checks", "status"},
	"issue":    {"view", "list", "status"},
	"run":      {"view", "list"},
	"workflow": {"view", "list"},
	"release":  {"view", "list"},
	"repo":     {"view", "list"},
	"gist":     {"view", "list"},
	"label":    {"list"},
	"cache":    {"list"},
	"auth":     {"status"},
}

func readOnlyGh(args []string) bool {
	switch first(args, 0) {
	case "search", "status", "version", "help", "--version", "--help":
		return true
	case "api":
		return readOnlyGhAPI(args[1:])
	}
	return slices.Contains(ghRead[first(args, 0)], first(args, 1))
}

func readOnlyGhAPI(args []string) bool {
	for i, a := range args {
		method := ""
		switch {
		case a == "-X" || a == "--method":
			method = first(args, i+1)
		case strings.HasPrefix(a, "--method="):
			method = strings.TrimPrefix(a, "--method=")
		case strings.HasPrefix(a, "-X"):
			method = a[2:]
		case hasFlag([]string{a}, "--field", "--raw-field", "--input") || strings.HasPrefix(a, "-f") || strings.HasPrefix(a, "-F"):
			return false
		}
		if method != "" && strings.ToUpper(method) != "GET" {
			return false
		}
	}
	return true
}

func readOnlyJira(args []string) bool {
	switch first(args, 0) {
	case "me", "version", "serverinfo", "help", "--help":
		return true
	case "issue", "sprint", "epic", "board", "project", "release":
		return slices.Contains([]string{"list", "ls", "view"}, first(args, 1))
	}
	return false
}

func readOnlyKubectl(args []string) bool {
	pos := positionals(args, "-n", "--namespace", "--context", "--cluster", "--kubeconfig", "-l", "--selector", "-o", "--output", "-c", "--container")
	switch first(pos, 0) {
	case "get", "describe", "logs", "top", "explain", "api-resources", "api-versions", "version", "cluster-info", "events":
		return true
	case "auth":
		return first(pos, 1) == "can-i"
	case "config":
		return slices.Contains([]string{"get-contexts", "current-context", "view", "get-clusters"}, first(pos, 1))
	}
	return false
}

var gcloudReadVerbs = []string{"list", "describe", "read", "get-value", "get-iam-policy", "print-settings"}

func readOnlyGcloud(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			break
		}
		if slices.Contains(gcloudReadVerbs, a) {
			return true
		}
	}
	return false
}

var sqlRead = regexp.MustCompile(`(?i)^\s*(select|with)\b`)

func readOnlyBq(args []string) bool {
	pos := positionals(args)
	switch first(pos, 0) {
	case "ls", "show", "head", "version", "help":
		return true
	case "query":
		return len(pos) > 1 && sqlRead.MatchString(pos[1]) && !hasFlag(args, "--destination_table", "--replace", "--append_table")
	}
	return false
}

var tmuxRead = []string{"ls", "list-sessions", "list-windows", "lsw", "list-panes", "lsp", "capture-pane", "capturep", "display-message", "display", "show-options", "show", "has-session", "has"}

func readOnlyTmux(args []string) bool {
	return slices.Contains(tmuxRead, first(positionals(args, "-L", "-S", "-f"), 0))
}

func readOnlyCockpit(args []string) bool {
	switch first(args, 0) {
	case "list", "help", "version", "--help":
		return true
	case "project":
		return len(args) == 1 || args[1] == "list"
	case "fav":
		return len(args) == 1
	}
	return false
}
