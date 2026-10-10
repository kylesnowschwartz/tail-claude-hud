// Portions adapted from whats-agent-doing (MIT, Copyright (c) 2026 Tzafrir
// Rehan); see NOTICE.

/** Collapses runs of whitespace, newlines included, to single spaces. */
export function oneLine(text: string): string {
  return text.replace(/\s+/g, ' ').trim()
}

/** Cuts `text` to `max` characters, marking the cut with an ellipsis. */
export function clip(text: string, max: number): string {
  return text.length > max ? `${text.slice(0, max - 1)}…` : text
}

/**
 * A label safe to draw: control, format and line-separator characters out
 * (the engine refuses a whole tree holding one), whitespace collapsed, and
 * cut to `max`. Labels carry paths, commands and thoughts the model wrote.
 */
export function printable(text: string, max: number): string {
  const spaced = text.replace(/[\p{Cc}\p{Zl}\p{Zp}]/gu, ' ').replace(/\p{Cf}/gu, '')

  return clip(oneLine(spaced), max)
}

/** A sentence this short has barely started; its predecessor gives context. */
const SHORT_SENTENCE_CHARS = 25

/**
 * The sentence Claude is thinking right now: the tail of the thought so far,
 * with the sentence before it when the current one has barely started.
 *
 * @param thought the thinking block's text so far
 * @param max the longest snippet, cut from the front
 */
export function thoughtOf(thought: string, max: number): string {
  const flat = oneLine(thought.replace(/[*_`#>]/g, ''))

  if (flat === '') {
    return ''
  }

  const sentences = flat.split(/(?<=[.!?:])\s+/)
  const last = sentences.at(-1) ?? ''
  const before = sentences.at(-2)
  const tail = last.length < SHORT_SENTENCE_CHARS && before !== undefined ? `${before} ${last}` : last

  return tail.length > max ? `…${tail.slice(-(max - 1))}` : tail
}

export function plural(count: number, word: string): string {
  return `${count} ${word}${count === 1 ? '' : 's'}`
}

/** `840 B`, `2.3 KB`, `1.1 MB`. */
export function sizeOf(chars: number): string {
  if (chars < 1024) {
    return `${chars} B`
  }

  if (chars < 1024 * 1024) {
    return `${(chars / 1024).toFixed(1)} KB`
  }

  return `${(chars / 1024 / 1024).toFixed(1)} MB`
}

/**
 * A finished duration as tail-claude-hud's tools widget prints it:
 * `<0.1s`, `0.6s`, `12.3s`, `1m 48s`.
 */
export function durationOf(ms: number): string {
  const whole = Math.floor(ms)

  if (whole <= 0) {
    return '0.0s'
  }

  if (whole < 100) {
    return '<0.1s'
  }

  if (whole < 60_000) {
    const seconds = Math.floor(whole / 1000)
    const tenths = Math.floor((whole % 1000) / 100)

    return tenths === 0 ? `${seconds}s` : `${seconds}.${tenths}s`
  }

  const minutes = Math.floor(whole / 60_000)
  const seconds = Math.floor((whole % 60_000) / 1000)

  return `${minutes}m ${seconds}s`
}

/**
 * A running timer, ticked once a second: whole seconds, then minutes and
 * seconds, as the HUD's agents widget counts a running agent.
 */
export function elapsedOf(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000))

  if (total < 60) {
    return `${total}s`
  }

  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)

  return hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m ${total % 60}s`
}

/**
 * Counts the words a streamed chunk adds, carrying whether the text so far
 * ends inside a word, so a reply is never counted again from its start.
 *
 * @param chunk the newly streamed text
 * @param isInWord whether the text before it ended inside a word
 */
export function wordsAdded(chunk: string, isInWord: boolean): { added: number; isInWord: boolean } {
  const words = chunk.match(/\S+/g)?.length ?? 0
  const continues = isInWord && /^\S/.test(chunk)

  return {
    added: continues ? words - 1 : words,
    isInWord: chunk === '' ? isInWord : /\S$/.test(chunk),
  }
}

/**
 * A prompt as the history lists it. A turn the engine started with a tagged
 * message (`<agent-message …>`, `<task-notification>`) is named by its tag,
 * since the markup says nothing to a reader.
 */
export function promptLabel(text: string, max: number): string {
  const flat = oneLine(text)
  const tag = /^<([a-z][a-z-]*)[\s>]/.exec(flat)?.[1]

  return tag === undefined ? clip(flat, max) : capitalized(tag.replace(/-/g, ' '))
}

export function capitalized(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1)
}
