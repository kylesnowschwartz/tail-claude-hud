import { describe, expect, test } from 'claude-code/testing'

import { BUILTIN_STYLE, color, parseStyle, toolIcon } from '../hooks/style'
import { durationOf, elapsedOf, printable, promptLabel } from '../hooks/text'

describe('parseStyle', () => {
  test('reads schema 1 and fills what it lacks from the built-in look', async () => {
    const style = parseStyle(JSON.stringify({
      schema: 1,
      icons: { read: 'R' },
      colors: { running: '#ff8800', thinking: 'red; drop' },
      toolCategories: { byName: { Read: 'Read' }, icons: { Read: 'read' } },
    }))

    expect(style).not.toBeNull()
    expect(toolIcon(style!, 'Read')).toBe('R')
    expect(toolIcon(style!, 'Bash')).toBe(BUILTIN_STYLE.icons['bash'])
    expect(color(style!, 'running')).toBe('#ff8800')
    // A color outside the HUD's syntax could get the tree refused: default fg.
    expect(color(style!, 'thinking')).toBeUndefined()
    expect(style!.agentColors).toEqual(BUILTIN_STYLE.agentColors)
  })

  test('refuses other schemas and non-JSON', async () => {
    expect(parseStyle('{"schema": 2}')).toBeNull()
    expect(parseStyle('nope')).toBeNull()
    expect(parseStyle('[1]')).toBeNull()
  })

  test('cleans control characters out of icons', async () => {
    const style = parseStyle(JSON.stringify({ schema: 1, icons: { check: `\u001b[31m✓` } }))

    expect(style!.icons['check']).toBe('[31m✓')
  })

  test('an unknown tool takes the default category icon', async () => {
    expect(toolIcon(BUILTIN_STYLE, 'mcp__x__y')).toBe(BUILTIN_STYLE.icons['other'])
  })
})

describe('text', () => {
  test('durations read as the HUD tools widget prints them', async () => {
    expect(durationOf(0)).toBe('0.0s')
    expect(durationOf(50)).toBe('<0.1s')
    expect(durationOf(600)).toBe('0.6s')
    expect(durationOf(12_000)).toBe('12s')
    expect(durationOf(12_340)).toBe('12.3s')
    expect(durationOf(108_000)).toBe('1m 48s')
  })

  test('running timers count whole seconds, then minutes', async () => {
    expect(elapsedOf(4_900)).toBe('4s')
    expect(elapsedOf(94_000)).toBe('1m 34s')
    expect(elapsedOf(3_720_000)).toBe('1h 2m')
  })

  test('printable drops control and invisible characters and clips', async () => {
    // A control character reads as a space; an invisible format one is gone.
    expect(printable(`a\u0007b\u202ec\nd`, 80)).toBe('a bc d')
    expect(printable('x'.repeat(10), 5)).toBe('xxxx…')
  })

  test('a tagged prompt the engine sent is named by its tag', async () => {
    expect(promptLabel('<agent-message from="a1">[Subagent hand-back] done', 60)).toBe('Agent message')
    expect(promptLabel('<task-notification>\n<task-id>x</task-id>', 60)).toBe('Task notification')
    expect(promptLabel('fix  the\nconfig', 60)).toBe('fix the config')
  })
})
