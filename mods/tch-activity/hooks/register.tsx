// The tracking hooks are adapted from whats-agent-doing's register.tsx (MIT,
// Copyright (c) 2026 Tzafrir Rehan); see NOTICE. The engine traces `$` and
// state references only within the hooks module's own file, so every function
// that takes `$` and every state atom lives here; the other files are pure.
import { atom, read, update } from 'claude-code'
import type {
  AgentInfo,
  EngineInterface,
  PluginOptions,
  Register,
  RenderInput,
  TurnStepChunk,
} from 'claude-code'

import type { ActivityAgent, ActivityEntry, ActivityOutcome, TchStyle, TchStyleSource } from '../types'
import { activityOf, isAgentTool } from './activity-of'
import {
  agentLabel,
  callOf,
  colorIndexOf,
  createLive,
  current,
  dropMainCalls,
  endWait,
  entryOf,
  IDLE,
  isAgentWaiting,
  outcomeOf,
  STARTING_LABEL,
  turnEndLabel,
  turnEndOf,
  UNNAMED_AGENT,
  type Agent,
  type Call,
  type Live,
  type ModelPhase,
  withAgentCall,
  withAgentKey,
} from './live'
import { AGENT_INDENT_COLUMNS, bandLayout, boxColumnsOf, columnsOf, type Segment } from './layout'
import { borderColor, entryMark, headlineMark, styleNote } from './marks'
import {
  agentColor,
  BARE_BINARY,
  BUILTIN_STYLE,
  builtin,
  GO_BIN_SUFFIX,
  icon,
  isStyleStale,
  parseStyle,
  type LoadedStyle,
} from './style'
import { breadcrumbOf, waitingPathOf } from './waiting'
import { capitalized, durationOf, elapsedOf, oneLine, TENTHS_UNDER_MS, plural, printable, promptLabel, wordsAdded } from './text'

/** The headline. */
const now = atom({ plugin: 'tch-activity', key: 'now' } as const, IDLE)

/** The expanded box's rows, oldest first. */
const history = atom(
  { plugin: 'tch-activity', key: 'history' } as const,
  [] as readonly ActivityEntry[],
)

/** The subagents at work, in the order they were met. */
const team = atom(
  { plugin: 'tch-activity', key: 'agents' } as const,
  [] as readonly ActivityAgent[],
)

/** The Agent calls (or agent ids) whose position picks each agent's color. */
const agentOrder = atom({ plugin: 'tch-activity', key: 'agentOrder' } as const, [] as readonly string[])

/** How the last main turn ended, for the idle headline after a reload. */
const lastTurn = atom(
  { plugin: 'tch-activity', key: 'lastTurn' } as const,
  null as { label: string; outcome: ActivityOutcome } | null,
)

const isExpanded = atom({ plugin: 'tch-activity', key: 'isExpanded' } as const, false)

/** The widest the box has been this session, in columns: it grows to fit and never shrinks. */
const boxWidth = atom({ plugin: 'tch-activity', key: 'width' } as const, 0)

/** The HUD's look, loaded once per session; `null` until then. */
const styleState = atom({ plugin: 'tch-activity', key: 'style' } as const, null as TchStyle | null)

const styleSource = atom(
  { plugin: 'tch-activity', key: 'styleSource' } as const,
  null as TchStyleSource | null,
)

/**
 * Registers tch-activity: a box above the prompt naming what Claude is doing
 * right now in tail-claude-hud's icons and colors, and, behind its triangle,
 * what it has done.
 *
 * The model's stream (`turn.step`) says whether it is reading, thinking,
 * writing the reply or composing a tool's input; `tool.call` which tool
 * runs; `tool.check` when one is put to approval; `session.compact` when
 * the conversation compacts; a subagent's calls carry its `agentId`. Every hook is display only: it reads
 * its event and passes it on unchanged, but for the terminal's spinner line,
 * which the box stands in for.
 *
 * @param on the engine's registrar
 * @param options `userConfig` values; `binary` names the tail-claude-hud
 *   binary, `preset` its preset, `hideSpinner` whether the spinner line is hidden
 */
