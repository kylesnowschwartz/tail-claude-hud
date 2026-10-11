// Adapted from whats-agent-doing's register.tsx (MIT, Copyright (c) 2026
// Tzafrir Rehan); see NOTICE.
import type { Timer, TurnCompleteReason } from 'claude-code'

import type { ActivityEntry, ActivityNow, ActivityOutcome, TurnClock } from '../types'
import { activityOf, isAgentTool, isShellTool, partialArgsOf } from './activity-of'
import { elapsedOf, plural, sizeOf, thoughtOf } from './text'

/** The headline before any turn. */
export const IDLE: ActivityNow = {
  phase: 'idle',
  label: 'Idle, waiting for your prompt',
  detail: null,
  sinceMs: 0,
  tool: null,
  colorIndex: null,
  lead: null,
  lastOutcome: null,
  turn: null,
}

/** The tool that asks you questions; the main loop waits on your answer while it runs. */
export const QUESTION_TOOL = 'AskUserQuestion'

const MAX_THOUGHT_CHARS = 70

/** A tool input this large is worth a size beside its label. */
const LARGE_INPUT_CHARS = 1024

export const STARTING_LABEL = 'Getting started'

/** The headline while the model writes a call its input does not name yet. */
export const PREPARING_LABEL = 'Preparing the next action'

export const UNNAMED_AGENT = 'Agent'

/** What the model is doing in the main loop, between and around tool calls. */
export type ModelPhase = 'idle' | 'requesting' | 'thinking' | 'writing' | 'composing' | 'compacting'

/**
 * A tool call in flight: the main loop's, or a subagent's (`agentId`). An
 * `ask` verdict (at `askedMs`) puts it to the mode's decider; it awaits your
 * approval once that turns out to be the dialog, until it is seen running or
 * ends. `isPutToYou` stays set once it was taken for the dialog. `waitedMs`
 * is the approval wait already over, which its timer and duration leave out.
 */
export type Call = {
  tool: string
  label: string
  startMs: number
  agentId: string | null
  isAwaitingApproval: boolean
  isPutToYou: boolean
  askedMs: number | null
  waitedMs: number
}

/** A call that has just started, not put to approval. */
export function callOf(tool: string, label: string, startMs: number, agentId: string | null): Call {
  return { tool, label, startMs, agentId, isAwaitingApproval: false, isPutToYou: false, askedMs: null, waitedMs: 0 }
}

/**
 * Forgets the main loop's calls, as a turn's start or end does. A background
 * agent's calls outlive the main turn and stay, an approval wait included.
 */
export function dropMainCalls(live: Live): void {
  for (const [id, call] of live.calls) {
    if (call.agentId === null) {
      live.calls.delete(id)
    }
  }
}

/** Ends a call's approval wait at `nowMs`, adding it to the time the call leaves out. */
export function endWait(call: Call, nowMs: number): void {
  if (call.askedMs !== null) {
    call.waitedMs += Math.max(0, nowMs - call.askedMs)
    call.askedMs = null
  }
}

/**
 * The agents tail-claude-hud's agents widget colors: Agent calls in the order
 * the main conversation made them, the newest AGENT_ORDER_WINDOW kept, each
 * taking the palette slot of its position. An agent met with no Agent call
 * seen joins the list by its id.
 */
export const AGENT_ORDER_WINDOW = 10

/** The HUD's agent palette has eight colors; a position wraps at that. */
const AGENT_PALETTE_SIZE = 8

/**
 * A subagent at work, from its spawn (or its first call, when the spawn came
 * before a reload) to its stop. `toolUseId` names the Agent call that started
 * it; `activeMs` is when it last did something; `colorKey` its entry in the
 * agent order, which picks its color.
 */
export type Agent = {
  name: string
  label: string
  startMs: number
  activeMs: number
  toolUseId: string | null
  isBackground: boolean
  colorKey: string
}

