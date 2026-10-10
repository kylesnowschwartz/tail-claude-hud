// The turn, history, approval and agent scenarios are adapted from
// whats-agent-doing's tests (MIT, Copyright (c) 2026 Tzafrir Rehan); see NOTICE.
import type { On, TurnStepChunk } from 'claude-code'
import { describe, expect, mock, test } from 'claude-code/testing'
import type { Engine } from 'claude-code/testing'

const BAND = {
  plugin: 'tch-activity',
  surface: 'terminal',
  component: 'AbovePrompt',
  props: {
    hasSurvey: false,
    isWorking: false,
    maxRows: 20,
    bodyColumns: 100,
    scroll: { offset: 0, bodyRows: 20 },
    view: {},
  },
} as const

const DONE = {
  answer: 'All fixed.',
  durationMs: 1000,
  isAborted: false,
  reason: 'answer',
  category: null,
  explanation: null,
} as const

const SPAWN = {
  prompt: 'Do the work.',
  subagentType: 'general-purpose',
  provider: { plugin: 'engine', tier: 'core' },
  parentModel: 'claude-opus-5-5',
  background: true,
  fork: false,
} as const

/** A conversation to compact: a compaction always leaves at least one message. */
const TRANSCRIPT = [{ role: 'user', text: 'fix the config', toolUses: [] }] as never

const STEP = { turnId: 't1', model: 'claude-opus-5-5', messageCount: 1 } as const

/** Glyphs the HUD's export uses, distinct from the built-in look's. */
const HUD_ICONS = {
  check: 'OK',
  error: 'XX',
  running: 'RU',
  read: 'RD',
  bash: 'SH',
  task: 'TK',
  thinking: 'TH',
  permission: 'PM',
  clock: 'CL',
  write: 'WR',
  other: 'OT',
}

const HUD_STYLE = JSON.stringify({
  schema: 1,
  version: 'test',
  iconMode: 'nerdfont',
  icons: HUD_ICONS,
  colors: { running: 'ansi256(3)', success: 'ansi256(2)', error: 'ansi256(1)', muted: 'ansi256(8)', thinking: 'bogus color' },
  agentColors: ['ansi256(12)', 'ansi256(10)'],
  toolCategories: {
    byName: { Read: 'Read', Bash: 'Bash', Agent: 'Task' },
    default: 'Other',
    icons: { Read: 'read', Bash: 'bash', Task: 'task' },
  },
})

/**
 * Stands for the engine beneath the band, which draws nothing of its own
 * there (the surveys aside).
 */
function seatBand(on: On) {
  on('ui.render', { component: 'AbovePrompt' }, ($, e) => {
    const { Box } = $.ui.resolve(e)

    return <Box key="engine" />
  })
}

/**
 * Stands for a host where `tail-claude-hud style --json` prints `stdout`
 * with `exitCode`, found in `$HOME/go/bin`. `ran` keeps each argv run.
 */
function seatHud(on: On, stdout: string, exitCode = 0) {
  const ran: string[][] = []

  mock.env(on, { HOME: '/home/kyle' })
  on('fs.exists', () => ({ value: true }))
  on('process.run', ($, e) => {
    ran.push([...e.argv, `stdin=${JSON.stringify(e.init?.stdin)}`])

    return {
      value: { exitCode, stdout, stderr: '', isStdoutTruncated: false, isStderrTruncated: false },
    }
  })

  return ran
}

/**
 * Seats the engine beneath the plugin: a clock, turns, a model whose steps
 * yield `script[index]`, and tools that answer `toolAnswer`. `seen` keeps
 * what the box said at each `look()` a step or a tool takes.
 */
function seat(on: On, script: readonly (readonly TurnStepChunk[])[]) {
  const box = {
    read: async () => '',
    seen: [] as string[],
    toolAnswer: { deny: 'stubbed in the test' } as { deny: string },
    onTool: async (_call: { tool_use_id: string }) => {},
    onCompact: async () => {},
    verdict: 'allow' as 'allow' | 'ask' | 'deny',
  }

  const clock = mock.clock(on, { now: 1_000_000 })
  seatBand(on)

  const look = async () => {
    box.seen.push(await box.read())
  }

  on('turn.start', ($, e) => ({ turnId: e.turnId }))
  on('turn.complete', ($, e) => ({ text: e.answer }))
  on('tool.check', () => ({ decision: box.verdict }))
  on('session.compact', async () => {
    await box.onCompact()

    return { messages: TRANSCRIPT }
  })

  on('turn.step', async function* ($, e) {
    for (const chunk of script[e.index] ?? []) {
      yield chunk
      await clock.advance(300)
      await look()
    }

    return {
      turnId: e.turnId,
      index: e.index,
      answer: '',
      toolUses: [],
      stopReason: e.index + 1 < script.length ? 'tool_use' : 'end_turn',
      usage: null,
    } as const
  })

  on('tool.call', async ($, e) => {
    await box.onTool(e)
    await look()

    return box.toolAnswer
  })

  return { box, clock }
}

async function mount($: Engine, box: { read: () => Promise<string> }) {
  const ui = await $.ui.mount(BAND)

  box.read = async () => (await ui.find({ key: 'activity' }))?.text ?? ''

  return ui
}

async function step($: Engine, index: number) {
  for await (const _ of $.turn.step({ ...STEP, index })) {
    // drained: the beneath hook looks at the box between chunks
  }
}