export const register: Register = (on, options) => {
  const live = createLive()
  const asked: StyleAsked = { isAsked: false, options }

  on('session.start', async ($, e, next) => {
    const result = await next(e)

    ensureStyle($, asked)
    await publish($, live)

    return result
  })

  on('turn.start', async ($, e, next) => {
    // A module loaded after the session started (a plugin folder added or
    // edited mid-session) saw no `session.start`: its first turn loads the look.
    ensureStyle($, asked)

    // A prompt folded into a running turn starts no new one.
    if (live.turnStartMs !== null) {
      return next(e)
    }

    live.turnStartMs = await $.clock.now()
    live.turnActions = 0
    live.isDeclinedLast = false
    live.isFirstRequest = true
    live.resultsToReview = 0
    live.lastAction = null
    dropMainCalls(live)

    const prompt = promptLabel(e.text, MAX_PROMPT_CHARS)

    if (prompt !== '') {
      await remember($, entryOf('turn', prompt))
    }

    await enter($, live, 'requesting')

    keepTicking($, live)
    await publish($, live)

    return next(e)
  })

  on('turn.step', async function* ($, e, next) {
    if (e.agentId !== undefined) {
      return yield* next(e)
    }

    // A module reloaded mid-turn missed the turn's start: it takes the turn
    // up here, so the turn's end returns the box to idle.
    if (live.turnStartMs === null) {
      live.turnStartMs = await $.clock.now()
      keepTicking($, live)
    }

    await enter($, live, 'requesting')
    await publish($, live)

    try {
      for await (const chunk of next(e)) {
        await follow($, live, chunk)
        yield chunk
      }
    } finally {
      live.isFirstRequest = false
      live.composing = null
      await enter($, live, 'requesting')
      await publish($, live)
    }
  })

  on('tool.call', async ($, e, next) => {
    const args = e as unknown as Record<string, unknown>
    const label = activityOf(e.tool, args)

    if (e.agentId !== undefined) {
      const agent = await agentOf($, live, e.agentId)
      const startMs = await $.clock.now()

      // Kept after the call: the agent's last action stands while it thinks.
      agent.label = label
      agent.activeMs = startMs
      live.calls.set(e.tool_use_id, callOf(e.tool, label, startMs, e.agentId))
      await publish($, live, false)

      try {
        return await next(e)
      } finally {
        live.calls.delete(e.tool_use_id)
        await publish($, live, false)
      }
    }

    const startMs = await $.clock.now()

    if (isAgentTool(e.tool)) {
      await rememberAgentKey($, live, e.tool_use_id)
    }

    const call = callOf(e.tool, label, startMs, null)

    live.calls.set(e.tool_use_id, call)
    // Kept after the call: the turn's last action stands while the model works.
    live.lastAction = label
    live.turnActions += 1
    await publish($, live)

    // A call that never settles (the turn was interrupted) keeps this.
    let outcome: ActivityOutcome = 'interrupted'
    let done = activityOf(e.tool, args, 'past')

    try {
      const result = await next(e)

      outcome = result.deny !== undefined ? 'denied' : result.isError ? 'error' : 'ok'

      if (isAgentTool(e.tool)) {
        const handle = args['name']
        const description = args['description']

        // A named agent runs as a teammate, which the engine lists by the
        // call's `name`; the call's `description` is what to show for it.
        if (typeof handle === 'string' && typeof description === 'string' && oneLine(description) !== '') {
          live.described.set(handle, description)
          void sweep($, live)
        }

        if (await launched($, live, result.result, description, e.tool_use_id)) {
          done = `Started an agent: ${typeof description === 'string' ? oneLine(description) : ''}`
        }
      }

      return result
    } finally {
      // A call put to approval that errors without ever being seen running
      // was refused by its decider, which the engine reports as an error.
      if (outcome === 'error' && call.askedMs !== null) {
        outcome = 'denied'
      }

      // Decided before it was taken for a dialog: no wait of yours to leave out.
      if (!call.isPutToYou) {
        call.askedMs = null
      }

      // Still waiting as it ends: the dialog was declined or the turn stopped.
      if (call.isAwaitingApproval) {
        clearWaiting($, live, call)
      }

      const endMs = await $.clock.now()

      endWait(call, endMs)
      live.calls.delete(e.tool_use_id)
      live.resultsToReview += 1
      live.isDeclinedLast = outcome === 'denied'

      await remember($, {
        ...entryOf('tool', done, outcome),
        durationMs: endMs - startMs - call.waitedMs,
        tool: e.tool,
      })

      await publish($, live)
    }
  })

  // An `ask` verdict puts the call to the mode's decider: the person's
  // dialog, or the auto-mode classifier. Display only: the verdict passes on
  // unchanged.
  on('tool.check', async ($, e, next) => {
    const verdict = await next(e)
    const call = e.tool_use_id === undefined ? undefined : live.calls.get(e.tool_use_id)

    if (verdict.decision === 'ask' && call !== undefined && call.askedMs === null && !call.isAwaitingApproval) {
      call.askedMs = await $.clock.now()
      $.clock.after(DIALOG_GRACE_MS, () => void putToYou($, live, call))
    }

    return verdict
  }).catch(($, e, next) => next(e))

  // A call's row is drawn running once it runs, or at once when a decider
  // other than the dialog (the auto-mode classifier) reviews it: for a call
  // that waited on you, it means you approved it.
  on('ui.render', { component: 'ToolUse' }, async ($, e, next) => {
    if (e.props.isRunning) {
      await approved($, live, e.props.tool_use_id)
    }

    return next(e)
  })

  // The run-in-background pill shows only under a running call: the same sign.
  on('ui.render', { component: 'ToolProgress' }, async ($, e, next) => {
    await approved($, live, e.props.tool_use_id)

    return next(e)
  })

  // The box says what the turn does and for how long, so the terminal's
  // spinner line draws nothing in its place. A Spinner is drawn for the
  // conversation in view, the main one or an agent's, and the box shows both.
  // A render hook answers a tree, never null: an empty Box draws nothing.
  on('ui.render', { component: 'Spinner' }, ($, e, next) => {
    if (e.surface !== 'terminal' || !isSpinnerHidden(options)) {
      return next(e)
    }

    const { Box } = $.ui.resolve(e)

    return <Box />
  }).catch(($, e, next) => next(e))

  on('agent.spawn', async ($, e, next) => {
    const result = await next(e)

    if (result.agentId !== undefined) {
      const startMs = await $.clock.now()
      const name = e.description || e.name || UNNAMED_AGENT

      live.agentNames.set(result.agentId, name)
      live.agents.set(result.agentId, {
        name,
        label: STARTING_LABEL,
        startMs,
        activeMs: startMs,
        toolUseId: e.tool_use_id,
        isBackground: e.background,
        colorKey: e.tool_use_id || result.agentId,
      })
      await placeAgent($, live, result.agentId, e.tool_use_id)
      keepTicking($, live)
      await publish($, live)
    }

    return result
  })

  // A compaction of the main conversation, timed from the call to its
  // answer. One made ahead of time (`precompute`) runs unseen in the
  // background and is not shown.
  on('session.compact', async ($, e, next) => {
    if (e.agentId !== undefined || e.trigger === 'precompute') {
      return next(e)
    }

    await enter($, live, 'compacting')
    keepTicking($, live)
    await publish($, live)

    let outcome: ActivityOutcome = 'error'

    try {
      const result = await next(e)

      outcome = result.skip === undefined ? 'ok' : 'denied'

      return result
    } finally {
      const durationMs = (await $.clock.now()) - live.phaseStartMs
      const label = outcome === 'ok' ? 'Compacted the conversation' : outcome === 'denied' ? 'Compaction skipped' : 'Compaction failed'

      await enter($, live, live.turnStartMs === null ? 'idle' : 'requesting')
      await remember($, { ...entryOf('compact', label, outcome), durationMs })
      keepTicking($, live)
      await publish($, live)
    }
  })

  // Every end, a /clear's included, takes the ending session's breadcrumb
  // with it; a /clear's new session id is not the one that waited.
  on('session.end', async ($, e, next) => {
    clearWaiting($, live, null, e.sessionId)

    return next(e)
  })

  on('turn.complete', async ($, e, next) => {
    if (e.agentId !== undefined) {
      await finish($, live, e.agentId, outcomeOf(e.reason))

      return next(e)
    }

    if (live.turnStartMs === null) {
      return next(e)
    }

    const durationMs = (await $.clock.now()) - live.turnStartMs
    const end = turnEndOf(e.reason, live.isDeclinedLast)
    const outcome = outcomeOf(end)
    const label = turnEndLabel(end, durationMs, live.turnActions)

    await enter($, live, 'idle')
    // The row says its time in its label, so it has no duration of its own.
    await remember($, entryOf('end', capitalized(label), outcome))

    live.lastTurn = { label, outcome }
    await update($, lastTurn, () => live.lastTurn)
    live.turnStartMs = null
    dropMainCalls(live)

    keepTicking($, live)
    await publish($, live)

    return next(e)
  })

  on('ui.render', { component: 'AbovePrompt' }, async ($, e, next) => {
    if (e.props.hasSurvey) {
      return next(e)
    }

    // A reload (an edit, a changed option) sees no session start: the first
    // draw after it checks the look, off the draw.
    ensureStyle($, asked)

    // The band is one site for every plugin: what the plugins beneath draw
    // stays, under the box, so another mod's band is never hidden.
    const below = await next(e)
    const { Box } = $.ui.resolve(e)

    return (
      <Box flexDirection="column">
        {await drawBox($, e)}
        {below}
      </Box>
    )
  })
}

