// Pure helpers for the pi extension: the stdin document for the
// tail-claude-hud binary, and the text pi draws around the binary's output.

/** A session entry, reduced to the fields that date a prompt-cache write. */
export type CacheEntry = {
	type: string;
	timestamp?: string;
	kind?: string;
	provider?: string;
	model?: string;
	message?: { role?: string; provider?: string; model?: string; stopReason?: string; timestamp?: number };
};

export type StatusInput = {
	sessionId: string;
	cwd: string;
	model?: {
		id: string;
		name?: string;
		provider: string;
		contextWindow?: number;
		promptCache?: { short?: number; long?: number };
	};
	contextUsage?: { tokens: number | null; contextWindow: number; percent: number | null };
	entries: readonly CacheEntry[];
	/** "long" when pi asks providers for the long cache lifetime. */
	cacheRetention: "short" | "long";
};

/**
 * When the prompt cache of `model` was last written, in Unix milliseconds:
 * the newest answered request or cache refresh that used it. Undefined when
 * the entries hold none.
 */
export function lastCacheWriteAt(
	entries: readonly CacheEntry[],
	model: { id: string; provider: string },
): number | undefined {
	let latest: number | undefined;
	const keep = (at: number | undefined) => {
		if (at !== undefined && Number.isFinite(at) && (latest === undefined || at > latest)) latest = at;
	};
	for (const entry of entries) {
		if (entry.type === "message" && entry.message?.role === "assistant") {
			const { provider, model: id, stopReason, timestamp } = entry.message;
			if (provider !== model.provider || id !== model.id) continue;
			if (stopReason === "error" || stopReason === "aborted") continue;
			keep(timestamp);
		} else if (entry.type === "usage" && entry.kind === "cache_warm") {
			// A refresh is stored under the name the provider answers with, which
			// for an alias is the model id plus a suffix.
			if (entry.provider !== model.provider || !entry.model?.startsWith(model.id)) continue;
			keep(entry.timestamp === undefined ? undefined : Date.parse(entry.timestamp));
		}
	}
	return latest;
}

/** The prompt_cache object for the binary, or undefined when the model declares no cache lifetime or has no cache write. */
export function promptCache(input: StatusInput): { last_request_at: string; ttl_seconds: number } | undefined {
	const { model } = input;
	const ttlSeconds = model?.promptCache?.[input.cacheRetention];
	if (!model || !ttlSeconds || ttlSeconds <= 0) return undefined;
	const writtenAt = lastCacheWriteAt(input.entries, model);
	if (writtenAt === undefined) return undefined;
	return { last_request_at: new Date(writtenAt).toISOString(), ttl_seconds: ttlSeconds };
}

/** The stdin document the tail-claude-hud binary reads, in Claude Code's status-line format. */
export function statusPayload(input: StatusInput): Record<string, unknown> {
	const { model, contextUsage } = input;
	const payload: Record<string, unknown> = {
		session_id: input.sessionId,
		cwd: input.cwd,
		workspace: { current_dir: input.cwd, project_dir: input.cwd },
	};
	if (model) payload.model = { id: model.id, display_name: model.name || model.id };

	const windowSize = contextUsage?.contextWindow ?? model?.contextWindow ?? 0;
	if (windowSize > 0) {
		const tokens = contextUsage?.tokens ?? 0;
		payload.context_window = {
			context_window_size: windowSize,
			used_percentage: contextUsage?.percent ?? 0,
			current_usage: { input_tokens: tokens, cache_creation_input_tokens: 0, cache_read_input_tokens: 0 },
		};
	}

	const cache = promptCache(input);
	if (cache) payload.prompt_cache = cache;
	return payload;
}

/** Milliseconds until the status line next needs a redraw: one second in the cache countdown's last minute, otherwise `idleMs`. */
export function nextRefreshDelay(
	cache: { last_request_at: string; ttl_seconds: number } | undefined,
	now: number,
	idleMs: number,
): number {
	if (!cache) return idleMs;
	const left = Date.parse(cache.last_request_at) + cache.ttl_seconds * 1000 - now;
	return left > 0 && left <= 61_000 ? 1000 : idleMs;
}

const eraseLine = /\x1b\[\d*K/g;

/** The binary's stdout as status-line rows: erase-line codes removed, blank rows dropped. */
export function hudRows(stdout: string): string[] {
	return stdout
		.split("\n")
		.map((row) => row.replace(eraseLine, "").trimEnd())
		.filter((row) => row.replace(/\x1b\[[\d;]*m/g, "").trim() !== "");
}

/** One line of text from another extension's status: control characters become spaces. */
export function statusText(text: string): string {
	return text
		.replace(/[\r\n\t]/g, " ")
		.replace(/ +/g, " ")
		.trim();
}

/**
 * The footer rows: the binary's rows, indented as Claude Code indents its
 * status line, with the other extensions' statuses joined to the first row.
 */
export function footerRows(rows: readonly string[], statuses: readonly string[], separator: string): string[] {
	const texts = statuses.map(statusText).filter((text) => text !== "");
	const [first, ...rest] = rows;
	const firstRow = [...(first === undefined ? [] : [first]), ...texts].join(separator);
	return [firstRow, ...rest].filter((row) => row !== "").map((row) => `  ${row}`);
}

const effortGlyphs: Record<string, string> = {
	off: "◌",
	minimal: "○",
	low: "○",
	medium: "◐",
	high: "●",
	xhigh: "◉",
};

/** The thinking level as Claude Code writes its effort, for example "◐ medium". */
export function effortLabel(level: string): string {
	return `${effortGlyphs[level] ?? "○"} ${level}`;
}