describe('register', () => {
  test('the box says idle before any turn, its triangle closed', async ($, on) => {
    mock.clock(on)
    seatBand(on)

    const ui = await $.ui.mount(BAND)
    const text = (await ui.find({ key: 'activity' }))?.text ?? ''

    expect(text).toContain('Idle')
    expect(text).toContain('▸')
  })

  test('another plugin drawing the band keeps its box, under this one', async ($, on) => {
    mock.clock(on)

    on('ui.render', { component: 'AbovePrompt' }, ($, e) => {
      const { Box, Text } = $.ui.resolve(e)

      return (
        <Box key="beneath">
          <Text>2 tasks for you</Text>
        </Box>
      )
    })

    const ui = await $.ui.mount(BAND)

    expect((await ui.find({ key: 'activity' }))?.text ?? '').toContain('Idle')
    expect((await ui.find({ key: 'beneath' }))?.text ?? '').toContain('2 tasks for you')
  })

  test('a survey holding the band leaves it to the engine', async ($, on) => {
    mock.clock(on)
    seatBand(on)

    const ui = await $.ui.mount({ ...BAND, props: { ...BAND.props, hasSurvey: true } })

    expect(await ui.find({ key: 'activity' })).toBeUndefined()
    expect(await ui.find({ key: 'engine' })).toBeDefined()
  })

  test('a turn reads, thinks, prepares, runs, reviews, writes and ends', async ($, on) => {
    const { box } = seat(on, [
      [
        { kind: 'thinking', index: 0, text: 'The config looks off. I should check the loader first' },
        { kind: 'tool', index: 1, id: 'tu1', name: 'Read' },
        { kind: 'input', index: 1, json: '{"file_path": "src/app.ts"' },
      ],
      [{ kind: 'text', index: 0, text: 'All fixed now, the loader reads it.' }],
    ])

    await mount($, box)

    const { turnId } = await $.turn.start({ text: 'fix the config', turnId: 't1' })

    expect(await box.read()).toContain('Reading your prompt')

    await step($, 0)

    expect(box.seen[0]).toContain('Thinking: I should check the loader first')
    // The call is named once its input says what it does.
    expect(box.seen[1]).toContain('◐ Preparing the next action')
    expect(box.seen[2]).toContain('Reading app.ts')

    await $.tool.call({ tool: 'Read', file_path: 'src/app.ts' })

    expect(box.seen[3]).toContain('Reading app.ts')
    expect(await box.read()).toContain('Reviewing the results of 1 action')

    await step($, 1)

    expect(box.seen[4]).toContain('❝ Writing the reply | 7 words')

    await $.turn.complete({ ...DONE, turnId })

    expect(await box.read()).toContain('Idle · last turn done in')
  })

  test('a turn whose start came before a reload still ends idle', async ($, on) => {
    const { box } = seat(on, [[{ kind: 'text', index: 0, text: 'Done.' }]])

    await mount($, box)
    // No turn.start reaches this module load: the step is the first it sees.
    await step($, 0)
    await $.turn.complete({ ...DONE, turnId: 't1' })

    expect(await box.read()).toContain('Idle · last turn done in')
  })

  test('the triangle opens the history of the turn', async ($, on) => {
    const { box } = seat(on, [
      [{ kind: 'thinking', index: 0, text: 'Look at the file.' }],
      [{ kind: 'text', index: 0, text: 'Done.' }],
    ])

    const ui = await mount($, box)

    const { turnId } = await $.turn.start({ text: 'fix the config', turnId: 't1' })

    await step($, 0)
    await $.tool.call({ tool: 'Read', file_path: 'src/app.ts' })
    await step($, 1)
    await $.turn.complete({ ...DONE, turnId })

    expect(await ui.find({ key: 'history' })).toBeUndefined()

    await ui.press({ key: 'toggle' })

    const history = (await ui.find({ key: 'history' }))?.text ?? ''

    expect(history).toContain('fix the config')
    expect(history).toContain('Thought (4 words)')
    // Denied: the muted mark in place of the Read category's icon.
    expect(history).toContain('⊘ Read app.ts')
    expect(history).not.toContain('≡')
    expect(history).toContain('Wrote the reply (1 word)')
    // The time is said once, in the label: no duration follows the row.
    expect(history).toMatch(/Done in \d+\.\ds, 1 action(?!\s*[<\d])/)
    expect((await ui.find({ key: 'activity' }))?.text).toContain('▾')

    await ui.press({ key: 'toggle' })

    expect(await ui.find({ key: 'history' })).toBeUndefined()
  })

  test('thinking with no thought text says thinking, not the last action', async ($, on) => {
    const { box } = seat(on, [
      [{ kind: 'thinking', index: 0, text: '' }],
      [{ kind: 'thinking', index: 0, text: '' }],
    ])

    await mount($, box)
    await $.turn.start({ text: 'fix the config', turnId: 't1' })
    await step($, 0)
    await $.tool.call({ tool: 'Read', file_path: 'src/app.ts' })
    await step($, 1)

    expect(box.seen[0]).toContain('∴ Thinking')
    expect(box.seen[2]).toContain('∴ Thinking')
    expect(box.seen[2]).not.toContain('app.ts')
  })

  test('a command being written shows its description, never the command', async ($, on) => {
    const { box } = seat(on, [
      [
        { kind: 'tool', index: 0, id: 'tu1', name: 'Bash' },
        { kind: 'input', index: 0, json: '{"command": "npm test --silent"' },
        { kind: 'input', index: 0, json: ', "description": "Run the tests"' },
      ],
    ])

    await mount($, box)
    await $.turn.start({ text: 'test it', turnId: 't1' })
    await step($, 0)

    expect(box.seen[1]).toContain('◐ Preparing the next action')
    expect(box.seen[2]).toContain('$ Run the tests')
    expect(box.seen.join(' ')).not.toContain('npm test')
  })

  test('parallel calls show the newest with a count of the rest', async ($, on) => {
    const { box } = seat(on, [])
    let release = () => {}
    const held = new Promise<void>(resolve => {
      release = resolve
    })

    box.onTool = () => held

    await mount($, box)
    await $.turn.start({ text: 'look around', turnId: 't1' })

    const first = $.tool.call({ tool: 'Read', file_path: 'src/a.ts' })
    const second = $.tool.call({ tool: 'Read', file_path: 'src/b.ts' })

    // Both calls are in flight once the box names the second.
    for (let i = 0; i < 20 && !(await box.read()).includes('b.ts'); i++) {
      await Promise.resolve()
    }

    expect(await box.read()).toContain('Reading b.ts (+1 more)')

    release()
    await Promise.all([first, second])
  })

  test('a long path is named by its file', async ($, on) => {
    const { box } = seat(on, [])

    await mount($, box)
    await $.turn.start({ text: 'read it', turnId: 't1' })
    await $.tool.call({ tool: 'Read', file_path: `/Users/someone/${'deep/'.repeat(20)}VERSION` })

    expect(box.seen[0]).toContain('Reading VERSION')
  })

  test('control characters in a label never take the box down', async ($, on) => {
    const { box } = seat(on, [])
    const ui = await mount($, box)
    const esc = String.fromCharCode(27)
    const bel = String.fromCharCode(7)
    const rlo = String.fromCharCode(0x202e)

    await $.turn.start({ text: 'x', turnId: 't1' })
    await $.tool.call({ tool: 'Read', file_path: `src/${esc}]0;title${bel}${esc}[31m${rlo}red.ts` })
    await $.tool.call({ tool: 'Read', file_path: `src/${'x'.repeat(200000)}.ts` })
    await ui.press({ key: 'toggle' })

    const history = (await ui.find({ key: 'history' }))?.text ?? ''

    expect(box.seen[0]).toContain('Reading')
    expect(box.seen[0]).toContain('red.ts')
    expect(box.seen[0]).not.toContain(esc)
    expect(box.seen[0]).not.toContain(rlo)
    expect(box.seen[1]?.length ?? 0).toBeLessThan(400)
    expect(history).toContain('red.ts')
  })

  test('a call put to approval waits, and one refused there is marked denied', async ($, on) => {
    const { box } = seat(on, [])

    box.verdict = 'ask'
    box.toolAnswer = { isError: true, text: 'The user refused this tool use.', result: null } as never
    // The engine decides on the call from inside it, before it runs.
    box.onTool = async call => {
      await $.tool.check({ tool: 'Bash', input: { command: 'osascript' }, tool_use_id: call.tool_use_id })
    }

    const ui = await mount($, box)

    await $.turn.start({ text: 'run it', turnId: 't1' })
    await $.tool.call({ tool: 'Bash', command: 'osascript', description: 'Ask AppleScript' })
    await ui.press({ key: 'toggle' })

    expect(box.seen[0]).toContain('Waiting for your approval: Ask AppleScript')
    // The built-in look's permission icon.
    expect(box.seen[0]).toContain('! Waiting')
    expect((await ui.find({ key: 'history' }))?.text ?? '').toContain('⊘ Ask AppleScript')
  })

  test('a turn whose last call you declined ends declined, the dialog time left out', async ($, on) => {
    const { box, clock } = seat(on, [])

    box.verdict = 'ask'
    box.toolAnswer = { isError: true, text: 'The user refused this tool use.', result: null } as never
    box.onTool = async call => {
      await $.tool.check({ tool: 'Bash', input: { command: 'osascript' }, tool_use_id: call.tool_use_id })
      await clock.advance(4000)
    }

    const ui = await mount($, box)
    const { turnId } = await $.turn.start({ text: 'run it', turnId: 't1' })

    await $.tool.call({ tool: 'Bash', command: 'osascript', description: 'Ask AppleScript' })
    await $.turn.complete({ ...DONE, turnId })
    await ui.press({ key: 'toggle' })

    const history = (await ui.find({ key: 'history' }))?.text ?? ''

    expect(await box.read()).toContain('⊘ Idle · last turn stopped at a declined call after 4.0s')
    expect(history).toContain('⊘ Ask AppleScript 0.0s')
    expect(history).toMatch(/⊘ Stopped at a declined call after 4\.0s(?!\s*[<\d])/)
    expect(history).not.toContain('Done')
  })

  test('a turn that goes on after a declined call ends done', async ($, on) => {
    const { box } = seat(on, [[], [{ kind: 'text', index: 0, text: 'I will not run it, then.' }]])

    box.toolAnswer = { deny: 'auto mode said no' }

    const ui = await mount($, box)
    const { turnId } = await $.turn.start({ text: 'run it', turnId: 't1' })

    await step($, 0)
    await $.tool.call({ tool: 'Bash', command: 'rm -rf build', description: 'Clean the build' })
    await step($, 1)
    await $.turn.complete({ ...DONE, turnId })
    await ui.press({ key: 'toggle' })

    expect((await ui.find({ key: 'history' }))?.text ?? '').toMatch(/Done in [\d.]+s, 1 action(?!\s*[<\d])/)
  })

  test('an approved call times only its run, not the dialog', async ($, on) => {
    const { box, clock } = seat(on, [])

    on('ui.render', { component: 'ToolUse' }, ($, e) => {
      const { Box } = $.ui.resolve(e)

      return <Box key="row" />
    })

    box.verdict = 'ask'
    box.toolAnswer = { text: 'ok', result: null } as never
    box.onTool = async call => {
      await $.tool.check({ tool: 'Bash', input: {}, tool_use_id: call.tool_use_id })
      await clock.advance(5000)
      box.seen.push(await box.read())

      await $.ui.mount({
        plugin: 'tch-activity',
        surface: 'terminal',
        component: 'ToolUse',
        props: { tool_use_id: call.tool_use_id, tool: 'Bash', input: {}, isRunning: true, isErrored: false, isInterrupted: false },
      })
      await clock.advance(1)
      box.seen.push(await box.read())
      await clock.advance(2000)
    }

    const ui = await mount($, box)

    await $.turn.start({ text: 'run it', turnId: 't1' })
    await $.tool.call({ tool: 'Bash', command: 'osascript', description: 'Wait in AppleScript' })
    await ui.press({ key: 'toggle' })

    // The dialog's own wait shows while it is up.
    expect(box.seen[0]).toContain('Waiting for your approval: Wait in AppleScript | 5.0s')
    expect(box.seen[1]).toContain('Wait in AppleScript | 0.0s')
    expect((await ui.find({ key: 'history' }))?.text ?? '').toContain('$ Wait in AppleScript 2s')
  })

  test('agent rows start under the headline label', async ($, on) => {
    const { box } = seat(on, [])

    on('agent.spawn', ($, e) => ({ model: 'claude-opus-5-5', agentId: `agent-${e.tool_use_id}` }))

    const ui = await mount($, box)

    await $.agent.spawn({ ...SPAWN, tool_use_id: 'tu1', description: 'First' })
    await $.agent.spawn({ ...SPAWN, tool_use_id: 'tu2', description: 'Second' })

    expect((await ui.find({ key: 'agents' }))?.props['paddingLeft']).toBe(4)
  })

  test('the box grows to its widest row and never shrinks', async ($, on) => {
    const { box, clock } = seat(on, [])

    box.toolAnswer = { text: 'ok', result: null } as never

    const ui = await mount($, box)
    const widthOf = async () => (await ui.find({ key: 'frame' }))?.props['minWidth'] as number
    const idle = await widthOf()

    const { turnId } = await $.turn.start({ text: 'read it', turnId: 't1' })

    await $.tool.call({ tool: 'Read', file_path: `src/${'long-name-'.repeat(8)}.ts` })
    await $.turn.complete({ ...DONE, turnId })
    await ui.press({ key: 'toggle' })
    await clock.advance(1)

    const open = await widthOf()

    await ui.press({ key: 'toggle' })
    await clock.advance(1)

    expect(open).toBeGreaterThan(idle)
    expect(await widthOf()).toBe(open)
  })

  test('an approved call stops waiting once it is drawn running', async ($, on) => {
    const { box, clock } = seat(on, [])

    on('ui.render', { component: 'ToolUse' }, ($, e) => {
      const { Box } = $.ui.resolve(e)

      return <Box key="row" />
    })

    await mount($, box)

    box.verdict = 'ask'
    box.onTool = async call => {
      await $.tool.check({ tool: 'WebFetch', input: {}, tool_use_id: call.tool_use_id })
      box.seen.push(await box.read())

      // The person approves: the engine draws the call's row running.
      await $.ui.mount({
        plugin: 'tch-activity',
        surface: 'terminal',
        component: 'ToolUse',
        props: { tool_use_id: call.tool_use_id, tool: 'WebFetch', input: {}, isRunning: true, isErrored: false, isInterrupted: false },
      })
      await clock.advance(1)
    }

    await $.turn.start({ text: 'fetch it', turnId: 't1' })
    await $.tool.call({ tool: 'WebFetch', url: 'https://example.org', prompt: 'title' } as Parameters<Engine['tool']['call']>[0])

    expect(box.seen[0]).toContain('Waiting for your approval: Fetching example.org')
    expect(box.seen[1]).toContain('Fetching example.org')
    expect(box.seen[1]).not.toContain('Waiting')
  })

  test("a subagent's approval wait is said once and ends with its call", async ($, on) => {
    const { box, clock } = seat(on, [])
    const call = { tool: 'Bash', command: 'npm test', description: 'Run the tests', agentId: 'a1' }

    on('agent.spawn', () => ({ model: 'claude-opus-5-5', agentId: 'a1' }))
    box.verdict = 'ask'
    box.onTool = async ({ tool_use_id }) => {
      // Asked twice: the label still says it once.
      await $.tool.check({ tool: 'Bash', input: {}, tool_use_id, agentId: 'a1' })
      await $.tool.check({ tool: 'Bash', input: {}, tool_use_id, agentId: 'a1' })
      await clock.advance(300)
    }

    await mount($, box)
    await $.agent.spawn({ ...SPAWN, tool_use_id: 'tu1', description: 'Test it' })
    await $.tool.call(call as Parameters<Engine['tool']['call']>[0])

    expect(box.seen[0]).toContain('Test it | Waiting for your approval: Run the tests')
    expect(box.seen[0]).not.toContain('approval: Waiting')

    // A subagent's updates land at most every 250 ms.
    await clock.advance(300)

    expect(await box.read()).toContain('Test it | Run the tests')
    expect(await box.read()).not.toContain('Waiting')
  })

  test('a question to the person says it waits for an answer, under the bell there and in history', async ($, on) => {
    const { box } = seat(on, [])

    box.toolAnswer = { text: 'Blue.', result: null } as never

    const ui = await mount($, box)

    await $.turn.start({ text: 'ask me', turnId: 't1' })
    await $.tool.call({ tool: 'AskUserQuestion', questions: [] } as Parameters<Engine['tool']['call']>[0])
    await ui.press({ key: 'toggle' })

    expect(box.seen[0]).toContain('! Waiting for your answer')
    expect((await ui.find({ key: 'history' }))?.text ?? '').toContain('! Asked you a question')
  })

  test('compacting says so, times itself and is kept in the history', async ($, on) => {
    const { box, clock } = seat(on, [])
    const ui = await mount($, box)

    box.onCompact = async () => {
      await clock.advance(5000)
      box.seen.push(await box.read())
    }

    await $.session.compact({ trigger: 'manual', messages: TRANSCRIPT })

    expect(box.seen[0]).toContain('Compacting the conversation | 5.0s')

    await ui.press({ key: 'toggle' })

    expect((await ui.find({ key: 'history' }))?.text ?? '').toContain('Compacted the conversation 5s')
  })

  test('a compaction inside a turn restarts the timer of what follows', async ($, on) => {
    const { box, clock } = seat(on, [])

    await mount($, box)
    await $.turn.start({ text: 'go', turnId: 't1' })
    box.onCompact = async () => {
      await clock.advance(7000)
    }

    await $.session.compact({ trigger: 'auto', messages: TRANSCRIPT })
    await clock.advance(2000)

    expect(await box.read()).toContain('| 2.0s')
  })

  test('a compaction made ahead of time is not shown', async ($, on) => {
    const { box } = seat(on, [])

    box.onCompact = async () => {
      box.seen.push(await box.read())
    }

    await mount($, box)
    await $.session.compact({ trigger: 'precompute', messages: TRANSCRIPT })

    expect(box.seen[0]).toContain('Idle')
  })

  test('agents still at work after the turn keep the box busy', async ($, on) => {
    const { box, clock } = seat(on, [])

    on('agent.spawn', ($, e) => ({ model: 'claude-opus-5-5', agentId: e.tool_use_id === 'tu1' ? 'a1' : 'a2' }))

    const ui = await mount($, box)
    const { turnId } = await $.turn.start({ text: 'launch the panel', turnId: 't1' })

    await $.agent.spawn({ ...SPAWN, tool_use_id: 'tu1', description: 'Draft the plan' })
    await $.agent.spawn({ ...SPAWN, tool_use_id: 'tu2', description: 'Storyboard the video' })
    await $.turn.complete({ ...DONE, turnId })
    // A call made inside a subagent's loop, which carries its id.
    await $.tool.call({ tool: 'Read', file_path: 'README.md', agentId: 'a1' } as Parameters<Engine['tool']['call']>[0])
    await clock.advance(300)

    const team = (await ui.find({ key: 'agents' }))?.text ?? ''

    expect(await box.read()).toContain('2 agents working')
    expect(team).toContain('◆ Draft the plan | Reading README.md')
    expect(team).toContain('◆ Storyboard the video | Getting started')

    await $.turn.complete({ ...DONE, turnId: 't2', agentId: 'a2' })

    expect(await box.read()).toContain('Draft the plan | Reading README.md')
    expect(await ui.find({ key: 'agents' })).toBeUndefined()

    await $.turn.complete({ ...DONE, turnId: 't3', agentId: 'a1', reason: 'error' })
    await ui.press({ key: 'toggle' })

    const history = (await ui.find({ key: 'history' }))?.text ?? ''

    expect(await box.read()).toContain('Idle')
    expect(history).toContain('◆ Ran an agent: Storyboard the video')
    expect(history).toContain('◆ Ran an agent: Draft the plan')

    // The robot takes the outcome's color: success, then error.
    const robots = await ui.findAll({ type: 'Text', text: '◆ ' })

    expect(robots.map(robot => robot.props['color'])).toEqual(['ansi256(2)', 'ansi256(1)'])
  })

  test('agents take the palette in spawn order', async ($, on) => {
    const { box } = seat(on, [])

    on('agent.spawn', ($, e) => ({ model: 'claude-opus-5-5', agentId: `agent-${e.tool_use_id}` }))

    const ui = await mount($, box)

    await $.agent.spawn({ ...SPAWN, tool_use_id: 'tu1', description: 'First' })
    await $.agent.spawn({ ...SPAWN, tool_use_id: 'tu2', description: 'Second' })

    const first = await ui.find({ type: 'Text', text: 'First' })
    const second = await ui.find({ type: 'Text', text: 'Second' })

    expect(first?.props['color']).toBe('ansi256(12)')
    expect(second?.props['color']).toBe('ansi256(10)')
  })

  test('agent colors follow the order the conversation made the Agent calls', async ($, on) => {
    const { box } = seat(on, [
      [
        { kind: 'tool', index: 0, id: 'tuA', name: 'Agent' },
        { kind: 'tool', index: 1, id: 'tuB', name: 'Agent' },
      ],
    ])

    on('agent.spawn', ($, e) => ({ model: 'claude-opus-5-5', agentId: `agent-${e.tool_use_id}` }))

    const ui = await mount($, box)

    await $.turn.start({ text: 'two agents', turnId: 't1' })
    await step($, 0)
    // The second call's agent starts first; the order still follows the calls.
    await $.agent.spawn({ ...SPAWN, tool_use_id: 'tuB', description: 'Second' })
    await $.agent.spawn({ ...SPAWN, tool_use_id: 'tuA', description: 'First' })

    expect((await ui.find({ type: 'Text', text: 'First' }))?.props['color']).toBe('ansi256(12)')
    expect((await ui.find({ type: 'Text', text: 'Second' }))?.props['color']).toBe('ansi256(10)')
  })

  test("the headline draws an agent's name in its palette color, as the HUD does", async ($, on) => {
    const { box } = seat(on, [])

    on('agent.spawn', () => ({ model: 'claude-opus-5-5', agentId: 'a1' }))

    const ui = await mount($, box)

    await $.agent.spawn({ ...SPAWN, tool_use_id: 'tu1', description: 'Draft the plan' })

    expect(await box.read()).toContain('◆ Draft the plan | Getting started')
    expect((await ui.find({ type: 'Text', text: 'Draft the plan' }))?.props['color']).toBe('ansi256(12)')
    expect((await ui.find({ type: 'Text', text: ' ◆ ' }))?.props['color']).toBe('ansi256(12)')
  })

  test('a named agent is shown by its Agent call description, never its prompt', async ($, on) => {
    const { box, clock } = seat(on, [])

    on('agent.list', () => ({
      value: [{
        id: 'areviewer-1f2e',
        description: 'You are reviewing the diff. Read every file and',
        name: 'reviewer',
        type: 'general-purpose',
        status: 'running',
      }],
    }))

    const ui = await mount($, box)

    await $.tool.call({ tool: 'Read', file_path: 'src/app.ts', agentId: 'areviewer-1f2e' } as Parameters<Engine['tool']['call']>[0])
    await clock.advance(1000)

    // Before its Agent call is seen, the engine's handle for it stands.
    expect(await box.read()).toContain('reviewer | Reading app.ts')

    box.toolAnswer = { result: { status: 'teammate_spawned' } } as never
    await $.tool.call({
      tool: 'Agent',
      name: 'reviewer',
      description: 'Review the diff',
      prompt: 'You are reviewing the diff. Read every file and…',
    } as Parameters<Engine['tool']['call']>[0])
    await clock.advance(1000)

    expect(await box.read()).toContain('Review the diff | Reading app.ts')
    expect(await box.read()).not.toContain('You are reviewing')
    expect(await ui.find({ key: 'agents' })).toBeUndefined()
  })

  test('a background agent is named from the Agent call that launched it', async ($, on) => {
    const { box, clock } = seat(on, [])

    const ui = await mount($, box)

    box.toolAnswer = {
      result: { status: 'async_launched', agentId: 'b1', description: 'This is a long prompt. Read it all.' },
    } as never

    await $.tool.call({ tool: 'Agent', description: 'Tour the source', prompt: 'Read it.' } as Parameters<Engine['tool']['call']>[0])

    expect(await box.read()).toContain('Tour the source | Getting started')

    await $.tool.call({ tool: 'Read', file_path: 'src/app.ts', agentId: 'b1' } as Parameters<Engine['tool']['call']>[0])
    await clock.advance(300)

    expect(await box.read()).toContain('Tour the source | Reading app.ts')

    await ui.press({ key: 'toggle' })

    expect((await ui.find({ key: 'history' }))?.text ?? '').toContain('◆ Started an agent: Tour the source')
  })

  test('the timer shows tenths under ten seconds, whole seconds after', async ($, on) => {
    const { box, clock } = seat(on, [])

    await mount($, box)
    await $.turn.start({ text: 'go', turnId: 't1' })
    await clock.advance(4200)

    expect(await box.read()).toContain('Reading your prompt | 4.2s')

    await clock.advance(8000)

    expect(await box.read()).toContain('Reading your prompt | 12s')
  })

  test('history tool rows are in the past tense', async ($, on) => {
    const { box } = seat(on, [])

    box.toolAnswer = { text: 'ok', result: null } as never

    const ui = await mount($, box)

    await $.turn.start({ text: 'go', turnId: 't1' })
    await $.tool.call({ tool: 'Edit', file_path: 'src/a.ts', old_string: 'a', new_string: 'b' } as Parameters<Engine['tool']['call']>[0])
    await $.tool.call({ tool: 'WebFetch', url: 'https://example.org/x', prompt: 'title' } as Parameters<Engine['tool']['call']>[0])
    await ui.press({ key: 'toggle' })

    const history = (await ui.find({ key: 'history' }))?.text ?? ''

    expect(history).toContain('Edited a.ts')
    expect(history).toContain('Fetched example.org')
    expect(history).not.toContain('Editing')
  })
})