const MAX_HISTORY = 100

const MAX_PROMPT_CHARS = 60

const MAX_THOUGHT_BUFFER = 2000

const MAX_INPUT_HEAD = 4000

/** Streaming chunks arrive far faster than anyone reads; publish at most this often. */
const STREAM_THROTTLE_MS = 250

/** The running timer shows tenths under ten seconds; the box redraws this often then. */
const TICK_MS = 100

/** Past ten seconds the timer counts whole seconds: the box redraws, and checks on agents, once a second. */
const TICKS_PER_SECOND = 10

/**
 * Writes the headline. A streaming update (`isUrgent` false) lands at most
 * every STREAM_THROTTLE_MS, the last one held on a timer.
 */
async function publish($: EngineInterface, live: Live, isUrgent = true): Promise<void> {
  const nowMs = await $.clock.now()

  if (!isUrgent && nowMs - live.lastPublishMs < STREAM_THROTTLE_MS) {
    live.trailing ??= $.clock.after(STREAM_THROTTLE_MS, () => {
      live.trailing = null
      void publish($, live)
    })

    return
  }

  live.lastPublishMs = nowMs
  await restore($, live)

  const headline = current(live)
  const working: ActivityAgent[] = [...live.agents.entries()].map(([id, agent]) => ({
    name: agent.name,
    label: agentLabel(live, { ...agent, id }),
    sinceMs: agent.startMs,
    colorIndex: colorIndexOf(live, agent.colorKey),
    isWaiting: isAgentWaiting(live, id),
  }))

  await update($, now, () => headline)
  await update($, team, () => working)
}

