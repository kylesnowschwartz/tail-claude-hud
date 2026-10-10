// The glyph and color each state is drawn with: tail-claude-hud's icons and
// semantic colors where the HUD has one, plain text marks where it has none.
import type { ActivityAgent, ActivityEntry, ActivityNow, ActivityOutcome, TchStyle, TchStyleSource } from '../types'
import { agentColor, color, icon, toolIcon } from './style'
import { printable } from './text'

const MAX_NOTE_CHARS = 120

/** Outcomes the HUD has no icon for, whats-agent-doing's glyphs drawn in the HUD's muted color. */
const DENIED_GLYPH = '⊘'
const INTERRUPTED_GLYPH = '■'

/** History marks for rows that are no tool call: a prompt (default text) and a compaction (faint). */
const PROMPT_GLYPH = '›'
const COMPACT_GLYPH = '⇣'

/**
 * The reply's speech bubble, which the HUD's icon set has no key for: the
 * Nerd Font comment icon (nf-fa-comment), a narrow quote mark otherwise.
 */
const REPLY_GLYPHS: Readonly<Record<string, string>> = { nerdfont: '\uf075', unicode: '❝', ascii: '"' }
const REPLY_FALLBACK = '❝'

/** The tool that puts a question to you, drawn with the needs-you bell wherever it shows. */
const QUESTION_TOOL = 'AskUserQuestion'

/**
 * A glyph, the color it is drawn in (`undefined`: default foreground), and
 * whether it is drawn faint.
 */
export type Mark = { glyph: string; color: string | undefined; isFaint?: boolean }

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
      return { glyph: replyGlyph(style), color: running }
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
      return { glyph: icon(style, 'thinking'), color: undefined, isFaint: true }
    case 'reply':
      return { glyph: replyGlyph(style), color: undefined, isFaint: true }
    case 'compact':
      return { glyph: COMPACT_GLYPH, color: undefined, isFaint: true }
    case 'tool':
      return entry.tool === null ? outcomeMark(style, entry.outcome) : toolMark(style, entry.tool, entry.outcome)
    case 'end':
      return outcomeMark(style, entry.outcome)
  }
}

/**
 * A finished tool's one glyph, as the HUD's tools widget draws it: the
 * category icon in the success or error color. A declined or interrupted
 * call shows its muted mark in the icon's place, so its row never depends
 * on color alone.
 */
function toolMark(style: TchStyle, tool: string, outcome: ActivityOutcome): Mark {
  switch (outcome) {
    case 'ok':
      return { glyph: entryToolIcon(style, tool), color: color(style, 'success') }
    case 'error':
      return { glyph: entryToolIcon(style, tool), color: color(style, 'error') }
    case 'denied':
    case 'interrupted':
      return outcomeMark(style, outcome)
  }
}

/** A history tool row's kind icon: the HUD's for the tool's category, the bell for a question to you. */
function entryToolIcon(style: TchStyle, tool: string): string {
  return tool === QUESTION_TOOL ? icon(style, 'permission') : toolIcon(style, tool)
}

function replyGlyph(style: TchStyle): string {
  return REPLY_GLYPHS[style.iconMode] ?? REPLY_FALLBACK
}

/**
 * The border's color: the needs-you color while the main loop or an agent
 * waits on you, muted when idle, else the color of the work under way.
 */
export function borderColor(style: TchStyle, headline: ActivityNow, agents: readonly ActivityAgent[]): string | undefined {
  if (headline.phase === 'approval' || headline.phase === 'question' || agents.some(agent => agent.isWaiting)) {
    return color(style, 'permission')
  }

  switch (headline.phase) {
    case 'idle':
      return color(style, 'muted')
    case 'thinking':
      return color(style, 'thinking')
    default:
      return color(style, 'running')
  }
}

/** The one line saying the HUD's look is not in use, and why. */
export function styleNote(source: TchStyleSource | null): string | null {
  return source?.kind === 'builtin' ? `Built-in look: ${printable(source.reason, MAX_NOTE_CHARS)}` : null
}
