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

const LeadEnv = "COCKPIT_LEAD"

const LeadPrompt = `You are a lead session in agent-cockpit. You talk with the user, hold the context, plan and delegate. You do not change anything yourself: no file edits, commits, pushes, builds, test runs, deploys, ticket or PR updates, and no monitoring. Your file edit tools are blocked and your shell only runs read-only commands (reading and searching files, git log/diff/status/show, gh and jira views, kubectl get/describe/logs).

When something needs doing, spawn a child with the cockpit spawn_agent tool and give it a self-contained brief: the goal, the files, decisions and constraints that matter, what is already ruled out, and what to report back. Pass worktree: true when several children work in one repo. Then wait_for_agent, review with get_output and get_diff (review_agent for an independent review), send fixes with send_task, and close_agent once the work is done. Keep children's reports short so your context stays focused on the user's goals.

Answer questions about the code yourself by reading it. Ask the user before anything outward-facing.`

func Command(kind, resumeID string, lead bool) string {
	arg, ok := resumeArgs[kind]
	if resumeID == "" || !ok {
		return withLead(kind, commands[kind], lead)
	}
	return withLead(kind, commands[kind]+" "+arg+" "+Quote(resumeID), lead)
}

var promptArgs = map[string]string{
	"claude":   "",
	"codex":    "",
	"opencode": "--prompt ",
}

func Start(kind, prompt string, lead bool) string {
	arg, ok := promptArgs[kind]
	if !ok {
		return ""
	}
	if lead && kind != "claude" {
		prompt = strings.TrimSpace(LeadPrompt + "\n\n" + prompt)
	}
	if prompt == "" {
		return withLead(kind, commands[kind], lead)
	}
	return withLead(kind, commands[kind]+" "+arg+Quote(prompt), lead)
}

func withLead(kind, command string, lead bool) string {
	if !lead || command == "" {
		return command
	}
	if kind == "claude" {
		command = "claude --disallowedTools Edit,Write,NotebookEdit --append-system-prompt " + Quote(LeadPrompt) + strings.TrimPrefix(command, "claude")
	}
	return LeadEnv + "=1 " + command
}

func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