/**
 * Takes up, once per module load, what the session's state kept across a
 * reload: the agent order and how the last turn ended.
 */
async function restore($: EngineInterface, live: Live): Promise<void> {
  if (live.isRestored) {
    return
  }

  live.isRestored = true
  live.agentOrder = await read($, agentOrder)
  live.lastTurn ??= await read($, lastTurn)
}

/** Adds an agent to the order that picks its color, as the HUD's agents widget orders it. */
async function rememberAgentKey($: EngineInterface, live: Live, key: string): Promise<void> {
  await restore($, live)

  const order = withAgentKey(live.agentOrder, key)

  if (order !== live.agentOrder) {
    live.agentOrder = order
    await update($, agentOrder, () => order)
  }
}

/**
 * Places an agent in the color order by its Agent call, dropping the entry
 * its id took if its own calls arrived first.
 */
async function placeAgent($: EngineInterface, live: Live, agentId: string, toolUseId: string): Promise<void> {
  await restore($, live)

  const order = toolUseId === '' ? withAgentKey(live.agentOrder, agentId) : withAgentCall(live.agentOrder, agentId, toolUseId)

  if (order.join('\n') !== live.agentOrder.join('\n')) {
    live.agentOrder = order
    await update($, agentOrder, () => order)
  }
}

/**
 * Marks a call that waited on approval as approved, once it is seen running:
 * the dialog has closed, and the call's timer counts from here.
 */
async function approved($: EngineInterface, live: Live, toolUseId: string): Promise<void> {
  const call = live.calls.get(toolUseId)

  // Drawn running within the grace: no dialog is up, the decider's time is the call's.
  if (call !== undefined && call.askedMs !== null && !call.isPutToYou) {
    call.askedMs = null

    return
  }

  if (call?.isAwaitingApproval) {
    call.isAwaitingApproval = false

    if (call.agentId === null) {
      clearWaiting($, live, call)
    }

    // The wait ends now, before the call can: a fast tool's end must find it closed.
    endWait(call, await $.clock.now())

    // Drawing may not write state: the headline is published just after.
    $.clock.after(0, () => void publish($, live))
  }
}

/**
 * How long a call put to approval may go undrawn before the box takes it for
 * the dialog. The engine draws a call running at once when a decider other
 * than the dialog (the auto-mode classifier, a host) reviews it, and only
 * after you answer when the dialog asks you.
 */
const DIALOG_GRACE_MS = 250

/**
 * Takes a call still asked and undrawn after the grace as waiting on you:
 * the headline and border say so and, for the main loop, the breadcrumb is
 * written.
 */
async function putToYou($: EngineInterface, live: Live, call: Call): Promise<void> {
  const isInFlight = [...live.calls.values()].includes(call)

  if (!isInFlight || call.askedMs === null || call.isAwaitingApproval) {
    return
  }

  call.isAwaitingApproval = true
  call.isPutToYou = true

  if (call.agentId === null) {
    await markWaiting($, call.tool)
  }

  await publish($, live)
}

/**
 * Writes this session's approval breadcrumb, which other sessions' HUD
 * status lines show as an alert. A failure is let go: the HUD ignores a
 * breadcrumb past its staleness limit.
 */
async function markWaiting($: EngineInterface, tool: string): Promise<void> {
  try {
    const home = await $.env.get('HOME')

    if (home === undefined || home === '') {
      return
    }

    const sessionId = await $.session.id()

    await $.fs.write(waitingPathOf(home, sessionId), breadcrumbOf(sessionId, await $.session.cwd(), tool))
  } catch {
    // The HUD's staleness limit retires a breadcrumb that never lands.
  }
}

/** `rm` answers at once; this bounds a stuck one. */
const REMOVE_TIMEOUT_MS = 1500

/**
 * Removes a session's approval breadcrumb, unless another main-loop call
 * besides `ending` still waits. The session is `sessionId`, or the current
 * one, read fresh since a /clear changes it. The removal runs just after:
 * inside a declined call's chain the engine aborts `$` calls. `$.fs` has no
 * delete, so `/bin/rm -f` removes the file; a failure is let go, as the HUD
 * ignores a breadcrumb past its staleness limit.
 */
