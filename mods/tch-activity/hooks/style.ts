import type { TchStyle, TchStyleSource } from '../types'
import { printable } from './text'

/** The `style --json` schema this module reads. */
const STYLE_SCHEMA = 1

/** Where `go install` puts the HUD, which the login PATH often lacks. */
export const GO_BIN_SUFFIX = '/go/bin/tail-claude-hud'

/** The bare name, found on the session's PATH when nothing else is. */
export const BARE_BINARY = 'tail-claude-hud'

/** Icons and colors are short; anything longer is not one. */
const MAX_ICON_CHARS = 8

const MAX_REASON_CHARS = 120

/**
 * Mod color syntax the HUD emits. Any other string could make the engine
 * refuse the whole tree, so it is dropped to the default foreground.
 */
const COLOR_PATTERN = /^(ansi256\(\d{1,3}\)|#[0-9a-fA-F]{6})$/

/**
 * The look drawn when the HUD binary is missing or answers badly: the HUD's
 * own default colors with narrow unicode glyphs, which need no Nerd Font.
 */
export const BUILTIN_STYLE: TchStyle = {
  iconMode: 'unicode',
  icons: {
    check: '✓',
    running: '◐',
    clock: '◷',
    read: '≡',
    edit: '✎',
    write: '✎',
    bash: '$',
    grep: '⌕',
    glob: '⌕',
    web: '◎',
    task: '◆',
    skill: '✦',
    thinking: '∴',
    other: '⚙',
    error: '✗',
    permission: '!',
  },
  colors: {
    running: 'ansi256(3)',
    success: 'ansi256(2)',
    error: 'ansi256(1)',
    thinking: 'ansi256(3)',
    permission: 'ansi256(1)',
    muted: 'ansi256(8)',
    normal: 'ansi256(2)',
    warning: 'ansi256(3)',
    critical: 'ansi256(1)',
  },
  agentColors: [
    'ansi256(12)',
    'ansi256(10)',
    'ansi256(9)',
    'ansi256(11)',
    'ansi256(13)',
    'ansi256(14)',
    'ansi256(3)',
    'ansi256(5)',
  ],
  toolCategories: {
    byName: {
      Agent: 'Task',
      Bash: 'Bash',
      Edit: 'Edit',
      Glob: 'Glob',
      Grep: 'Grep',
      NotebookEdit: 'Write',
      Read: 'Read',
      Skill: 'Skill',
      Task: 'Task',
      WebFetch: 'Web',
      WebSearch: 'Web',
      Workflow: 'Task',
      Write: 'Write',
    },
    default: 'Other',
    icons: {
      Bash: 'bash',
      Edit: 'edit',
      Glob: 'glob',
      Grep: 'grep',
      Read: 'read',
      Skill: 'skill',
      Task: 'task',
      Thinking: 'thinking',
      Web: 'web',
      Write: 'write',
    },
  },
}

/** The semantic color names the box draws with. */
export type ColorName = 'running' | 'success' | 'error' | 'thinking' | 'permission' | 'muted'

/** The icon keys the box draws with, beyond the per-category ones. */
export type IconName = 'check' | 'running' | 'clock' | 'task' | 'thinking' | 'error' | 'permission' | 'write' | 'other'

/**
 * The icon for a tool: its HUD category (Read, Bash, Web, ...) mapped to
 * that category's icon key, `other` for tools the HUD does not categorize.
 */
export function toolIcon(style: TchStyle, tool: string): string {
  const category = style.toolCategories.byName[tool] ?? style.toolCategories.default
  const key = style.toolCategories.icons[category] ?? 'other'

  return icon(style, key)
}

export function icon(style: TchStyle, key: IconName | string): string {
  return style.icons[key] || BUILTIN_STYLE.icons[key] || '•'
}

/** A semantic color, or `undefined` for the terminal's default foreground. */
export function color(style: TchStyle, name: ColorName): string | undefined {
  return style.colors[name] || undefined
}

/** The palette color of the agent spawned `colorIndex`-th in the session. */
export function agentColor(style: TchStyle, colorIndex: number): string | undefined {
  const palette = style.agentColors.length > 0 ? style.agentColors : BUILTIN_STYLE.agentColors

  return palette[colorIndex % palette.length] || undefined
}

/** A loaded look and where it came from. */
export type LoadedStyle = { style: TchStyle; source: TchStyleSource }

/**
 * Reads `style --json` output into a look, the built-in one filling any key
 * the output lacks. `null` when it is not JSON or not schema 1.
 */
export function parseStyle(stdout: string): TchStyle | null {
  let raw: unknown

  try {
    raw = JSON.parse(stdout)
  } catch {
    return null
  }

  if (!isRecord(raw) || raw['schema'] !== STYLE_SCHEMA) {
    return null
  }

  const categories = isRecord(raw['toolCategories']) ? raw['toolCategories'] : {}
  const agentColors = Array.isArray(raw['agentColors'])
    ? raw['agentColors'].filter((one): one is string => typeof one === 'string').map(colorOf)
    : []

  return {
    iconMode: typeof raw['iconMode'] === 'string' ? raw['iconMode'] : BUILTIN_STYLE.iconMode,
    icons: { ...BUILTIN_STYLE.icons, ...stringsOf(raw['icons'], iconOf) },
    colors: { ...BUILTIN_STYLE.colors, ...stringsOf(raw['colors'], colorOf) },
    agentColors: agentColors.length > 0 ? agentColors : BUILTIN_STYLE.agentColors,
    toolCategories: {
      byName: { ...BUILTIN_STYLE.toolCategories.byName, ...stringsOf(categories['byName'], plainOf) },
      default: typeof categories['default'] === 'string' ? categories['default'] : BUILTIN_STYLE.toolCategories.default,
      icons: { ...BUILTIN_STYLE.toolCategories.icons, ...stringsOf(categories['icons'], plainOf) },
    },
  }
}

export function builtin(reason: string, binary: string, preset: string): LoadedStyle {
  return {
    style: BUILTIN_STYLE,
    source: { kind: 'builtin', reason: printable(reason, MAX_REASON_CHARS), binary, preset },
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

/** The string entries of `value`, each passed through `clean`; others dropped. */
function stringsOf(value: unknown, clean: (text: string) => string): Record<string, string> {
  if (!isRecord(value)) {
    return {}
  }

  const out: Record<string, string> = {}

  for (const [key, entry] of Object.entries(value)) {
    if (typeof entry === 'string') {
      out[key] = clean(entry)
    }
  }

  return out
}

/** A color the engine accepts, or empty for the default foreground. */
function colorOf(text: string): string {
  return COLOR_PATTERN.test(text) ? text : ''
}

function iconOf(text: string): string {
  return printable(text, MAX_ICON_CHARS)
}

function plainOf(text: string): string {
  return printable(text, MAX_REASON_CHARS)
}

/**
 * Whether to ask the HUD for its look: always, unless the stored look came
 * from the HUD run with this binary and preset. The built-in look is never
 * kept, so a binary that gains `style` later is picked up.
 */
export function isStyleStale(stored: TchStyleSource | null, binary: string, preset: string): boolean {
  return !(stored?.kind === 'hud' && stored.binary === binary && stored.preset === preset)
}
