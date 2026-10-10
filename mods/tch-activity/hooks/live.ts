// Adapted from whats-agent-doing's register.tsx (MIT, Copyright (c) 2026
// Tzafrir Rehan); see NOTICE.
import type { Timer, TurnCompleteReason } from 'claude-code'

import type { ActivityEntry, ActivityNow, ActivityOutcome } from '../types'
import { activityOf, isAgentTool, isShellTool, partialArgsOf } from './activity-of'
import { durationOf, plural, sizeOf, thoughtOf } from './text'

/** The headline before any turn. */
export const IDLE: ActivityNow = {
  phase: 'idle',
  label: 'Idle, waiting for your prompt',
  sinceMs: 0,
  tool: null,
  colorIndex: null,
  lastOutcome: null,
}

const MAX_THOUGHT_CHARS = 70

/** A tool input this large is worth a size beside its label. */
const LARGE_INPUT_CHARS = 1024

export const STARTING_LABEL = 'Getting started'

export const UNNAMED_AGENT = 'Agent'

/** What the model is doing in the main loop, between and around tool calls. */
export type ModelPhase = 'idle' | 'requesting' | 'thinking' | 'writing' | 'composing' | 'compacting'

/** A main-loop tool call in flight. */
export type Call = {
  tool: string
  label: string
  startMs: number
  isAwaitingApproval: boolean
}

/**
 * A subagent at work, from its spawn (or its first call, when the spawn came
 * before a reload) to its stop. `toolUseId` names the Agent call that started
 * it; `activeMs` is when it last did something; `colorIndex` its palette slot.
 */
export type Agent = {
  name: string
  label: string
  startMs: number
  activeMs: number
  toolUseId: string | null
  isBackground: boolean
  colorIndex: number
}

/**
 * The module's live view of the session, from which the headline is drawn:
 * started over on a reload, while the headline and history in `$.state` stay.
 */
export type Live = {
  calls: Map<string, Call>
  agents: Map<string, Agent>
  agentNames: Map<string, string>
  described: Map<string, string>
  nextColorIndex: number
  phase: ModelPhase
  phaseStartMs: number
  turnStartMs: number | null
  turnActions: number
  isFirstRequest: boolean
  resultsToReview: number
  thought: string
  thoughtWords: number
  isThoughtInWord: boolean
  replyWords: number
  isReplyInWord: boolean
  composing: { tool: string; head: string; chars: number } | null
  lastAction: { label: string; tool: string } | null
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
    nextColorIndex: 0,
    phase: 'idle',
    phaseStartMs: 0,
    turnStartMs: null,
    turnActions: 0,
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

/** The headline as the live view has it. */
export function current(live: Live): ActivityNow {
  const base = { ...IDLE, lastOutcome: live.lastTurn?.outcome ?? null }

  if (live.phase === 'compacting') {
    return { ...base, phase: 'compacting', label: 'Compacting the conversation', sinceMs: live.phaseStartMs }
  }

  const open = [...live.calls.entries()]
  const [newestId, newest] = open.at(-1) ?? []

  if (newestId !== undefined && newest !== undefined) {
    const more = open.length > 1 ? ` (+${open.length - 1} more)` : ''
    const call = { ...base, sinceMs: newest.startMs, tool: newest.tool }

    if (newest.isAwaitingApproval) {
      return { ...call, phase: 'approval', label: `Waiting for your approval: ${newest.label}${more}` }
    }

    if (newest.tool === 'AskUserQuestion') {
      return { ...call, phase: 'question', label: newest.label }
    }

    const inner = isAgentTool(newest.tool) ? agentUnder(live, newestId) : undefined

    return inner === undefined
      ? { ...call, phase: 'tool', label: `${newest.label}${more}` }
      : { ...call, phase: 'agent', label: `${newest.label} › ${inner.label}${more}`, colorIndex: inner.colorIndex }
  }

  if (live.phase === 'idle' && live.agents.size > 0) {
    // Between turns, background agents may still be at work.
    const working = [...live.agents.values()]
    const sinceMs = Math.min(...working.map(agent => agent.startMs))
    const busiest = working.reduce((a, b) => (b.activeMs > a.activeMs ? b : a))
    const isAlone = working.length === 1

    return {
      ...base,
      phase: 'agent',
      label: isAlone ? `${busiest.name} › ${busiest.label}` : `${plural(working.length, 'agent')} working`,
      sinceMs,
      tool: 'Agent',
      colorIndex: isAlone ? busiest.colorIndex : null,
    }
  }

  const sinceMs = live.phaseStartMs

  switch (live.phase) {
    case 'requesting':
      return {
        ...base,
        phase: 'requesting',
        label: live.isFirstRequest
          ? 'Reading your prompt'
          : live.resultsToReview > 0
            ? `Reviewing the results of ${plural(live.resultsToReview, 'action')}`
            : 'Waiting for the model',
        sinceMs,
      }
    case 'thinking': {
      // With no thought text to quote, the last action stands, drawn with
      // the thinking icon; before any action, the prompt is what it reads.
      const snippet = thoughtOf(live.thought, MAX_THOUGHT_CHARS)
      const label = snippet ? `Thinking: ${snippet}` : standingLabel(live)

      return { ...base, phase: 'thinking', label, sinceMs }
    }
    case 'writing':
      return {
        ...base,
        phase: 'writing',
        label: `Writing the reply · ${plural(live.replyWords, 'word')}`,
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
 * command itself. Until then the last action stands.
 */
function composingHeadline(live: Live): { label: string; tool: string | null } {
  const { composing } = live
  const args = composing ? partialArgsOf(composing.head) : {}
  const isDescribed = composing !== null && (isShellTool(composing.tool)
    ? typeof args['description'] === 'string'
    : Object.keys(args).length > 0)

  if (!composing || !isDescribed) {
    return { label: standingLabel(live), tool: live.lastAction?.tool ?? null }
  }

  const size = composing.chars >= LARGE_INPUT_CHARS ? ` · ${sizeOf(composing.chars)}` : ''

  return { label: `${activityOf(composing.tool, args)}${size}`, tool: composing.tool }
}

/**
 * What the box keeps saying while Claude thinks or writes a call it cannot
 * name yet: the turn's last action, or the prompt before there is one.
 */
function standingLabel(live: Live): string {
  return live.lastAction?.label ?? 'Reading your prompt'
}

/** The agent an Agent call started, or the one that last did something. */
function agentUnder(live: Live, toolUseId: string): Agent | undefined {
  const working = [...live.agents.values()]

  return working.find(agent => agent.toolUseId === toolUseId)
    ?? working.reduce<Agent | undefined>((a, b) => (a === undefined || b.activeMs > a.activeMs ? b : a), undefined)
}

/** How a turn ended, in the words the idle headline and the history use. */
export function turnEndLabel(reason: TurnCompleteReason, durationMs: number, actions: number): string {
  const took = durationOf(durationMs)

  switch (reason) {
    case 'aborted':
      return `interrupted after ${took}`
    case 'error':
      return `stopped on an error after ${took}`
    case 'refusal':
      return 'declined the request'
    case 'answer':
      return `done in ${took}, ${plural(actions, 'action')}`
  }
}

export function outcomeOf(reason: TurnCompleteReason): ActivityOutcome {
  return reason === 'aborted' ? 'interrupted' : reason === 'error' ? 'error' : reason === 'refusal' ? 'denied' : 'ok'
}

export function entryOf(kind: ActivityEntry['kind'], label: string, outcome: ActivityOutcome = 'ok'): ActivityEntry {
  return { kind, label, durationMs: null, outcome, tool: null, colorIndex: null }
}