/**
 * Stands for a host whose session `s1` runs in `/x/proj` under HOME
 * `/home/kyle`: `writes` keeps each file written, `removed` each `rm` run.
 */
function seatBreadcrumbs(on: On) {
  const writes: { path: string; text: string }[] = []
  const removed: string[][] = []

  mock.env(on, { HOME: '/home/kyle' })
  on('session.id', () => ({ value: 's1' }))
  on('session.cwd', () => ({ value: '/x/proj' }))
  on('fs.write', ($, e) => {
    writes.push({ path: e.path, text: e.text })

    return { value: undefined }
  })
  on('process.run', ($, e) => {
    removed.push([...e.argv])

    return { value: { exitCode: 0, stdout: '', stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }
  })

  return { writes, removed }
}

const CRUMB = '/home/kyle/.config/tail-claude-hud/waiting/s1'

/** Draws a call's row running, as the engine does once the person approves it. */
async function drawRunning($: Engine, toolUseId: string) {
  await $.ui.mount({
    plugin: 'tch-activity',
    surface: 'terminal',
    component: 'ToolUse',
    props: { tool_use_id: toolUseId, tool: 'Bash', input: {}, isRunning: true, isErrored: false, isInterrupted: false },
  })
}

describe('approval breadcrumb', () => {
  test("an approval wait writes the HUD's breadcrumb, and approving removes it", async ($, on) => {
    const { writes, removed } = seatBreadcrumbs(on)
    const { box, clock } = seat(on, [])
    const during: string[][][] = []

    on('ui.render', { component: 'ToolUse' }, ($, e) => {
      const { Box } = $.ui.resolve(e)

      return <Box key="row" />
    })

    box.verdict = 'ask'
    box.toolAnswer = { text: 'ok', result: null } as never
    box.onTool = async call => {
      await $.tool.check({ tool: 'Bash', input: {}, tool_use_id: call.tool_use_id })
      during.push(removed.filter(argv => argv[0] === '/bin/rm'))
      await drawRunning($, call.tool_use_id)
      await clock.advance(1)
      during.push(removed.filter(argv => argv[0] === '/bin/rm'))
    }

    await mount($, box)
    await $.turn.start({ text: 'run it', turnId: 't1' })
    await $.tool.call({ tool: 'Bash', command: 'osascript', description: 'Wait in AppleScript' })

    expect(writes.filter(write => write.path === CRUMB)).toEqual([
      { path: CRUMB, text: JSON.stringify({ session_id: 's1', project: 'proj', tool_name: 'Bash' }) },
    ])
    expect(during[0]).toEqual([])
    expect(during[1]).toEqual([['/bin/rm', '-f', CRUMB]])
  })

  test('declining removes the breadcrumb once the call ends', async ($, on) => {
    const { writes, removed } = seatBreadcrumbs(on)
    const { box, clock } = seat(on, [])

    box.verdict = 'ask'
    box.toolAnswer = { isError: true, text: 'The user refused this tool use.', result: null } as never
    box.onTool = async call => {
      await $.tool.check({ tool: 'Bash', input: {}, tool_use_id: call.tool_use_id })
    }

    await mount($, box)
    await $.turn.start({ text: 'run it', turnId: 't1' })
    await $.tool.call({ tool: 'Bash', command: 'osascript', description: 'Wait in AppleScript' })
    await clock.advance(1)

    expect(writes.some(write => write.path === CRUMB)).toBe(true)
    expect(removed.filter(argv => argv[0] === '/bin/rm')).toEqual([['/bin/rm', '-f', CRUMB]])
  })

  test("a subagent's approval wait writes no breadcrumb", async ($, on) => {
    const { writes } = seatBreadcrumbs(on)
    const { box, clock } = seat(on, [])

    on('agent.spawn', () => ({ model: 'claude-opus-5-5', agentId: 'a1' }))
    box.verdict = 'ask'
    box.onTool = async ({ tool_use_id }) => {
      await $.tool.check({ tool: 'Bash', input: {}, tool_use_id, agentId: 'a1' })
    }

    await mount($, box)
    await $.agent.spawn({ ...SPAWN, tool_use_id: 'tu1', description: 'Test it' })
    await $.tool.call({ tool: 'Bash', command: 'npm test', description: 'Run the tests', agentId: 'a1' } as Parameters<Engine['tool']['call']>[0])
    await clock.advance(1)

    expect(writes.some(write => write.path.includes('/waiting/'))).toBe(false)
  })

  test("the session's end removes its breadcrumb, a /clear too", async ($, on) => {
    const { removed } = seatBreadcrumbs(on)
    const clock = mock.clock(on, { now: 1_000_000 })

    seatBand(on)
    on('session.end', ($, e) => ({ sessionId: e.sessionId }))

    await $.ui.mount(BAND)
    await $.session.end({ reason: 'clear', sessionId: 'old-session', resume: {} as never })
    await clock.advance(1)

    expect(removed.filter(argv => argv[0] === '/bin/rm')).toEqual([
      ['/bin/rm', '-f', '/home/kyle/.config/tail-claude-hud/waiting/old-session'],
    ])
  })

  test('a breadcrumb that cannot be written or removed leaves the call alone', async ($, on) => {
    const { box, clock } = seat(on, [])

    mock.env(on, { HOME: '/home/kyle' })
    on('session.id', () => ({ value: 's1' }))
    on('session.cwd', () => ({ value: '/x/proj' }))
    on('fs.write', () => {
      throw new Error('read-only file system')
    })
    on('process.run', () => {
      throw new Error('no rm')
    })

    box.verdict = 'ask'
    box.toolAnswer = { isError: true, text: 'The user refused this tool use.', result: null } as never
    box.onTool = async call => {
      await $.tool.check({ tool: 'Bash', input: {}, tool_use_id: call.tool_use_id })
    }

    const ui = await mount($, box)

    await $.turn.start({ text: 'run it', turnId: 't1' })

    const result = await $.tool.call({ tool: 'Bash', command: 'osascript', description: 'Ask AppleScript' })

    await clock.advance(1)
    await ui.press({ key: 'toggle' })

    expect(result.isError).toBe(true)
    expect((await ui.find({ key: 'history' }))?.text ?? '').toContain('⊘ Ask AppleScript')
  })
})

describe('emphasis', () => {
  test('state color sits on the glyph and the border, never on a label or in bold', async ($, on) => {
    seatHud(on, HUD_STYLE)
    const { box, clock } = seat(on, [])
    const ui = await mount($, box)
    const seen: { border: unknown; label: unknown; bold: boolean }[] = []

    await clock.advance(1)

    box.onTool = async () => {
      const texts = await ui.findAll({ type: 'Text' })

      seen.push({
        border: (await ui.find({ key: 'frame' }))?.props['borderColor'],
        label: (await ui.find({ type: 'Text', text: 'Reading app.ts' }))?.props['color'],
        bold: texts.some(text => text.props['bold'] === true),
      })
    }

    expect((await ui.find({ key: 'frame' }))?.props['borderColor']).toBe('ansi256(8)')

    const { turnId } = await $.turn.start({ text: 'read it', turnId: 't1' })

    await $.tool.call({ tool: 'Read', file_path: 'src/app.ts' })

    expect(seen[0]).toEqual({ border: 'ansi256(3)', label: undefined, bold: false })

    await $.turn.complete({ ...DONE, turnId })
    await ui.press({ key: 'toggle' })

    const texts = await ui.findAll({ type: 'Text' })

    expect(texts.some(text => text.props['bold'] === true)).toBe(false)
    expect((await ui.find({ key: 'frame' }))?.props['borderColor']).toBe('ansi256(8)')
  })

  test('a call put to approval turns the border the needs-you color', async ($, on) => {
    seatHud(on, HUD_STYLE)
    const { box, clock } = seat(on, [])
    const ui = await mount($, box)
    let border: unknown = 'unread'

    await clock.advance(1)

    box.verdict = 'ask'
    box.onTool = async call => {
      await $.tool.check({ tool: 'Bash', input: {}, tool_use_id: call.tool_use_id })
      border = (await ui.find({ key: 'frame' }))?.props['borderColor']
    }

    await $.turn.start({ text: 'run it', turnId: 't1' })
    await $.tool.call({ tool: 'Bash', command: 'osascript', description: 'Ask AppleScript' })

    expect(border).toBe('ansi256(1)')
  })

  test("a background agent waiting on approval turns the border red, its label default", async ($, on) => {
    const { box, clock } = seat(on, [])

    on('agent.spawn', () => ({ model: 'claude-opus-5-5', agentId: 'a1' }))

    const ui = await mount($, box)
    let border: unknown = 'unread'
    let label: unknown = 'unread'

    box.verdict = 'ask'
    box.onTool = async ({ tool_use_id }) => {
      await $.tool.check({ tool: 'Bash', input: {}, tool_use_id, agentId: 'a1' })
      await clock.advance(300)
      border = (await ui.find({ key: 'frame' }))?.props['borderColor']
      label = (await ui.find({ type: 'Text', text: 'Waiting for approval' }))?.props['color']
    }

    await $.agent.spawn({ ...SPAWN, tool_use_id: 'tu1', description: 'Test it' })
    await $.tool.call({ tool: 'Bash', command: 'npm test', description: 'Run the tests', agentId: 'a1' } as Parameters<Engine['tool']['call']>[0])

    expect(border).toBe('ansi256(1)')
    expect(label).toBeUndefined()
  })

  test('separators are faint and a thought, reply and agent icon in history are faint', async ($, on) => {
    const { box } = seat(on, [
      [{ kind: 'thinking', index: 0, text: 'Look.' }],
      [{ kind: 'text', index: 0, text: 'Done here.' }],
    ])

    on('agent.spawn', () => ({ model: 'claude-opus-5-5', agentId: 'a1' }))

    const ui = await mount($, box)
    const { turnId } = await $.turn.start({ text: 'go', turnId: 't1' })

    await step($, 0)
    await $.agent.spawn({ ...SPAWN, tool_use_id: 'tu1', description: 'Helper' })
    await step($, 1)

    const separator = await ui.find({ type: 'Text', text: /^ \| $/ })

    expect(separator?.props['dimColor']).toBe(true)

    await $.turn.complete({ ...DONE, turnId: 't2', agentId: 'a1' })
    await $.turn.complete({ ...DONE, turnId })
    await ui.press({ key: 'toggle' })

    for (const glyph of ['∴ ', '❝ ']) {
      const found = await ui.find({ type: 'Text', text: glyph })

      expect(found?.props['dimColor']).toBe(true)
      expect(found?.props['color']).toBeUndefined()
    }
  })
})

describe('style', () => {
  test('the HUD look is asked for once and draws the box', async ($, on) => {
    const ran = seatHud(on, HUD_STYLE)
    const { box, clock } = seat(on, [])
    const ui = await mount($, box)

    await clock.advance(1)

    const { turnId } = await $.turn.start({ text: 'read it', turnId: 't1' })

    await $.tool.call({ tool: 'Read', file_path: 'src/app.ts' })
    await $.turn.complete({ ...DONE, turnId })
    await $.turn.start({ text: 'again', turnId: 't2' })

    // Once per module load, with an empty stdin so an old binary returns at once.
    expect(ran).toEqual([['/home/kyle/go/bin/tail-claude-hud', 'style', '--json', 'stdin=""']])
    expect(box.seen[0]).toContain('RD Reading app.ts')

    await ui.press({ key: 'toggle' })

    const history = (await ui.find({ key: 'history' }))?.text ?? ''

    expect(history).toContain('⊘ Read app.ts')
    expect(history).not.toContain('Built-in look')
  })

  test('a finished tool row has one glyph, its category icon in the outcome color', async ($, on) => {
    seatHud(on, HUD_STYLE)
    const { box, clock } = seat(on, [])
    const ui = await mount($, box)

    await clock.advance(1)

    const { turnId } = await $.turn.start({ text: 'read it', turnId: 't1' })

    box.toolAnswer = { text: 'ok', result: null } as never
    await $.tool.call({ tool: 'Read', file_path: 'src/ok.ts' })
    box.toolAnswer = { isError: true, text: 'no such file', result: null } as never
    await $.tool.call({ tool: 'Bash', command: 'false', description: 'Fail on purpose' })
    await $.turn.complete({ ...DONE, turnId })
    await ui.press({ key: 'toggle' })

    const history = (await ui.find({ key: 'history' }))?.text ?? ''

    expect(history).toContain('RD Read ok.ts')
    expect(history).toContain('SH Fail on purpose')
    expect(history).not.toContain('OK RD')
    expect((await ui.find({ type: 'Text', text: 'RD ' }))?.props['color']).toBe('ansi256(2)')
    expect((await ui.find({ type: 'Text', text: 'SH ' }))?.props['color']).toBe('ansi256(1)')
  })

  test('the binary option names the HUD to run', { options: { binary: '/opt/tch/tail-claude-hud' } }, async ($, on) => {
    const ran = seatHud(on, HUD_STYLE)
    const { box, clock } = seat(on, [])

    await mount($, box)
    await clock.advance(1)

    expect(ran[0]?.[0]).toBe('/opt/tch/tail-claude-hud')
  })

  test('the preset option is passed to the HUD', { options: { preset: 'powerline' } }, async ($, on) => {
    const ran = seatHud(on, HUD_STYLE)
    const { box, clock } = seat(on, [])

    await mount($, box)
    await clock.advance(1)

    expect(ran[0]?.slice(1, 5)).toEqual(['style', '--json', '--preset', 'powerline'])
  })

  test('the look loads without holding up the event that asked for it', async ($, on) => {
    let release = () => {}
    const held = new Promise<void>(resolve => {
      release = resolve
    })

    mock.env(on, { HOME: '/home/kyle' })
    on('fs.exists', () => ({ value: true }))
    on('process.run', async () => {
      await held

      return { value: { exitCode: 0, stdout: HUD_STYLE, stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }
    })

    const { box, clock } = seat(on, [])

    await mount($, box)
    await $.turn.start({ text: 'go', turnId: 't1' })
    await clock.advance(1)

    // The turn started and the box drew while the HUD had not answered.
    expect(await box.read()).toContain('◐ Reading your prompt')

    release()
    await clock.advance(1)

    for (let i = 0; i < 20 && !(await box.read()).includes('RU'); i++) {
      await Promise.resolve()
    }

    expect(await box.read()).toContain('RU Reading your prompt')
  })

  test('a running tool is drawn in the HUD running color', async ($, on) => {
    seatHud(on, HUD_STYLE)
    const { box, clock } = seat(on, [])
    let glyphColor: unknown = 'unread'
    const ui = await mount($, box)

    await clock.advance(1)

    box.onTool = async () => {
      glyphColor = (await ui.find({ type: 'Text', text: ' RD ' }))?.props['color']
    }

    await $.turn.start({ text: 'read it', turnId: 't1' })
    await $.tool.call({ tool: 'Read', file_path: 'src/app.ts' })

    expect(glyphColor).toBe('ansi256(3)')
  })

  test('a HUD that fails leaves the built-in look and a dim note', async ($, on) => {
    seatHud(on, 'not json', 0)
    const { box, clock } = seat(on, [])
    const ui = await mount($, box)

    await clock.advance(1)

    await $.turn.start({ text: 'read it', turnId: 't1' })
    await $.tool.call({ tool: 'Read', file_path: 'src/app.ts' })

    expect(box.seen[0]).toContain('≡ Reading app.ts')

    await ui.press({ key: 'toggle' })

    expect((await ui.find({ key: 'history' }))?.text ?? '').toContain('Built-in look: /home/kyle/go/bin/tail-claude-hud style --json printed no style this mod reads')
  })
})