/**
 * The module's live view of the session, from which the headline is drawn:
 * started over on a reload, while the headline and history in `$.state` stay.
 * `turnPausedMs` is the time the turn has waited on you in waits now over,
 * `turnPauseStartMs` when the wait still on began, and `turnPauseEndMs`
 * when the last one ended.
 */
export type Live = {
  calls: Map<string, Call>
  agents: Map<string, Agent>
  agentNames: Map<string, string>
  described: Map<string, string>
  agentOrder: readonly string[]
  isRestored: boolean
  phase: ModelPhase
  phaseStartMs: number
  turnStartMs: number | null
  turnPausedMs: number
  turnPauseStartMs: number | null
  turnPauseEndMs: number
  turnActions: number
  isDeclinedLast: boolean
  isFirstRequest: boolean
  resultsToReview: number
  thought: string
  thoughtWords: number
  isThoughtInWord: boolean
  replyWords: number
  isReplyInWord: boolean
  composing: { tool: string; head: string; chars: number } | null
  lastAction: string | null
  lastTurn: { label: string; outcome: ActivityOutcome } | null
  lastPublishMs: number
  trailing: Timer | null
  isSweeping: boolean
  ticker: Timer | null
}

export function createLive(): Live {
  return {
    calls: new Map(),
    agents: new Map(),
    agentNames: new Map(),
    described: new Map(),
    agentOrder: [],
    isRestored: false,
    phase: 'idle',
    phaseStartMs: 0,
    turnStartMs: null,
    turnPausedMs: 0,
    turnPauseStartMs: null,
    turnPauseEndMs: 0,
    turnActions: 0,
    isDeclinedLast: false,
    isFirstRequest: false,
    resultsToReview: 0,
    thought: '',
    thoughtWords: 0,
    isThoughtInWord: false,
    replyWords: 0,
    isReplyInWord: false,
    composing: null,
    lastAction: null,
    lastTurn: null,
    lastPublishMs: 0,
    trailing: null,
    isSweeping: false,
    ticker: null,
  }
}

/** Starts the turn's clock at `startMs`, with no wait on you yet. */
export function startTurn(live: Live, startMs: number): void {
  live.turnStartMs = startMs
  live.turnPausedMs = 0
  live.turnPauseStartMs = null
  live.turnPauseEndMs = startMs
}

/** Whether a call holds the main loop on you: its approval dialog is up, or it asks you a question. */
function isOnYou(call: Call): boolean {
  return call.agentId === null && (call.isAwaitingApproval || call.tool === QUESTION_TOOL)
}

/**
 * Pauses the turn's clock from `fromMs`, as a main-loop call starts waiting
 * on you; a pause already on stands, and none reaches back into one over.
 */
export function pauseTurn(live: Live, fromMs: number): void {
  if (live.turnStartMs === null || live.turnPauseStartMs !== null) {
    return
  }

  live.turnPauseStartMs = Math.max(fromMs, live.turnStartMs, live.turnPauseEndMs)
}

/** Resumes the turn's clock at `nowMs`, once no main-loop call in flight waits on you. */
export function resumeTurn(live: Live, nowMs: number): void {
  if (live.turnPauseStartMs === null || [...live.calls.values()].some(isOnYou)) {
    return
  }

  live.turnPausedMs += Math.max(0, nowMs - live.turnPauseStartMs)
  live.turnPauseStartMs = null
  live.turnPauseEndMs = nowMs
}

/** The running turn's clock, or null between turns. */
export function turnClockOf(live: Live): TurnClock | null {
  return live.turnStartMs === null
    ? null
    : { sinceMs: live.turnStartMs + live.turnPausedMs, pausedAtMs: live.turnPauseStartMs }
}

/** The turn's total at `nowMs`: its time so far, less its waits on you. */
export function turnTotalOf(clock: TurnClock, nowMs: number): number {
  return Math.max(0, (clock.pausedAtMs ?? nowMs) - clock.sinceMs)
}

