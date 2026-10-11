// Written by `squadai apply` from usage in .squadai/project.json. squadai
// rewrites or deletes this file when usage.enforcement changes; edits are lost.
import { spawn } from "node:child_process";
import type { ExtensionAPI, ExtensionContext } from "@earendil-works/pi-coding-agent";

const ENFORCEMENT = "{{ENFORCEMENT}}";
const TIMEOUT_MS = 10_000;

type HookResult = { code: number; stdout: string; stderr: string };

// Same stdin payload and exit contract as the Claude Code hooks squadai writes:
// exit 2 refuses with the reason on stderr, exit 0 may carry JSON on stdout.
function budgetHook(event: string, ctx: ExtensionContext): Promise<HookResult> {
	return new Promise((resolve) => {
		const child = spawn("squadai", ["budget-hook", "--harness", "pi", "--event", event], {
			cwd: ctx.cwd,
			timeout: TIMEOUT_MS,
		});
		let stdout = "";
		let stderr = "";
		child.stdout.on("data", (d) => (stdout += d));
		child.stderr.on("data", (d) => (stderr += d));
		child.stdin.on("error", () => {});
		child.on("error", (err) => resolve({ code: -1, stdout, stderr: String(err) }));
		child.on("close", (code) => resolve({ code: code ?? -1, stdout, stderr }));
		child.stdin.end(JSON.stringify({ cwd: ctx.cwd, transcript_path: ctx.sessionManager.getSessionFile() ?? "" }));
	});
}

function parse(r: HookResult): any {
	try {
		return JSON.parse(r.stdout);
	} catch {
		return {};
	}
}

function reason(r: HookResult): string {
	const line = r.stderr.split("\n")[0].replace(/^Error E-\d+: /, "").trim();
	return line || "squadai token budget exceeded";
}

// A hook that could not run refuses under block, like onFailure "block" in Claude Code.
function failed(r: HookResult): boolean {
	return r.code !== 0 && r.code !== 2 && ENFORCEMENT === "block";
}

// JSON and print modes have no UI, so the message would otherwise vanish.
function tell(ctx: ExtensionContext, msg: string, level: "warning" | "error"): void {
	if (ctx.hasUI) ctx.ui.notify(msg, level);
	else process.stderr.write(msg + "\n");
}

export default function (pi: ExtensionAPI) {
	pi.on("tool_call", async (_event, ctx) => {
		// warn stays quiet on tool calls, so skip the process spawn.
		if (ENFORCEMENT === "warn") return undefined;
		const r = await budgetHook("PreToolUse", ctx);
		if (r.code === 2 || failed(r)) return { block: true, reason: reason(r) };
		const out = parse(r).hookSpecificOutput;
		if (out?.permissionDecision !== "ask") return undefined;
		if (ctx.hasUI && (await ctx.ui.confirm("squadai token budget", `${out.permissionDecisionReason}\n\nRun this tool anyway?`))) {
			return undefined;
		}
		return { block: true, reason: out.permissionDecisionReason };
	});

	pi.on("input", async (event, ctx) => {
		if (event.source === "extension") return { action: "continue" };
		const r = await budgetHook("UserPromptSubmit", ctx);
		if (r.code === 2 || failed(r)) {
			tell(ctx, reason(r), "error");
			return { action: "handled" };
		}
		const msg = parse(r).systemMessage;
		if (msg) tell(ctx, msg, "warning");
		return { action: "continue" };
	});

	// Notify only: returning continue here would keep the agent working.
	pi.on("agent_before_settle", async (_event, ctx) => {
		const msg = parse(await budgetHook("Stop", ctx)).systemMessage;
		if (msg) tell(ctx, msg, "warning");
		return undefined;
	});
}
