import { GetAgentConnection } from '../../wailsjs/go/app/Backend'

/** How a local MCP / CLI client reaches this Daygo instance (docs/05 §5.9). */
export interface AgentConnection {
  /** Absolute path of the running executable; empty when the OS cannot report it. */
  executablePath: string
  /** True while this instance serves agent.sock (only the read-write instance does). */
  socketActive: boolean
}

/** Thrown when the page runs in a plain browser, outside the Wails WebView. */
export const WAILS_UNAVAILABLE = 'wails_unavailable'

function hasBridge(): boolean {
  return (window as { go?: unknown }).go !== undefined
}

export function parseAgentConnection(value: unknown): AgentConnection {
  if (typeof value !== 'object' || value === null) throw new Error('invalid agent connection payload')
  const record = value as Record<string, unknown>
  if (typeof record.executablePath !== 'string' || typeof record.socketActive !== 'boolean') {
    throw new Error('invalid agent connection payload')
  }
  return { executablePath: record.executablePath, socketActive: record.socketActive }
}

export async function getAgentConnection(): Promise<AgentConnection> {
  if (!hasBridge()) throw new Error(WAILS_UNAVAILABLE)
  const payload: unknown = await GetAgentConnection()
  return parseAgentConnection(payload)
}