/** The headline as the live view has it. */
export function current(live: Live): ActivityNow {
  const base = { ...IDLE, lastOutcome: live.lastTurn?.outcome ?? null, turn: turnClockOf(live) }

  if (live.phase === 'compacting') {
    return { ...base, phase: 'compacting', label: 'Compacting the conversation', sinceMs: live.phaseStartMs }
  }

  const open = [...live.calls.entries()].filter(([, call]) => call.agentId === null)
  const [newestId, newest] = open.at(-1) ?? []

  if (newestId !== undefined && newest !== undefined) {
    const more = open.length > 1 ? ` (+${open.length - 1} more)` : ''
    const call = { ...base, sinceMs: newest.startMs + newest.waitedMs, tool: newest.tool }

    if (newest.isAwaitingApproval) {
      return { ...call, sinceMs: newest.askedMs ?? call.sinceMs, phase: 'approval', label: `Waiting for your approval: ${newest.label}${more}` }
    }

    if (newest.tool === QUESTION_TOOL) {
      return { ...call, phase: 'question', label: newest.label }
    }

    const inner = isAgentTool(newest.tool) ? agentUnder(live, newestId) : undefined

    return inner === undefined
      ? { ...call, phase: 'tool', label: `${newest.label}${more}` }
      : {
          ...call,
          phase: 'agent',
          lead: newest.label,
          label: `${agentLabel(live, inner)}${more}`,
          colorIndex: colorIndexOf(live, inner.colorKey),
        }
  }

  if (live.phase === 'idle' && live.agents.size > 0) {
    // Between turns, background agents may still be at work.
    const working = [...live.agents.entries()]
    const sinceMs = Math.min(...working.map(([, agent]) => agent.startMs))
    const [busiestId, busiest] = working.reduce((a, b) => (b[1].activeMs > a[1].activeMs ? b : a))
    const isAlone = working.length === 1

    return {
      ...base,
      phase: 'agent',
      lead: isAlone ? busiest.name : null,
      label: isAlone
        ? agentLabel(live, { ...busiest, id: busiestId })
        : `${plural(working.length, 'agent')} working`,
      sinceMs,
      tool: 'Agent',
      colorIndex: isAlone ? colorIndexOf(live, busiest.colorKey) : null,
    }
  }

  const sinceMs = live.phaseStartMs

  // Between calls the turn's last action stands under the phase's glyph, so
  // a run of quick calls reads as one stretch of work.
  switch (live.phase) {
    case 'requesting':
      return {
        ...base,
        phase: 'requesting',
        label: live.isFirstRequest
          ? 'Reading your prompt'
          : live.lastAction ?? (live.resultsToReview > 0
            ? `Reviewing the results of ${plural(live.resultsToReview, 'action')}`
            : 'Waiting for the model'),
        sinceMs,
      }
    case 'thinking': {
      const snippet = thoughtOf(live.thought, MAX_THOUGHT_CHARS)
      const label = snippet ? `Thinking: ${snippet}` : standingLabel(live)

      return { ...base, phase: 'thinking', label, sinceMs }
    }
    case 'writing':
      return {
        ...base,
        phase: 'writing',
        label: 'Writing the reply',
        detail: plural(live.replyWords, 'word'),
        sinceMs,
      }
    case 'composing':
      return { ...base, ...composingHeadline(live), phase: 'composing', sinceMs }
    case 'idle':
      return live.lastTurn === null ? base : { ...base, label: `Idle · last turn ${live.lastTurn.label}` }
  }
}

/**
 * A call being written is named once its input says what it does: a file
 * tool by its path, a command by its description, which streams after the
 * command itself. Until then the turn's last action stands, or, before
 * there is one, the headline says a call is being prepared.
 */
function composingHeadline(live: Live): { label: string; detail: string | null; tool: string | null } {
  const { composing } = live
  const args = composing ? partialArgsOf(composing.head) : {}
  const isDescribed = composing !== null && (isShellTool(composing.tool)
    ? typeof args['description'] === 'string'
    : Object.keys(args).length > 0)

  if (!composing || !isDescribed) {
    return { label: live.lastAction ?? PREPARING_LABEL, detail: null, tool: null }
  }

  const detail = composing.chars >= LARGE_INPUT_CHARS ? sizeOf(composing.chars) : null

  return { label: activityOf(composing.tool, args), detail, tool: composing.tool }
}

