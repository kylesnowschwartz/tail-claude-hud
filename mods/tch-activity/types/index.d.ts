/**
 * What the box says Claude is doing: `idle` between turns, a model phase
 * (`requesting`, `thinking`, `writing`, `composing`), a tool phase (`tool`,
 * `agent`, `approval`, `question`), or `compacting`.
 */
export type ActivityPhase =
  | 'idle'
  | 'requesting'
  | 'thinking'
  | 'writing'
  | 'composing'
  | 'tool'
  | 'agent'
  | 'approval'
  | 'question'
  | 'compacting'

export type ActivityOutcome = 'ok' | 'error' | 'denied' | 'interrupted'

/**
 * The box's headline: the phase, its words, and when it began. `tool` is the
 * tool the headline is about (its category picks the icon); `colorIndex` the
 * agent palette slot when the headline names an agent, and `lead` the agent's
 * name, drawn in that color before `label`; `detail` a count or size drawn
 * faint after it (`120 words`); `lastOutcome` how the previous turn ended,
 * for the idle glyph; `turn` the running turn's clock, null between turns.
 */
export type ActivityNow = {
  phase: ActivityPhase
  label: string
  detail: string | null
  sinceMs: number
  tool: string | null
  colorIndex: number | null
  lead: string | null
  lastOutcome: ActivityOutcome | null
  turn: TurnClock | null
}

/**
 * A running turn's clock, which leaves out the time the main loop waits on
 * you: the total runs from `sinceMs` (the turn's start, moved on by the
 * waits already over) to now, or to `pausedAtMs` while a wait is on.
 */
export type TurnClock = {
  sinceMs: number
  pausedAtMs: number | null
}

/**
 * A subagent at work: what it was asked to do, its latest action, when it
 * started, its palette slot (its spawn order in the session), and whether a
 * call of its waits on your approval.
 */
export type ActivityAgent = {
  name: string
  label: string
  sinceMs: number
  colorIndex: number
  isWaiting: boolean
}

/**
 * One row of the expanded box: a prompt that started a turn, a stretch of
 * thinking, a reply, a tool call, a compaction, or how a turn ended. `tool`
 * names the tool of a `tool` row.
 */
export type ActivityEntry = {
  kind: 'turn' | 'thought' | 'reply' | 'tool' | 'compact' | 'end'
  label: string
  durationMs: number | null
  outcome: ActivityOutcome
  tool: string | null
}

/**
 * The look tail-claude-hud draws with, as `tail-claude-hud style --json`
 * prints it (schema 1). Colors are mod color strings (`ansi256(N)`,
 * `#rrggbb`), or empty for the terminal's default foreground.
 */
export type TchStyle = {
  iconMode: string
  icons: Readonly<Record<string, string>>
  colors: Readonly<Record<string, string>>
  agentColors: readonly string[]
  toolCategories: {
    byName: Readonly<Record<string, string>>
    default: string
    icons: Readonly<Record<string, string>>
  }
}

/**
 * Where the box's look came from: the HUD binary run with `preset` (empty
 * for none), or the built-in look with the reason that binary could not
 * supply one.
 */
export type TchStyleSource =
  | { kind: 'hud'; binary: string; preset: string }
  | { kind: 'builtin'; reason: string; binary: string; preset: string }

/**
 * The widest the box has been since the last turn ended (`columns`), and how
 * many turns have ended, each of which set `columns` back to 0.
 */
export type BoxWidth = {
  columns: number
  turnEnds: number
}

declare module 'claude-code' {
  interface PluginState {
    'tch-activity': {
      now: ActivityNow
      history: readonly ActivityEntry[]
      agents: readonly ActivityAgent[]
      isExpanded: boolean
      boxWidth: BoxWidth
      style: TchStyle | null
      styleSource: TchStyleSource | null
      agentOrder: readonly string[]
      lastTurn: { label: string; outcome: ActivityOutcome } | null
    }
  }
}
