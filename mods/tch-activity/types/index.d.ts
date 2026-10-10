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
 * agent palette slot when the headline names an agent; `lastOutcome` how the
 * previous turn ended, for the idle glyph.
 */
export type ActivityNow = {
  phase: ActivityPhase
  label: string
  sinceMs: number
  tool: string | null
  colorIndex: number | null
  lastOutcome: ActivityOutcome | null
}

/**
 * A subagent at work: what it was asked to do, its latest action, when it
 * started, and its palette slot (its spawn order in the session).
 */
export type ActivityAgent = {
  name: string
  label: string
  sinceMs: number
  colorIndex: number
}

/**
 * One row of the expanded box: a prompt that started a turn, a stretch of
 * thinking, a reply, a tool call, a compaction, or how a turn ended. `tool`
 * names the tool of a `tool` row; `colorIndex` the palette slot of an agent's.
 */
export type ActivityEntry = {
  kind: 'turn' | 'thought' | 'reply' | 'tool' | 'compact' | 'end'
  label: string
  durationMs: number | null
  outcome: ActivityOutcome
  tool: string | null
  colorIndex: number | null
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
 * Where the box's look came from: the HUD binary, or the built-in look with
 * the reason the binary could not supply it.
 */
export type TchStyleSource =
  | { kind: 'hud'; binary: string }
  | { kind: 'builtin'; reason: string }

declare module 'claude-code' {
  interface PluginState {
    'tch-activity': {
      now: ActivityNow
      history: readonly ActivityEntry[]
      agents: readonly ActivityAgent[]
      isExpanded: boolean
      style: TchStyle | null
      styleSource: TchStyleSource | null
    }
  }
}
