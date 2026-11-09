package agents

import (
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const mcpGrace = 30 * time.Second

var (
	mcpKinds = []string{"claude", "opencode"}
	helpers  = []string{"daemon", "bg-spare", "bg-pty-host"}
)

type process struct {
	ppid string
	age  time.Duration
	args []string
}

func MarkMCP(live []Live) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,etime=,args=").Output()
	if err != nil {
		return
	}
	procs := parseProcesses(string(out))
	for i := range live {
		live[i].MCPDown = mcpDown(live[i].Kind, live[i].PID, procs)
	}
}

func parseProcesses(out string) map[string]process {
	procs := map[string]process{}
	for line := range strings.SplitSeq(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		procs[f[0]] = process{f[1], elapsed(f[2]), f[3:]}
	}
	return procs
}

func elapsed(etime string) time.Duration {
	var days int
	if d, clock, ok := strings.Cut(etime, "-"); ok {
		days, _ = strconv.Atoi(d)
		etime = clock
	}
	secs := 0
	for part := range strings.SplitSeq(etime, ":") {
		n, _ := strconv.Atoi(part)
		secs = secs*60 + n
	}
	return time.Duration(days*86400+secs) * time.Second
}

func mcpDown(kind, pid string, procs map[string]process) bool {
	if pid == "" || !slices.Contains(mcpKinds, kind) {
		return false
	}
	var agentAge time.Duration
	for p, proc := range procs {
		if !descends(p, pid, procs) {
			continue
		}
		switch filepath.Base(proc.args[0]) {
		case "cockpit":
			if len(proc.args) > 1 && proc.args[1] == "mcp" {
				return false
			}
		case kind:
			agentAge = max(agentAge, proc.age)
		}
	}
	return agentAge > mcpGrace
}

func descends(p, root string, procs map[string]process) bool {
	for range len(procs) {
		if p == root {
			return true
		}
		next, ok := procs[p]
		if !ok || next.ppid == p || len(next.args) > 1 && slices.Contains(helpers, next.args[1]) {
			return false
		}
		p = next.ppid
	}
	return false
}
