// The approval breadcrumb tail-claude-hud's `permission` widget reads: one
// file per session waiting on an approval, which other sessions' status lines
// show as an alert. The HUD's own `hook permission-request` writes the same file.

/** Where the HUD keeps breadcrumbs, under the home folder. */
const WAITING_DIR_SUFFIX = '/.config/tail-claude-hud/waiting'

/** The breadcrumb file of a session. */
export function waitingPathOf(home: string, sessionId: string): string {
  return `${home}${WAITING_DIR_SUFFIX}/${sessionId}`
}

/**
 * The breadcrumb's JSON, as the HUD writes it: the session, its project (the
 * working directory's last component, empty at a root) and the tool waiting.
 */
export function breadcrumbOf(sessionId: string, cwd: string, tool: string): string {
  const project = cwd.split('/').filter(part => part !== '').at(-1) ?? ''

  return JSON.stringify({ session_id: sessionId, project, tool_name: tool })
}
