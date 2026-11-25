package hook

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

const dynamic = "\x00"

var tmuxKills = []string{"kill-server", "kill-session"}

var tmuxDestructive = []string{
	"kill-server", "kill-session", "kill-window", "kill-pane", "respawn-pane", "respawn-window", "unlink-window",
}

var tmuxAliases = map[string]string{
	"killp": "kill-pane", "killw": "kill-window", "respawnp": "respawn-pane", "respawnw": "respawn-window", "unlinkw": "unlink-window",
}

var shells = []string{"sh", "bash", "zsh", "dash", "ksh", "fish"}

const tmuxDenyReason = "cockpit blocked this command: it would kill part of the user's own tmux server (tmux %s). " +
	"Use a separate server for tests: pass -L <name> (or -S <socket>) to every tmux command, e.g. `tmux -L test-1234 kill-server`. If the user asked for this, tell them to run it themselves."

func tmuxGuard(command string) string {
	for _, words := range simpleCommands(command) {
		if cmd := blockedTmux(words); cmd != "" {
			return fmt.Sprintf(tmuxDenyReason, cmd)
		}
	}
	return ""
}

func blockedTmux(words []string) string {
	if i := slices.IndexFunc(words, isTmux); i >= 0 {
		return defaultServerMutation(words[i+1:])
	}
	dyn := false
	for _, w := range words {
		if dyn && (strings.HasPrefix(w, "kill-ser") || strings.HasPrefix(w, "kill-ses")) {
			return firstMatch([]string{w}, tmuxKills)
		}
		dyn = dyn || strings.Contains(w, dynamic)
	}
	return ""
}

func isTmux(w string) bool {
	return path.Base(w) == "tmux"
}

func defaultServerMutation(args []string) string {
	for len(args) > 0 && strings.HasPrefix(args[0], "-") && args[0] != "-" {
		flags := strings.TrimPrefix(args[0], "-")
		args = args[1:]
		if flags == "-" {
			break
		}
		for j, f := range flags {
			if !strings.ContainsRune("LSfcT", f) {
				continue
			}
			value := flags[j+1:]
			if value == "" && len(args) > 0 {
				value, args = args[0], args[1:]
			}
			if (f == 'L' || f == 'S') && path.Base(value) != "default" {
				return ""
			}
			break
		}
	}
	for _, seq := range tmuxSequence(args) {
		if cmd := firstMatch(seq[:1], tmuxDestructive); cmd != "" {
			return cmd
		}
	}
	return ""
}

func tmuxSequence(args []string) [][]string {
	var seqs [][]string
	var cur []string
	for _, a := range args {
		end := strings.HasSuffix(a, ";")
		if a = strings.TrimSuffix(a, ";"); a != "" {
			cur = append(cur, a)
		}
		if end && len(cur) > 0 {
			seqs, cur = append(seqs, cur), nil
		}
	}
	if len(cur) > 0 {
		seqs = append(seqs, cur)
	}
	return seqs
}

func firstMatch(words, names []string) string {
	for _, w := range words {
		if full, ok := tmuxAliases[w]; ok && slices.Contains(names, full) {
			return full
		}
		for _, n := range names {
			if w != "" && strings.HasPrefix(n, w) {
				return n
			}
		}
	}
	return ""
}

func simpleCommands(s string) [][]string {
	var cmds [][]string
	var words []string
	var word strings.Builder
	inWord, dq := false, false
	flushWord := func() {
		if inWord {
			words = append(words, word.String())
		}
		word.Reset()
		inWord = false
	}
	flushCmd := func() {
		flushWord()
		cmds = append(cmds, nestedShells(words)...)
		if len(words) > 0 {
			cmds = append(cmds, words)
		}
		words = nil
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s):
			i++
			if s[i] != '\n' {
				word.WriteByte(s[i])
				inWord = true
			}
		case c == '\'' && !dq:
			end := strings.IndexByte(s[i+1:], '\'')
			if end < 0 {
				end = len(s) - i - 1
			}
			word.WriteString(s[i+1 : i+1+end])
			inWord, i = true, i+1+end
		case c == '"':
			dq, inWord = !dq, true
		case c == '$' && i+1 < len(s) && s[i+1] == '(', c == '`':
			inner, next := substitution(s, i)
			cmds = append(cmds, simpleCommands(inner)...)
			word.WriteString(dynamic)
			inWord, i = true, next
		case c == '$':
			word.WriteString(dynamic)
			inWord = true
		case dq:
			word.WriteByte(c)
		case c == '>' || c == '&' && i+1 < len(s) && s[i+1] == '>':
			flushWord()
			next, toFile := outputRedirect(s, i)
			if toFile {
				words = append(words, redirect)
			}
			i = next
		case strings.IndexByte(";&|()\n", c) >= 0:
			flushCmd()
		case strings.IndexByte(" \t<", c) >= 0:
			flushWord()
		default:
			word.WriteByte(c)
			inWord = true
		}
	}
	flushCmd()
	return cmds
}

func outputRedirect(s string, i int) (int, bool) {
	if s[i] == '&' {
		i++
	}
	if i+1 < len(s) && (s[i+1] == '>' || s[i+1] == '|') {
		i++
	}
	if i+1 < len(s) && s[i+1] == '&' {
		i++
		j := i + 1
		for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == '-') {
			j++
		}
		if j > i+1 {
			return j - 1, false
		}
	}
	return i, true
}

func substitution(s string, i int) (string, int) {
	if s[i] == '`' {
		end := strings.IndexByte(s[i+1:], '`')
		if end < 0 {
			return s[i+1:], len(s)
		}
		return s[i+1 : i+1+end], i + 1 + end
	}
	depth := 0
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				return s[i+2 : j], j
			}
		}
	}
	return s[i+2:], len(s)
}

func nestedShells(words []string) [][]string {
	var cmds [][]string
	shell := false
	for i, w := range words {
		switch {
		case w == "eval":
			cmds = append(cmds, simpleCommands(strings.Join(words[i+1:], " "))...)
		case slices.Contains(shells, path.Base(w)):
			shell = true
		case shell && i+1 < len(words) && strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "--") && strings.ContainsRune(w, 'c'):
			cmds = append(cmds, simpleCommands(words[i+1])...)
		}
	}
	return cmds
}
