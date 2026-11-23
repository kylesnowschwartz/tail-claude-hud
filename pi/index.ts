// Pi extension: draws pi's status line with the tail-claude-hud binary, so pi
// and Claude Code show the same line from one configuration. The extension
// sends pi's state to the binary in Claude Code's status-line format and
// shows the binary's output as the footer. Other extensions' statuses join
// the first row. The thinking level sits right-aligned above the prompt,
// where Claude Code shows its effort level.

import { execFile } from "node:child_process";
import type { ExtensionAPI, ExtensionContext } from "@earendil-works/pi-coding-agent";
import { truncateToWidth, visibleWidth } from "@earendil-works/pi-tui";
import { effortLabel, footerRows, hudRows, nextRefreshDelay, statusPayload, type CacheEntry } from "./payload.ts";

const binary = process.env.TCH_HUD_BINARY || "tail-claude-hud";
const effortWidget = "tail-claude-hud-effort";

/** How often the line redraws while nothing happens, which keeps the git state and the cache countdown current. */
const idleRefreshMs = 15_000;
const binaryTimeoutMs = 2000;

/** Failed runs in a row after which pi's own footer comes back. */
const maxFailedRuns = 3;

export default function (pi: ExtensionAPI): void {
	let ctx: ExtensionContext | undefined;
	let rows: string[] = [];
	let requestRender = (): void => {};
	let running = false;
	let rerun = false;
	let failedRuns = 0;
	let columns = 0;
	let timer: NodeJS.Timeout | undefined;

	const schedule = (delay: number): void => {
		clearTimeout(timer);
		timer = setTimeout(refresh, delay);
		timer.unref();
	};

	/** Give the footer back to pi when the binary cannot be started. */
	const stop = (reason: string): void => {
		clearTimeout(timer);
		const current = ctx;
		ctx = undefined;
		if (!current) return;
		current.ui.setFooter(undefined);
		current.ui.setWidget(effortWidget, undefined);
		current.ui.notify(`tail-claude-hud status line is off: ${reason}`, "warning");
	};

	/** Run the binary on pi's current state and show its output. One run at a time; a request during a run starts one more after it. */
	function refresh(): void {
		if (!ctx) return;
		if (running) {
			rerun = true;
			return;
		}
		let payload: Record<string, unknown>;
		try {
			payload = statusPayload({
				sessionId: ctx.sessionManager.getSessionId(),
				cwd: ctx.cwd,
				model: ctx.model,
				contextUsage: ctx.getContextUsage(),
				entries: ctx.sessionManager.getBranch() as readonly CacheEntry[],
				cacheRetention: process.env.PI_CACHE_RETENTION === "long" ? "long" : "short",
			});
		} catch {
			// The session was replaced; its successor draws the line.
			return;
		}
		running = true;
		const child = execFile(
			binary,
			[],
			{ timeout: binaryTimeoutMs, env: { ...process.env, COLUMNS: String(Math.max(20, (columns || 120) - 2)) } },
			(error, stdout) => {
				running = false;
				if (!ctx) {
					rerun = false;
					return;
				}
				if (error && (error as NodeJS.ErrnoException).code === "ENOENT") {
					stop(`"${binary}" is not on the PATH.`);
					return;
				}
				if (error) {
					failedRuns += 1;
					if (failedRuns >= maxFailedRuns) {
						stop(`"${binary}" failed or took longer than ${binaryTimeoutMs / 1000} s, ${maxFailedRuns} times in a row.`);
						return;
					}
				} else {
					failedRuns = 0;
					rows = hudRows(stdout);
					requestRender();
				}
				if (rerun) {
					rerun = false;
					refresh();
					return;
				}
				const cache = payload.prompt_cache as Parameters<typeof nextRefreshDelay>[0];
				schedule(nextRefreshDelay(cache, Date.now(), idleRefreshMs));
			},
		);
		child.stdin?.on("error", () => {});
		child.stdin?.end(JSON.stringify(payload));
	}

	/** Take the footer and the line above the prompt. The latest footer wins in pi, so this runs again when another extension may have set its own. */
	const install = (current: ExtensionContext): void => {
		current.ui.setFooter((tui, theme, footerData) => {
			requestRender = () => tui.requestRender();
			const unsubscribe = footerData.onBranchChange(refresh);
			const separator = theme.fg("dim", " | ");
			return {
				dispose: unsubscribe,
				invalidate() {},
				render(width: number): string[] {
					if (width !== columns) {
						columns = width;
						refresh();
					}
					const statuses = [...footerData.getExtensionStatuses()]
						.sort(([a], [b]) => a.localeCompare(b))
						.map(([, text]) => text);
					return footerRows(rows, statuses, separator).map((row) =>
						truncateToWidth(row, width, theme.fg("dim", "...")),
					);
				},
			};
		});

		current.ui.setWidget(effortWidget, (_tui, theme) => ({
			invalidate() {},
			render(width: number): string[] {
				if (!ctx?.model?.reasoning) return [];
				const label = theme.fg("dim", effortLabel(pi.getThinkingLevel()));
				const pad = Math.max(0, width - visibleWidth(label) - 2);
				return [truncateToWidth(" ".repeat(pad) + label, width)];
			},
		}));
	};

	pi.on("session_start", (_event, startCtx) => {
		if (startCtx.mode !== "tui") return;
		ctx = startCtx;
		install(startCtx);
		refresh();
	});

	pi.on("model_select", (_event, eventCtx) => {
		if (!ctx) return;
		ctx = eventCtx;
		install(eventCtx);
		refresh();
	});

	for (const event of ["turn_end", "agent_settled", "tool_execution_end", "session_compact", "session_tree"] as const) {
		pi.on(event as "turn_end", (_event, eventCtx) => {
			if (!ctx) return;
			ctx = eventCtx;
			refresh();
		});
	}

	pi.on("thinking_level_select", () => requestRender());

	pi.on("session_shutdown", () => {
		clearTimeout(timer);
		ctx = undefined;
	});
}
