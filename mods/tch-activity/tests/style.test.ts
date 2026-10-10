import { describe, expect, test } from 'claude-code/testing'

import { bandLayout } from '../hooks/layout'
import { withAgentCall, withAgentKey } from '../hooks/live'
import { BUILTIN_STYLE, color, isStyleStale, parseStyle, toolIcon } from '../hooks/style'
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

describe('isStyleStale', () => {
  const bin = '/home/kyle/go/bin/tail-claude-hud'

  test('a HUD look for the same binary and preset stands', async () => {
    expect(isStyleStale({ kind: 'hud', binary: bin, preset: '' }, bin, '')).toBe(false)
  })

  test('nothing stored, the built-in look, or other options ask again', async () => {
    expect(isStyleStale(null, bin, '')).toBe(true)
    expect(isStyleStale({ kind: 'builtin', reason: 'old', binary: bin, preset: '' }, bin, '')).toBe(true)
    expect(isStyleStale({ kind: 'hud', binary: '/other', preset: '' }, bin, '')).toBe(true)
    expect(isStyleStale({ kind: 'hud', binary: bin, preset: '' }, bin, 'powerline')).toBe(true)
  })
})

describe('bandLayout', () => {
  /** Rows the layout draws: frame and headline, agents, then the history's. */
  const rowsOf = (layout: ReturnType<typeof bandLayout>, entries: number) =>
    3 + layout.agentRows + (layout.hasMoreAgents ? 1 : 0)
    + (layout.isHistoryShown
      ? 1 + (layout.hasEarlier ? 1 : 0) + (entries === 0 ? 1 : layout.historyRows) + (layout.hasNote ? 1 : 0)
      : 0)

  test('every row drawn fits in maxRows', async () => {
    for (const maxRows of [3, 4, 5, 6, 8, 12, 20, 40]) {
      for (const agents of [0, 1, 3, 9]) {
        for (const entries of [0, 1, 5, 30]) {
          for (const hasNote of [false, true]) {
            const layout = bandLayout({ maxRows, agents, entries, isOpen: true, hasNote })

            expect(rowsOf(layout, entries)).toBeLessThanOrEqual(Math.max(3, maxRows))
          }
        }
      }
    }
  })

  test('a long history leads with the earlier row and keeps the newest', async () => {
    const layout = bandLayout({ maxRows: 10, agents: 0, entries: 30, isOpen: true, hasNote: true })

    expect(layout).toEqual({
      agentRows: 0,
      hasMoreAgents: false,
      isHistoryShown: true,
      historyRows: 4,
      hasEarlier: true,
      hasNote: true,
    })
  })

  test('agents past the cap share one more row', async () => {
    const layout = bandLayout({ maxRows: 40, agents: 9, entries: 0, isOpen: false, hasNote: false })

    expect(layout.agentRows).toBe(5)
    expect(layout.hasMoreAgents).toBe(true)
  })
})

describe('agent order', () => {
  test('keeps each key once, the newest ten', async () => {
    let order: readonly string[] = []

    for (let i = 0; i < 12; i++) {
      order = withAgentKey(order, `tu${i}`)
    }

    order = withAgentKey(order, 'tu11')

    expect(order).toEqual(['tu2', 'tu3', 'tu4', 'tu5', 'tu6', 'tu7', 'tu8', 'tu9', 'tu10', 'tu11'])
  })

  test('an agent met by its id moves to its Agent call once known', async () => {
    expect(withAgentCall(['tuA', 'a2', 'tuB'], 'a2', 'tuB')).toEqual(['tuA', 'tuB'])
    expect(withAgentCall(['tuA', 'a2'], 'a2', 'tuC')).toEqual(['tuA', 'tuC'])
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
