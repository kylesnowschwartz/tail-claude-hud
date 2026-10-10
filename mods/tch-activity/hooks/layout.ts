/** Rows every box takes: the top and bottom border and the headline. */
const FIXED_ROWS = 3

/** The blank row between the headline (and agents) and the history. */
const HISTORY_GAP_ROWS = 1

export const MAX_AGENT_ROWS = 6
export const MAX_HISTORY_ROWS = 12

/** What the band shows, given the rows it may take. */
export type BandLayout = {
  /** Agent rows drawn, newest last. */
  agentRows: number
  /** Whether a `… N more agents` row follows them. */
  hasMoreAgents: boolean
  /** Whether the history is drawn: it is open and at least one row fits. */
  isHistoryShown: boolean
  /** History entries drawn, newest last. */
  historyRows: number
  /** Whether an `… N earlier` row leads them. */
  hasEarlier: boolean
  /** Whether the built-in-look note closes the history. */
  hasNote: boolean
}

/**
 * Fits the band into `maxRows`, counting every row it draws: the frame and
 * headline, each agent row and the `more agents` row, the history's gap,
 * its `earlier` row, its entries or `No actions yet`, and the note. A tree
 * taller than `maxRows` scrolls in the band, so nothing past it is drawn.
 */
export function bandLayout(input: {
  maxRows: number
  agents: number
  entries: number
  isOpen: boolean
  hasNote: boolean
}): BandLayout {
  let room = Math.max(0, input.maxRows - FIXED_ROWS)

  const agentCap = Math.min(MAX_AGENT_ROWS, room)
  const hasMoreAgents = input.agents > agentCap && agentCap > 0
  const agentRows = hasMoreAgents ? agentCap - 1 : Math.min(input.agents, agentCap)

  room -= agentRows + (hasMoreAgents ? 1 : 0)

  // The history needs its gap and one row at least.
  const isHistoryShown = input.isOpen && room >= HISTORY_GAP_ROWS + 1

  if (!isHistoryShown) {
    return { agentRows, hasMoreAgents, isHistoryShown, historyRows: 0, hasEarlier: false, hasNote: false }
  }

  room -= HISTORY_GAP_ROWS

  // The note gives way before the entries do.
  const hasNote = input.hasNote && room >= 2
  room -= hasNote ? 1 : 0

  const historyCap = Math.min(MAX_HISTORY_ROWS, room)
  const hasEarlier = input.entries > historyCap
  const historyRows = hasEarlier ? Math.max(0, historyCap - 1) : input.entries

  return { agentRows, hasMoreAgents, isHistoryShown, historyRows, hasEarlier, hasNote }
}
