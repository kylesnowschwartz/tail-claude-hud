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
    ran.push([...e.argv])

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
    onTool: async () => {},
  }

  const clock = mock.clock(on, { now: 1_000_000 })
  seatBand(on)

  const look = async () => {
    box.seen.push(await box.read())
  }

  on('turn.start', ($, e) => ({ turnId: e.turnId }))
  on('turn.complete', ($, e) => ({ text: e.answer }))
  on('classic.PermissionRequest', () => ({}))
  on('classic.PreCompact', () => ({}))
  on('classic.PostCompact', () => ({}))

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

  on('tool.call', async () => {
    await box.onTool()
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
    expect(box.seen[1]).toContain('Reading your prompt')
    expect(box.seen[2]).toContain('Reading app.ts')
    expect(box.seen.join(' ')).not.toContain('Preparing')

    await $.tool.call({ tool: 'Read', file_path: 'src/app.ts' })

    expect(box.seen[3]).toContain('Reading app.ts')
    expect(await box.read()).toContain('Reviewing the results of 1 action')

    await step($, 1)

    expect(box.seen[4]).toContain('Writing the reply · 7 words')

    await $.turn.complete({ ...DONE, turnId })

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
    // Denied: the muted mark, then the Read category's built-in icon.
    expect(history).toContain('⊘ ≡ Reading app.ts')
    expect(history).toContain('Wrote the reply (1 word)')
    expect(history).toContain('Done in')
    expect((await ui.find({ key: 'activity' }))?.text).toContain('▾')

    await ui.press({ key: 'toggle' })

    expect(await ui.find({ key: 'history' })).toBeUndefined()
  })

  test('thinking with no thought text keeps the last action', async ($, on) => {
    const { box } = seat(on, [
      [{ kind: 'thinking', index: 0, text: '' }],
      [{ kind: 'thinking', index: 0, text: '' }],
    ])

    await mount($, box)
    await $.turn.start({ text: 'fix the config', turnId: 't1' })
    await step($, 0)
    await $.tool.call({ tool: 'Read', file_path: 'src/app.ts' })
    await step($, 1)

    expect(box.seen[0]).toContain('Reading your prompt')
    expect(box.seen[2]).toContain('Reading app.ts')
    expect(box.seen.join(' ')).not.toContain('Thinking')
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

    expect(box.seen[1]).toContain('Reading your prompt')
    expect(box.seen[2]).toContain('Run the tests')
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

  test('a call waiting on a permission prompt says so', async ($, on) => {
    const { box } = seat(on, [])

    box.onTool = async () => {
      await $.classic.PermissionRequest({ tool_name: 'Bash', tool_input: { command: 'npm test' } })
    }

    await mount($, box)
    await $.turn.start({ text: 'test it', turnId: 't1' })
    await $.tool.call({ tool: 'Bash', command: 'npm test', description: 'Run the tests' })

    expect(box.seen[0]).toContain('Waiting for your approval: Run the tests')
    // The built-in look's permission icon.
    expect(box.seen[0]).toContain('! Waiting')
  })

  test('a question to the person says it waits for an answer', async ($, on) => {
    const { box } = seat(on, [])

    await mount($, box)
    await $.turn.start({ text: 'ask me', turnId: 't1' })
    await $.tool.call({ tool: 'AskUserQuestion', questions: [] } as Parameters<Engine['tool']['call']>[0])

    expect(box.seen[0]).toContain('Waiting for your answer')
  })

  test('compacting says so and is kept in the history', async ($, on) => {
    const { box } = seat(on, [])
    const ui = await mount($, box)

    await $.classic.PreCompact({ trigger: 'manual', custom_instructions: null })

    expect(await box.read()).toContain('Compacting the conversation')

    await $.classic.PostCompact({ trigger: 'manual', compact_summary: 'summary' })
    await ui.press({ key: 'toggle' })

    expect((await ui.find({ key: 'history' }))?.text ?? '').toContain('Compacted the conversation')
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
    expect(team).toContain('◆ Draft the plan › Reading README.md')
    expect(team).toContain('◆ Storyboard the video › Getting started')

    await $.turn.complete({ ...DONE, turnId: 't2', agentId: 'a2' })

    expect(await box.read()).toContain('Draft the plan › Reading README.md')
    expect(await ui.find({ key: 'agents' })).toBeUndefined()

    await $.turn.complete({ ...DONE, turnId: 't3', agentId: 'a1', reason: 'error' })
    await ui.press({ key: 'toggle' })

    const history = (await ui.find({ key: 'history' }))?.text ?? ''

    expect(await box.read()).toContain('Idle')
    expect(history).toContain('✓ ◆ Agent: Storyboard the video')
    expect(history).toContain('✗ ◆ Agent: Draft the plan')
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
    expect(await box.read()).toContain('reviewer › Reading app.ts')

    box.toolAnswer = { result: { status: 'teammate_spawned' } } as never
    await $.tool.call({
      tool: 'Agent',
      name: 'reviewer',
      description: 'Review the diff',
      prompt: 'You are reviewing the diff. Read every file and…',
    } as Parameters<Engine['tool']['call']>[0])
    await clock.advance(1000)

    expect(await box.read()).toContain('Review the diff › Reading app.ts')
    expect(await box.read()).not.toContain('You are reviewing')
    expect(await ui.find({ key: 'agents' })).toBeUndefined()
  })

  test('a background agent is named from the Agent call that launched it', async ($, on) => {
    const { box, clock } = seat(on, [])

    await mount($, box)

    box.toolAnswer = {
      result: { status: 'async_launched', agentId: 'b1', description: 'This is a long prompt. Read it all.' },
    } as never

    await $.tool.call({ tool: 'Agent', description: 'Tour the source', prompt: 'Read it.' } as Parameters<Engine['tool']['call']>[0])

    expect(await box.read()).toContain('Tour the source › Getting started')

    await $.tool.call({ tool: 'Read', file_path: 'src/app.ts', agentId: 'b1' } as Parameters<Engine['tool']['call']>[0])
    await clock.advance(300)

    expect(await box.read()).toContain('Tour the source › Reading app.ts')
  })
})

