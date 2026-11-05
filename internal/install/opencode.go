package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const opencodePlugin = `import { execFile } from "node:child_process"

const cockpit = %q
const hook = (...args) => execFile(cockpit, ["hook", "opencode", ...args], () => {})
const statuses = {
  "session.execution.started": "working",
  "session.execution.succeeded": "done",
  "session.execution.failed": "done",
  "session.execution.interrupted": "idle",
}

export default {
  id: "agent-cockpit",
  setup: async (ctx) => {
    if (process.argv.includes("--service")) return
    const getSession = (sessionID) => ctx.session.get({ sessionID }).catch(() => ({}))
    const isChild = async (sessionID) => Boolean((await getSession(sessionID)).parentID)
    const models = new Map()
    const stats = async ({ sessionID, tokens: t }) => {
      const session = await getSession(sessionID)
      if (session.parentID) return
      const ref = models.get(sessionID) ?? {}
      const { data = [] } = await ctx.model.list().catch(() => ({}))
      const model = data.find((m) => m.id === ref.id && m.providerID === ref.providerID)
      hook("stats", JSON.stringify({
        model: model?.name ?? ref.id ?? "",
        context_used: t.input + t.output + t.reasoning + t.cache.read + t.cache.write,
        context_max: model?.limit.context ?? 0,
        cost_usd: session.cost ?? 0,
      }))
    }
    const handle = async ({ type, data }) => {
      if (type === "permission.asked") return hook("needs_input", [data.action, ...(data.resources ?? [])].join(" "))
      if (type === "permission.replied") return hook("working")
      if (type === "session.step.started") return models.set(data.sessionID, data.model)
      if (type === "session.step.ended") return stats(data)
      if (statuses[type] && !(await isChild(data.sessionID))) hook("session", data.sessionID, statuses[type])
    }
    ;(async () => {
      for await (const event of ctx.event.subscribe()) await handle(event)
    })().catch(() => {})
  },
}
`

func installOpencode(exe string) error {
	path := filepath.Join(configHome(), "opencode", "plugins", "agent-cockpit.js")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, fmt.Appendf(nil, opencodePlugin, exe), 0o644); err != nil {
		return err
	}
	done("plugin written to " + path)
	out, _ := exec.Command("opencode", "mcp", "list").CombinedOutput()
	if strings.Contains(string(out), "cockpit") {
		done("MCP server already registered (opencode mcp)")
	} else if err := command("opencode", "mcp", "add", "--global", "cockpit", "--", exe, "mcp"); err != nil {
		return err
	} else {
		done("MCP server registered (opencode mcp, global)")
	}
	fmt.Println("  ! run opencode with --standalone so cockpit can track it")
	return nil
}