function clearWaiting($: EngineInterface, live: Live, ending: Call | null, sessionId?: string): void {
  const isStillWaiting = [...live.calls.values()]
    .some(call => call !== ending && call.agentId === null && call.isAwaitingApproval)

  if (sessionId === undefined && isStillWaiting) {
    return
  }

  $.clock.after(0, async () => {
    try {
      const home = await $.env.get('HOME')

      if (home === undefined || home === '') {
        return
      }

      const path = waitingPathOf(home, sessionId ?? (await $.session.id()))

      await $.process.run(['/bin/rm', '-f', path], { timeoutMs: REMOVE_TIMEOUT_MS })
    } catch {
      // The HUD's staleness limit retires a breadcrumb left behind.
    }
  })
}

/** The agent at work under `agentId`, taken up if its spawn came before a reload. */
async function agentOf($: EngineInterface, live: Live, agentId: string): Promise<Agent> {
  const known = live.agents.get(agentId)

  if (known) {
    return known
  }

  const startMs = await $.clock.now()
  const agent: Agent = {
    name: live.agentNames.get(agentId) ?? UNNAMED_AGENT,
    label: STARTING_LABEL,
    startMs,
    activeMs: startMs,
    toolUseId: null,
    isBackground: true,
    colorKey: agentId,
  }

  live.agents.set(agentId, agent)
  await rememberAgentKey($, live, agentId)
  keepTicking($, live)

  return agent
}

/**
 * Lets an agent go. A background agent's run is recorded in the history; a
 * foreground one's is its Agent call's row.
 */
async function finish(
  $: EngineInterface,
  live: Live,
  agentId: string,
  outcome: ActivityOutcome,
): Promise<void> {
  const agent = live.agents.get(agentId)

  if (!agent) {
    return
  }

  live.agents.delete(agentId)

  if (agent.isBackground) {
    await remember($, {
      ...entryOf('tool', `Ran an agent: ${agent.name}`, outcome),
      durationMs: (await $.clock.now()) - agent.startMs,
      tool: 'Agent',
    })
  }

  keepTicking($, live)
  await publish($, live)
}

/**
 * Names agents first met by their calls, and lets go of agents the engine
 * lists as no longer running, in case their stop never reached the box.
 */
async function sweep($: EngineInterface, live: Live): Promise<void> {
  if (live.agents.size === 0 || live.isSweeping) {
    return
  }

  live.isSweeping = true

  try {
    await sweepListed($, live, await $.agent.list())
  } catch {
    // The list is a backstop; the box keeps going without it.
  } finally {
    live.isSweeping = false
  }
}

async function sweepListed($: EngineInterface, live: Live, listed: readonly AgentInfo[]): Promise<void> {
  const statuses = new Map(listed.map(info => [info.id, info.status]))
  let isRenamed = false

  for (const [agentId, agent] of [...live.agents.entries()]) {
    const status = statuses.get(agentId)

    // An agent first met by its calls is named once the engine lists it.
    if (!live.agentNames.has(agentId)) {
      const handle = listed.find(info => info.id === agentId)?.name
      const described = handle === undefined ? undefined : live.described.get(handle)

      if (described !== undefined) {
        live.agentNames.set(agentId, described)
      }

      const name = described ?? handle

      if (name !== undefined && name !== agent.name) {
        agent.name = name
        isRenamed = true
      }
    }

    if (status !== undefined && status !== 'running') {
      await finish($, live, agentId, status === 'completed' ? 'ok' : status === 'failed' ? 'error' : 'interrupted')
    }
  }

  if (isRenamed) {
    await publish($, live)
  }
}

/**
 * Keeps the elapsed time ticking while a turn runs, an agent works or the
 * conversation compacts, and stops it once none does.
 */
function keepTicking($: EngineInterface, live: Live): void {
  const isBusy = live.turnStartMs !== null || live.agents.size > 0 || live.phase === 'compacting'

  if (isBusy && live.ticker === null) {
    let ticks = 0

    live.ticker = $.clock.every(TICK_MS, async () => {
      ticks += 1

      const isWholeSecond = ticks % TICKS_PER_SECOND === 0

      if (isWholeSecond) {
        void sweep($, live)
      }

      if (isWholeSecond || hasTenthsTimer(live, await $.clock.now())) {
        $.ui.invalidate('ui.render')
      }
    })
  } else if (!isBusy && live.ticker !== null) {
    live.ticker.cancel()
    live.ticker = null
  }
}

/** Whether a timer the box draws is still under ten seconds, and so shows tenths. */
function hasTenthsTimer(live: Live, nowMs: number): boolean {
  const headline = current(live)
  const starts = [...live.agents.values()].map(agent => agent.startMs)

  if (headline.phase !== 'idle') {
    starts.push(headline.sinceMs)
  }

  return starts.some(startMs => nowMs - startMs < TENTHS_UNDER_MS)
}

async function remember($: EngineInterface, entry: ActivityEntry): Promise<void> {
  await update($, history, entries => [...entries, entry].slice(-MAX_HISTORY))
}

