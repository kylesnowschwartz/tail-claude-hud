// Adapted from whats-agent-doing (MIT, Copyright (c) 2026 Tzafrir Rehan);
// see NOTICE.
import { clip, oneLine } from './text'

const MAX_DETAIL_CHARS = 60

/** `present` for what a call is doing (the headline), `past` for what it did (the history). */
export type Tense = 'present' | 'past'

/**
 * Says in a few words what a tool call is doing, or did.
 *
 * @param tool the tool's name; a string, as the declared names are only the
 *   tools of the build that wrote the types (Grep and Glob are on some only)
 * @param args the call's arguments, whole or as far as they have streamed
 * @param tense whether the label says what the call is doing or what it did
 * @returns a short label such as `Reading register.tsx` or `Read register.tsx`
 */
export function activityOf(tool: string, args: Record<string, unknown>, tense: Tense = 'present'): string {
  const verb = (present: string, past: string): string => (tense === 'present' ? present : past)

  const raw = (key: string): string => {
    const value = args[key]

    return typeof value === 'string' ? oneLine(value) : ''
  }

  const arg = (key: string): string => clip(raw(key), MAX_DETAIL_CHARS)

  // The base name is taken before the cut, so a long path still names its file.
  const file = (key: string): string => clip(baseName(raw(key)), MAX_DETAIL_CHARS)

  switch (tool) {
    case 'Bash':
    case 'PowerShell':
      // The model's own description says what a command is for; the command
      // itself is the fallback.
      return arg('description') || `${verb('Running', 'Ran')} ${arg('command')}`
    case 'Read':
      return `${verb('Reading', 'Read')} ${file('file_path')}`
    case 'Edit':
      return `${verb('Editing', 'Edited')} ${file('file_path')}`
    case 'Write':
      return `${verb('Writing', 'Wrote')} ${file('file_path')}`
    case 'NotebookEdit':
      return `${verb('Editing', 'Edited')} ${file('notebook_path')}`
    case 'Grep':
      return `${verb('Searching', 'Searched')} for "${arg('pattern')}"`
    case 'Glob':
      return `${verb('Finding', 'Found')} files ${arg('pattern')}`
    case 'WebFetch':
      return `${verb('Fetching', 'Fetched')} ${hostOf(arg('url'))}`
    case 'WebSearch':
      return `${verb('Searching', 'Searched')} the web for "${arg('query')}"`
    case 'Agent':
    case 'Task':
      return `${verb('Running', 'Ran')} an agent: ${arg('description')}`
    case 'TodoWrite':
      return `${verb('Updating', 'Updated')} the todo list`
    case 'AskUserQuestion':
      return verb('Waiting for your answer', 'Asked you a question')
    case 'Skill':
      return `${verb('Using', 'Used')} the ${arg('skill')} skill`
  }

  if (tool.startsWith('mcp__')) {
    const [, server = '', name = ''] = tool.split('__')

    return `${verb('Using', 'Used')} ${server}: ${name}`
  }

  return `${verb('Using', 'Used')} ${tool}`
}

/**
 * The string arguments a tool call's input has streamed so far, read from
 * its partial JSON: enough to name the file or command before it is whole.
 */
export function partialArgsOf(json: string): Record<string, unknown> {
  const args: Record<string, unknown> = {}

  for (const match of json.matchAll(/"([a-z_]+)"\s*:\s*"((?:[^"\\]|\\.)*)/g)) {
    const [, key = '', raw = ''] = match

    args[key] ??= unescaped(raw)
  }

  return args
}

export function isShellTool(tool: string): boolean {
  return tool === 'Bash' || tool === 'PowerShell'
}

export function isAgentTool(tool: string): boolean {
  return tool === 'Agent' || tool === 'Task'
}

function unescaped(raw: string): string {
  try {
    return JSON.parse(`"${raw}"`) as string
  } catch {
    // A partial escape at the stream's edge: the raw text reads well enough.
    return raw
  }
}

function baseName(path: string): string {
  return path.split(/[\\/]/).pop() || path
}

function hostOf(url: string): string {
  try {
    return new URL(url).host
  } catch {
    // Not a whole URL yet (still streaming): show what there is.
    return url
  }
}
