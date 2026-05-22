import type { Plugin } from "@opencode-ai/plugin"

// SquadNudge fires the squadai squad-nudge subcommand on session.created.
// Tolerates an off-PATH binary by also probing $HOME/.local/bin.
export const SquadNudge: Plugin = async () => ({
  event: async ({ event }) => {
    if (event.type !== "session.created") return

    const { spawn } = await import("node:child_process")
    const { existsSync } = await import("node:fs")
    const { homedir } = await import("node:os")
    const { join } = await import("node:path")

    const fallback = join(homedir(), ".local", "bin", "squadai")
    const candidates: string[] = ["squadai"]
    if (existsSync(fallback)) candidates.push(fallback)

    for (const cmd of candidates) {
      const ok = await new Promise<boolean>((resolve) => {
        const child = spawn(cmd, ["_hook", "squad-nudge"], {
          stdio: ["ignore", "inherit", "inherit"],
          detached: false,
        })
        child.on("close", (code) => resolve(code === 0))
        child.on("error", () => resolve(false))
      })
      if (ok) return
    }
  },
})