/** Enters a model phase, recording the thinking or reply block it closes. */
async function enter($: EngineInterface, live: Live, next: ModelPhase): Promise<void> {
  if (next === live.phase) {
    return
  }

  const nowMs = await $.clock.now()
  const durationMs = nowMs - live.phaseStartMs

  if (live.phase === 'thinking') {
    const words = live.thoughtWords
    const label = words > 0 ? `Thought (${plural(words, 'word')})` : 'Thought'

    await remember($, { ...entryOf('thought', label), durationMs })
  }

  if (live.phase === 'writing' && live.replyWords > 0) {
    const label = `Wrote the reply (${plural(live.replyWords, 'word')})`

    await remember($, { ...entryOf('reply', label), durationMs })
  }

  if (next === 'thinking') {
    live.thought = ''
    live.thoughtWords = 0
    live.isThoughtInWord = false
  }

  if (next === 'writing') {
    live.replyWords = 0
    live.isReplyInWord = false
  }

  live.phase = next
  live.phaseStartMs = nowMs
}

/** Follows one chunk of the main loop's response. */
async function follow($: EngineInterface, live: Live, chunk: TurnStepChunk): Promise<void> {
  // The model went on after a declined call: the decline did not end the turn.
  if (chunk.kind === 'text' || chunk.kind === 'tool') {
    live.isDeclinedLast = false
  }

  switch (chunk.kind) {
    case 'thinking': {
      const isNew = live.phase !== 'thinking'

      await enter($, live, 'thinking')
      live.resultsToReview = 0
      const { added, isInWord } = wordsAdded(chunk.text, live.isThoughtInWord)

      live.thought = (live.thought + chunk.text).slice(-MAX_THOUGHT_BUFFER)
      live.thoughtWords += added
      live.isThoughtInWord = isInWord
      await publish($, live, isNew)

      return
    }
    case 'text': {
      const isNew = live.phase !== 'writing'

      await enter($, live, 'writing')
      live.resultsToReview = 0
      const { added, isInWord } = wordsAdded(chunk.text, live.isReplyInWord)

      live.replyWords += added
      live.isReplyInWord = isInWord
      await publish($, live, isNew)

      return
    }
    case 'tool':
      await enter($, live, 'composing')
      live.resultsToReview = 0
      live.composing = { tool: chunk.name, head: '', chars: 0 }

      // The stream names Agent calls in the order the transcript keeps them,
      // the order the HUD colors agents by.
      if (isAgentTool(chunk.name)) {
        await rememberAgentKey($, live, chunk.id)
      }

      await publish($, live)

      return
    case 'input': {
      const { composing } = live

      if (composing) {
        composing.chars += chunk.json.length

        if (composing.head.length < MAX_INPUT_HEAD) {
          composing.head += chunk.json
        }

        await publish($, live, false)
      }

      return
    }
  }
}

/**
 * Names a background agent by the Agent call that launched it: the call's
 * short `description`, keyed by the id its result carries (the result's own
 * `description` can hold the whole prompt). Its own calls may already have
 * shown it.
 *
 * @returns whether the call launched a background agent
 */
async function launched(
  $: EngineInterface,
  live: Live,
  result: unknown,
  asked: unknown,
  toolUseId: string,
): Promise<boolean> {
  if (typeof result !== 'object' || result === null) {
    return false
  }

  const { agentId, description, status } = result as Record<string, unknown>

  if (typeof agentId !== 'string' || status !== 'async_launched') {
    return false
  }

  const name = [asked, description].find(
    (text): text is string => typeof text === 'string' && oneLine(text) !== '',
  )

  if (name === undefined) {
    return true
  }

  const agent = await agentOf($, live, agentId)

  live.agentNames.set(agentId, name)
  agent.name = name
  agent.toolUseId ??= toolUseId
  agent.colorKey = toolUseId
  await placeAgent($, live, agentId, toolUseId)
  await publish($, live)

  return true
}

/**
 * The HUD answers in a few milliseconds. A binary too old to know `style`
 * runs as the statusline, which reads its JSON from stdin: the empty stdin
 * ends it at once, and this bounds anything slower.
 */
const STYLE_TIMEOUT_MS = 1500

/** Whether this module load has asked the HUD for its look yet. */
type StyleAsked = { isAsked: boolean; options: PluginOptions }

/**
 * Checks the look once per module load, off the hook that asks: the hook's
 * event goes on at once and the box redraws when the look arrives. A stored
 * look stands only when the HUD supplied it with this load's binary and
 * preset; the built-in look, or one from other options, is asked again.
 */
function ensureStyle($: EngineInterface, asked: StyleAsked): void {
  if (asked.isAsked) {
    return
  }

  asked.isAsked = true
  $.clock.after(0, () => void refreshStyle($, asked.options))
}

