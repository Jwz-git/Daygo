import assert from 'node:assert/strict'
import test from 'node:test'

import { parseAgentConnection } from '../src/api/agent'
import { cliCommand, isEphemeralExecutable, mcpClientConfig, shellQuote } from '../src/lib/agentConnection'

const APP = '/Applications/Daygo.app/Contents/MacOS/Daygo'

test('mcpClientConfig launches the running binary with the mcp argument', () => {
  assert.deepEqual(JSON.parse(mcpClientConfig(APP)), {
    mcpServers: { daygo: { command: APP, args: ['mcp'] } },
  })
  assert.equal(JSON.parse(mcpClientConfig('')).mcpServers.daygo.command, 'daygo')
})

test('cliCommand quotes only the words a shell would split', () => {
  assert.equal(cliCommand(APP, ['timeline', 'today', '--json']), `${APP} timeline today --json`)
  assert.equal(
    cliCommand('/Users/a b/Daygo', ['write', 'goal_set', '{"day":"YYYY-MM-DD"}']),
    `'/Users/a b/Daygo' write goal_set '{"day":"YYYY-MM-DD"}'`,
  )
  assert.equal(shellQuote("it's"), `'it'"'"'s'`)
})

test('parseAgentConnection rejects a payload that drifted from the DTO', () => {
  assert.deepEqual(parseAgentConnection({ executablePath: APP, socketActive: true }), {
    executablePath: APP,
    socketActive: true,
  })
  assert.throws(() => parseAgentConnection({ executablePath: APP }))
  assert.throws(() => parseAgentConnection(null))
})

test('isEphemeralExecutable flags translocated and disk-image launches only', () => {
  assert.equal(isEphemeralExecutable(APP), false)
  assert.equal(isEphemeralExecutable('/private/var/folders/x/T/AppTranslocation/ABC/d/Daygo.app/Contents/MacOS/Daygo'), true)
  assert.equal(isEphemeralExecutable('/Volumes/Daygo/Daygo.app/Contents/MacOS/Daygo'), true)
  assert.equal(isEphemeralExecutable(''), false)
})