describe('style', () => {
  test('the HUD look is asked for once and draws the box', async ($, on) => {
    const ran = seatHud(on, HUD_STYLE)
    const { box } = seat(on, [])
    const ui = await mount($, box)

    const { turnId } = await $.turn.start({ text: 'read it', turnId: 't1' })

    await $.tool.call({ tool: 'Read', file_path: 'src/app.ts' })
    await $.turn.complete({ ...DONE, turnId })
    await $.turn.start({ text: 'again', turnId: 't2' })

    expect(ran).toEqual([['/home/kyle/go/bin/tail-claude-hud', 'style', '--json']])
    expect(box.seen[0]).toContain('RD Reading app.ts')

    await ui.press({ key: 'toggle' })

    const history = (await ui.find({ key: 'history' }))?.text ?? ''

    expect(history).toContain('⊘ RD Reading app.ts')
    expect(history).not.toContain('Built-in look')
  })

  test('the binary option names the HUD to run', { options: { binary: '/opt/tch/tail-claude-hud' } }, async ($, on) => {
    const ran = seatHud(on, HUD_STYLE)
    const { box } = seat(on, [])

    await mount($, box)
    await $.turn.start({ text: 'go', turnId: 't1' })

    expect(ran[0]?.[0]).toBe('/opt/tch/tail-claude-hud')
  })

  test('a running tool is drawn in the HUD running color', async ($, on) => {
    seatHud(on, HUD_STYLE)
    const { box } = seat(on, [])
    let glyphColor: unknown = 'unread'
    const ui = await mount($, box)

    box.onTool = async () => {
      glyphColor = (await ui.find({ type: 'Text', text: ' RD ' }))?.props['color']
    }

    await $.turn.start({ text: 'read it', turnId: 't1' })
    await $.tool.call({ tool: 'Read', file_path: 'src/app.ts' })

    expect(glyphColor).toBe('ansi256(3)')
  })

  test('a HUD that fails leaves the built-in look and a dim note', async ($, on) => {
    seatHud(on, 'not json', 0)
    const { box } = seat(on, [])
    const ui = await mount($, box)

    await $.turn.start({ text: 'read it', turnId: 't1' })
    await $.tool.call({ tool: 'Read', file_path: 'src/app.ts' })

    expect(box.seen[0]).toContain('≡ Reading app.ts')

    await ui.press({ key: 'toggle' })

    expect((await ui.find({ key: 'history' }))?.text ?? '').toContain('Built-in look: /home/kyle/go/bin/tail-claude-hud style --json printed no style this mod reads')
  })
})