async function refreshStyle($: EngineInterface, options: PluginOptions): Promise<void> {
  const preset = presetOf(options)
  let binary = BARE_BINARY

  try {
    binary = await binaryOf($, options)
  } catch {
    // Without HOME or the file check, the bare name is still worth a try.
  }

  const stored = await read($, styleSource)

  if (!isStyleStale(stored, binary, preset)) {
    return
  }

  const loaded = await loadStyle($, binary, preset)

  if (loaded.source.kind === 'builtin') {
    $.ui.log(`tch-activity: ${loaded.source.reason}; drawing the built-in look`, { to: 'debug' })
  }

  await update($, styleState, () => loaded.style)
  await update($, styleSource, () => loaded.source)
  $.ui.invalidate('ui.render')
}

/**
 * Asks the HUD for its look. Never rejects: a binary that is missing, slow,
 * failing or answering something unreadable yields the built-in look and the
 * reason, which the expanded history shows as one dim line.
 */
async function loadStyle($: EngineInterface, binary: string, preset: string): Promise<LoadedStyle> {
  const argv = [binary, 'style', '--json', ...(preset === '' ? [] : ['--preset', preset])]
  const command = argv.slice(0, 3).join(' ') + (preset === '' ? '' : ` --preset ${preset}`)

  try {
    const ran = await $.process.run(argv, { stdin: '', timeoutMs: STYLE_TIMEOUT_MS })

    if (ran.exitCode !== 0) {
      return builtin(`${command} exited ${ran.exitCode}`, binary, preset)
    }

    const parsed = parseStyle(ran.stdout)

    return parsed === null
      ? builtin(`${command} printed no style this mod reads`, binary, preset)
      : { style: parsed, source: { kind: 'hud', binary, preset } }
  } catch (error) {
    return builtin(`${binary} could not run: ${error instanceof Error ? error.message : String(error)}`, binary, preset)
  }
}

/** The `preset` option: the preset the statusline command runs with, or empty. */
function presetOf(options: PluginOptions): string {
  const preset = options['preset']

  return typeof preset === 'string' ? preset.trim() : ''
}

/** The `hideSpinner` option: whether the terminal's spinner line is hidden; on unless set to false. */
function isSpinnerHidden(options: PluginOptions): boolean {
  return options['hideSpinner'] !== false
}

/**
 * The HUD binary: the `binary` option when set; else `$HOME/go/bin`'s copy
 * when it exists, since a session's PATH is the login one and often lacks
 * Go's bin folder; else the bare name.
 */
async function binaryOf($: EngineInterface, options: PluginOptions): Promise<string> {
  const configured = options['binary']

  if (typeof configured === 'string' && configured.trim() !== '') {
    return configured.trim()
  }

  const home = await $.env.get('HOME')

  if (home !== undefined && home !== '') {
    const goBin = `${home}${GO_BIN_SUFFIX}`

    if (await $.fs.exists(goBin)) {
      return goBin
    }
  }

  return BARE_BINARY
}

const MAX_LABEL_CHARS = 120
const MAX_AGENT_NAME_CHARS = 40

/** The HUD's segment separator, drawn faint as the statusline draws it. */
const SEPARATOR = ' | '

/** The headline's show/hide arrow, one column wide. */
const TOGGLE_COLUMNS = 1

type AbovePromptInput = RenderInput<'AbovePrompt'>

/**
 * Draws the box above the prompt: the headline, a row per agent at work
 * beside the main loop, and, behind the triangle, the history. The box is as
 * wide as the widest row it has drawn this session, so it never shrinks
 * between states.
 */
