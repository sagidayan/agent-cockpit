package launch

import "strings"

var commands = map[string]string{
	"claude":   "claude",
	"codex":    "codex",
	"opencode": "opencode --standalone",
}

var resumeArgs = map[string]string{
	"claude":   "--resume",
	"codex":    "resume",
	"opencode": "--session",
}

func Command(kind, resumeID string) string {
	arg, ok := resumeArgs[kind]
	if resumeID == "" || !ok {
		return commands[kind]
	}
	return commands[kind] + " " + arg + " " + Quote(resumeID)
}

var promptArgs = map[string]string{
	"claude":   "",
	"codex":    "",
	"opencode": "--prompt ",
}

func Start(kind, prompt string) string {
	arg, ok := promptArgs[kind]
	if prompt == "" || !ok {
		return commands[kind]
	}
	return commands[kind] + " " + arg + Quote(prompt)
}

func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