/**
 * What the headline keeps saying while the model thinks with no thought text
 * to quote: the turn's last action, or the prompt before there is one.
 */
function standingLabel(live: Live): string {
  return live.lastAction ?? 'Reading your prompt'
}

/** The agent an Agent call started, or the one that last did something. */
function agentUnder(live: Live, toolUseId: string): (Agent & { id: string }) | undefined {
  const working = [...live.agents.entries()].map(([id, agent]) => ({ ...agent, id }))

  return working.find(agent => agent.toolUseId === toolUseId)
    ?? working.reduce<(Agent & { id: string }) | undefined>((a, b) => (a === undefined || b.activeMs > a.activeMs ? b : a), undefined)
}

/**
 * What an agent is doing: waiting on approval for its newest call that
 * waits, else its latest action. Derived on each read, so a second request
 * never stacks a second prefix.
 */
export function agentLabel(live: Live, agent: Agent & { id: string }): string {
  const waiting = waitingCallOf(live, agent.id)

  return waiting === undefined ? agent.label : `Waiting for your approval: ${waiting.label}`
}

/** Whether a call of the agent's waits on approval. */
export function isAgentWaiting(live: Live, agentId: string): boolean {
  return waitingCallOf(live, agentId) !== undefined
}

function waitingCallOf(live: Live, agentId: string): Call | undefined {
  return [...live.calls.values()].reverse().find(call => call.agentId === agentId && call.isAwaitingApproval)
}

/** The order with `key` added at its end, once, the newest window kept. */
export function withAgentKey(order: readonly string[], key: string): readonly string[] {
  return order.includes(key) ? order : [...order, key].slice(-AGENT_ORDER_WINDOW)
}

/**
 * The order once an agent first met by its id is known by its Agent call:
 * the id's entry goes, and the call keeps (or takes) its own place.
 */
export function withAgentCall(order: readonly string[], agentId: string, toolUseId: string): readonly string[] {
  return withAgentKey(order.filter(key => key !== agentId), toolUseId)
}

/** An agent's palette slot: its position in the agent order, wrapped at the palette size. */
export function colorIndexOf(live: Live, key: string): number {
  return Math.max(0, live.agentOrder.indexOf(key)) % AGENT_PALETTE_SIZE
}

/**
 * How a turn ended: the engine's reason, or `declined` when its last call was
 * one you declined and nothing followed it.
 */
export type TurnEnd = TurnCompleteReason | 'declined'

export function turnEndOf(reason: TurnCompleteReason, isDeclinedLast: boolean): TurnEnd {
  return isDeclinedLast && (reason === 'answer' || reason === 'aborted') ? 'declined' : reason
}

/**
 * How a turn ended, with its time counted as the running timer counts it:
 * the idle headline's words, and, capitalized, the turn's last history row.
 */
export function turnEndLabel(end: TurnEnd, durationMs: number, actions: number): string {
  const took = elapsedOf(durationMs)

  switch (end) {
    case 'aborted':
      return `interrupted after ${took}`
    case 'error':
      return `stopped on an error after ${took}`
    case 'refusal':
      return 'declined the request'
    case 'declined':
      return `stopped at a declined call after ${took}`
    case 'answer':
      return `done in ${took}, ${plural(actions, 'action')}`
  }
}

export function outcomeOf(end: TurnEnd): ActivityOutcome {
  switch (end) {
    case 'aborted':
      return 'interrupted'
    case 'error':
      return 'error'
    case 'refusal':
    case 'declined':
      return 'denied'
    case 'answer':
      return 'ok'
  }
}

export function entryOf(kind: ActivityEntry['kind'], label: string, outcome: ActivityOutcome = 'ok'): ActivityEntry {
  return { kind, label, durationMs: null, outcome, tool: null }
}