async function drawBox($: EngineInterface, e: AbovePromptInput) {
  const { Box, Text, Button } = $.ui.resolve(e)

  const style = (await read($, styleState)) ?? BUILTIN_STYLE
  const source = await read($, styleSource)
  const headline = await read($, now)
  const entries = await read($, history)
  const working = await read($, team)
  const isOpen = await read($, isExpanded)
  const widest = await read($, boxWidth)
  const nowMs = await $.clock.now()

  /**
   * A row's segments as Text: a clipped segment gives way on a narrow
   * screen; the others keep their width, since they would wrap.
   */
  const line = (segments: readonly Segment[]) =>
    segments.map(segment => {
      const text = (
        <Text color={segment.color} dimColor={segment.isFaint === true} wrap={segment.isClipped ? 'truncate-end' : undefined}>
          {segment.text}
        </Text>
      )

      return segment.isClipped ? text : <Box flexShrink={0}>{text}</Box>
    })

  /**
   * One agent's row, as the HUD's agents widget draws a running agent: the
   * task icon and name in the agent's palette color, then its latest action
   * and its running time, each after a faint separator.
   */
  const agentRow = (agent: ActivityAgent): Segment[] => {
    const tint = agentColor(style, agent.colorIndex)

    return [
      { text: `${icon(style, 'task')} `, color: tint },
      { text: printable(agent.name, MAX_AGENT_NAME_CHARS), color: tint, isClipped: true },
      { text: SEPARATOR, isFaint: true },
      { text: printable(agent.label, MAX_LABEL_CHARS), isClipped: true },
      { text: `${SEPARATOR}${elapsedOf(nowMs - agent.sinceMs)}`, isFaint: true },
    ]
  }

  /**
   * One history row: its glyph (for a tool call, the category icon in the
   * outcome's color), the label, and the duration drawn faint, as the HUD's
   * tools widget draws a finished tool.
   */
  const historyRow = (entry: ActivityEntry): Segment[] => {
    const mark = entryMark(style, entry)
    const isQuiet = entry.kind === 'thought' || entry.kind === 'end'

    return [
      { text: `${mark.glyph} `, color: mark.color, isFaint: mark.isFaint },
      { text: printable(entry.label, MAX_LABEL_CHARS), isFaint: isQuiet, isClipped: true },
      { text: entry.durationMs === null ? '' : ` ${durationOf(entry.durationMs)}`, isFaint: true },
    ]
  }

  const isWorking = headline.phase !== 'idle'
  const mark = headlineMark(style, headline)
  const tail = [headline.detail, isWorking ? elapsedOf(nowMs - headline.sinceMs) : null]
    .filter((part): part is string => part !== null)
    .map(part => `${SEPARATOR}${part}`)
    .join('')

  // An agent's name takes its palette color, as the HUD's agents widget draws it.
  const lead: Segment[] = headline.lead === null
    ? []
    : [
        { text: printable(headline.lead, MAX_LABEL_CHARS), color: mark.color, isClipped: true },
        { text: SEPARATOR, isFaint: true },
      ]

  const headlineSegments: Segment[] = [
    { text: ` ${mark.glyph} `, color: mark.color },
    ...lead,
    { text: printable(headline.label, MAX_LABEL_CHARS), isClipped: true },
    { text: tail, isFaint: true },
  ]

  // One agent the headline names; more, or one beside the main loop's
  // work, get a row each.
  const isTeamListed = working.length > 1 || (working.length === 1 && headline.phase !== 'agent')
  const note = styleNote(source)
  const layout = bandLayout({
    maxRows: e.props.maxRows,
    agents: isTeamListed ? working.length : 0,
    entries: entries.length,
    isOpen,
    hasNote: note !== null,
  })

  const listed = layout.agentRows > 0 ? working.slice(-layout.agentRows) : []
  const unlisted = working.length - listed.length
  const agentLines = listed.map(agentRow)
  const moreAgents: Segment[][] = layout.hasMoreAgents ? [[{ text: `… ${plural(unlisted, 'more agent')}`, isFaint: true }]] : []

  const shown = layout.historyRows > 0 ? entries.slice(-layout.historyRows) : []
  const hidden = entries.length - shown.length
  const historyLines = shown.map(historyRow)
  const earlier: Segment[][] = layout.hasEarlier ? [[{ text: `… ${hidden} earlier`, isFaint: true }]] : []
  const empty: Segment[][] = entries.length === 0 ? [[{ text: 'No actions yet', isFaint: true }]] : []
  const footer: Segment[][] = layout.hasNote && note !== null ? [[{ text: note, isFaint: true, isClipped: true }]] : []

  const teamLines = isTeamListed ? [...agentLines, ...moreAgents] : []
  const bodyLines = layout.isHistoryShown ? [...earlier, ...historyLines, ...empty, ...footer] : []

  const natural = boxColumnsOf([
    TOGGLE_COLUMNS + columnsOf(headlineSegments),
    ...teamLines.map(segments => AGENT_INDENT_COLUMNS + columnsOf(segments)),
    ...bodyLines.map(columnsOf),
  ])

  if (natural > widest) {
    // Drawing may not write state: the wider box is kept just after.
    $.clock.after(0, () => void update($, boxWidth, width => Math.max(width, natural)))
  }

  const teamBox = teamLines.length > 0
    ? [
        <Box key="agents" flexDirection="column" paddingLeft={AGENT_INDENT_COLUMNS}>
          {teamLines.map((segments, i) => (
            <Box key={`agent-${i}`} flexDirection="row">
              {line(segments)}
            </Box>
          ))}
        </Box>,
      ]
    : []

  const body = layout.isHistoryShown
    ? [
        <Box key="history" flexDirection="column" marginTop={1}>
          {bodyLines.map((segments, i) => (
            <Box key={`row-${i}`} flexDirection="row">
              {line(segments)}
            </Box>
          ))}
        </Box>,
      ]
    : []

  return (
      <Box
        key="frame"
        flexDirection="column"
        borderStyle="round"
        borderColor={borderColor(style, headline, working)}
        paddingX={1}
        alignSelf="flex-start"
        minWidth={Math.min(Math.max(widest, natural), e.props.bodyColumns)}
      >
        <Box key="activity" flexDirection="row">
          <Button
            key="toggle"
            plain
            label={isOpen ? '▾' : '▸'}
            onPress={() => update($, isExpanded, open => !open)}
          />
          {line(headlineSegments)}
        </Box>
        {teamBox}
        {body}
      </Box>
  )
}
