// The glyph and color each state is drawn with: tail-claude-hud's icons and
// semantic colors where the HUD has one, muted text marks where it has none.
import type { ActivityEntry, ActivityNow, ActivityOutcome, TchStyle, TchStyleSource } from '../types'
import { agentColor, color, icon, toolIcon } from './style'
import { printable } from './text'

const MAX_NOTE_CHARS = 120

/** Outcomes the HUD has no icon for: drawn muted, as whats-agent-doing marks them. */
const DENIED_GLYPH = '⊘'
const INTERRUPTED_GLYPH = '■'

/** History marks for rows that are no tool call: a prompt and a compaction. */
const PROMPT_GLYPH = '›'
const COMPACT_GLYPH = '⇣'

/** A glyph and the color it is drawn in (`undefined`: default foreground). */
export type Mark = { glyph: string; color: string | undefined }

/** The headline's glyph: what kind of work, in the HUD's color for it. */
export function headlineMark(style: TchStyle, headline: ActivityNow): Mark {
  const running = color(style, 'running')

  switch (headline.phase) {
    case 'idle':
      return headline.lastOutcome === null
        ? { glyph: icon(style, 'clock'), color: color(style, 'muted') }
        : outcomeMark(style, headline.lastOutcome)
    case 'thinking':
      return { glyph: icon(style, 'thinking'), color: color(style, 'thinking') }
    case 'writing':
      return { glyph: icon(style, 'write'), color: running }
    case 'tool':
    case 'composing':
      return headline.tool === null
        ? { glyph: icon(style, 'running'), color: running }
        : { glyph: toolIcon(style, headline.tool), color: running }
    case 'agent':
      return {
        glyph: icon(style, 'task'),
        color: headline.colorIndex === null ? running : agentColor(style, headline.colorIndex),
      }
    case 'approval':
    case 'question':
      return { glyph: icon(style, 'permission'), color: color(style, 'permission') }
    case 'requesting':
    case 'compacting':
      return { glyph: icon(style, 'running'), color: running }
  }
}

/** A finished thing's glyph: the HUD's check and error icons, muted marks for the rest. */
export function outcomeMark(style: TchStyle, outcome: ActivityOutcome): Mark {
  switch (outcome) {
    case 'ok':
      return { glyph: icon(style, 'check'), color: color(style, 'success') }
    case 'error':
      return { glyph: icon(style, 'error'), color: color(style, 'error') }
    case 'denied':
      return { glyph: DENIED_GLYPH, color: color(style, 'muted') }
    case 'interrupted':
      return { glyph: INTERRUPTED_GLYPH, color: color(style, 'muted') }
  }
}

export function entryMark(style: TchStyle, entry: ActivityEntry): Mark {
  switch (entry.kind) {
    case 'turn':
      return { glyph: PROMPT_GLYPH, color: undefined }
    case 'thought':
      return { glyph: icon(style, 'thinking'), color: color(style, 'thinking') }
    case 'reply':
      return { glyph: icon(style, 'write'), color: color(style, 'success') }
    case 'compact':
      return { glyph: COMPACT_GLYPH, color: color(style, 'muted') }
    case 'tool':
    case 'end':
      return outcomeMark(style, entry.outcome)
  }
}

/** The one line saying the HUD's look is not in use, and why. */
export function styleNote(source: TchStyleSource | null): string | null {
  return source?.kind === 'builtin' ? `Built-in look: ${printable(source.reason, MAX_NOTE_CHARS)}` : null
}
