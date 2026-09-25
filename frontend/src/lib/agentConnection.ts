/*
 * Presentation of the local agent connection: the MCP client config and CLI
 * lines the settings page offers for copying. The installed app binary is the
 * `daygo` entry (cmd/daygo routes `mcp` and CLI commands before starting the
 * GUI), so the snippets name its absolute path rather than assuming a `daygo`
 * on PATH.
 */

const FALLBACK_COMMAND = 'daygo'

/** Quotes a POSIX shell word only when it needs it. */
export function shellQuote(word: string): string {
  if (/^[A-Za-z0-9_@%+=:,./-]+$/.test(word)) return word
  return `'${word.replaceAll("'", `'"'"'`)}'`
}

function command(executablePath: string): string {
  return executablePath === '' ? FALLBACK_COMMAND : executablePath
}

/** The `mcpServers` entry accepted by Claude Desktop / Claude Code style clients. */
export function mcpClientConfig(executablePath: string): string {
  return JSON.stringify({ mcpServers: { daygo: { command: command(executablePath), args: ['mcp'] } } }, null, 2)
}

/** One CLI invocation, shell-quoted so it pastes into a terminal unchanged. */
export function cliCommand(executablePath: string, args: readonly string[]): string {
  return [command(executablePath), ...args].map(shellQuote).join(' ')
}

/*
 * True when macOS runs the app from a path that will not survive a relaunch:
 * App Translocation (a quarantined app opened where it was downloaded) or a
 * mounted disk image. An MCP config pointing there breaks on the next launch,
 * so the page asks the user to move Daygo into Applications first.
 */
export function isEphemeralExecutable(executablePath: string): boolean {
  return executablePath.includes('/AppTranslocation/') || executablePath.startsWith('/Volumes/')
}
