import assert from "node:assert/strict";
import { test } from "node:test";
import {
	effortLabel,
	footerRows,
	hudRows,
	lastCacheWriteAt,
	nextRefreshDelay,
	statusPayload,
	type CacheEntry,
	type StatusInput,
} from "./payload.ts";

const model = { id: "claude-fable-5-2", name: "Claude Fable 5.2", provider: "anthropic", contextWindow: 500_000 };
const at = Date.parse("2026-10-10T09:00:00Z");

const assistant = (timestamp: number, extra: Record<string, unknown> = {}): CacheEntry => ({
	type: "message",
	message: { role: "assistant", provider: "anthropic", model: "claude-fable-5-2", stopReason: "stop", timestamp, ...extra },
});

const input = (extra: Partial<StatusInput> = {}): StatusInput => ({
	sessionId: "s1",
	cwd: "/work/project",
	model,
	contextUsage: { tokens: 210_000, contextWindow: 500_000, percent: 42 },
	entries: [],
	cacheRetention: "short",
	...extra,
});

test("the payload carries the model, folder and context use", () => {
	assert.deepEqual(statusPayload(input()), {
		session_id: "s1",
		cwd: "/work/project",
		workspace: { current_dir: "/work/project", project_dir: "/work/project" },
		model: { id: "claude-fable-5-2", display_name: "Claude Fable 5.2" },
		context_window: {
			context_window_size: 500_000,
			used_percentage: 42,
			current_usage: { input_tokens: 210_000, cache_creation_input_tokens: 0, cache_read_input_tokens: 0 },
		},
	});
});

test("the payload has no model or context window before a model is chosen", () => {
	const payload = statusPayload(input({ model: undefined, contextUsage: undefined }));
	assert.equal(payload.model, undefined);
	assert.equal(payload.context_window, undefined);
});

test("unknown context use after a compaction reads as zero", () => {
	const payload = statusPayload(input({ contextUsage: { tokens: null, contextWindow: 500_000, percent: null } }));
	assert.deepEqual(payload.context_window, {
		context_window_size: 500_000,
		used_percentage: 0,
		current_usage: { input_tokens: 0, cache_creation_input_tokens: 0, cache_read_input_tokens: 0 },
	});
});

test("the prompt cache is sent with the lifetime of the active tier", () => {
	const cached = { ...model, promptCache: { short: 300, long: 3600 } };
	const entries = [assistant(at)];
	assert.deepEqual(statusPayload(input({ model: cached, entries })).prompt_cache, {
		last_request_at: "2026-10-10T09:00:00.000Z",
		ttl_seconds: 300,
	});
	assert.deepEqual(statusPayload(input({ model: cached, entries, cacheRetention: "long" })).prompt_cache, {
		last_request_at: "2026-10-10T09:00:00.000Z",
		ttl_seconds: 3600,
	});
});

test("no prompt cache is sent without a declared lifetime or a cache write", () => {
	assert.equal(statusPayload(input({ entries: [assistant(at)] })).prompt_cache, undefined);
	const cached = { ...model, promptCache: { long: 3600 } };
	assert.equal(statusPayload(input({ model: cached, entries: [assistant(at)] })).prompt_cache, undefined);
	assert.equal(statusPayload(input({ model: { ...model, promptCache: { short: 300 } } })).prompt_cache, undefined);
});

test("the last cache write is the newest answered request or refresh of the model", () => {
	const entries: CacheEntry[] = [
		assistant(at),
		{ type: "usage", kind: "cache_warm", provider: "anthropic", model: "claude-fable-5-2", timestamp: "2026-10-10T09:04:00Z" },
		{ type: "usage", kind: "cache_warm", provider: "openai", model: "gpt", timestamp: "2026-10-10T09:30:00Z" },
		{ type: "usage", kind: "other", provider: "anthropic", model: "claude-fable-5-2", timestamp: "2026-10-10T09:40:00Z" },
		assistant(at + 600_000, { stopReason: "error" }),
		assistant(at + 700_000, { stopReason: "aborted" }),
		assistant(at + 800_000, { model: "claude-haiku-5-5" }),
		{ type: "message", message: { role: "user", timestamp: at + 900_000 } },
	];
	assert.equal(lastCacheWriteAt(entries, model), Date.parse("2026-10-10T09:04:00Z"));
	assert.equal(lastCacheWriteAt([], model), undefined);
});

test("the redraw runs each second only in the countdown's last minute", () => {
	const cache = { last_request_at: "2026-10-10T09:00:00.000Z", ttl_seconds: 300 };
	assert.equal(nextRefreshDelay(undefined, at, 15_000), 15_000);
	assert.equal(nextRefreshDelay(cache, at + 60_000, 15_000), 15_000);
	assert.equal(nextRefreshDelay(cache, at + 250_000, 15_000), 1000);
	assert.equal(nextRefreshDelay(cache, at + 301_000, 15_000), 15_000);
});

test("the binary's output becomes rows without erase codes or blank rows", () => {
	const stdout = "\x1b[0m\x1b[38;5;116mFable 5.2\x1b[m | pi-dev\x1b[0m\x1b[K\n\x1b[0m\x1b[0m\x1b[K\n";
	assert.deepEqual(hudRows(stdout), ["\x1b[0m\x1b[38;5;116mFable 5.2\x1b[m | pi-dev\x1b[0m"]);
	assert.deepEqual(hudRows(""), []);
});

test("the footer indents each row and joins the statuses to the first", () => {
	assert.deepEqual(footerRows(["model | 42%", "bell"], ["mcp: 2\nservers", " "], " | "), [
		"  model | 42% | mcp: 2 servers",
		"  bell",
	]);
	assert.deepEqual(footerRows([], ["mcp: 2"], " | "), ["  mcp: 2"]);
	assert.deepEqual(footerRows([], [], " | "), []);
});

test("the thinking level reads as Claude Code's effort", () => {
	assert.equal(effortLabel("medium"), "◐ medium");
	assert.equal(effortLabel("xhigh"), "◉ xhigh");
	assert.equal(effortLabel("off"), "◌ off");
});
