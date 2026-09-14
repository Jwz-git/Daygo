import assert from 'node:assert/strict'
import test from 'node:test'

import { formatClockTime, formatTimeZoneName } from '../src/lib/timeFormat'

test('formats clock time with the selected locale and time zone', () => {
  const timestamp = Date.UTC(2026, 0, 1, 3, 5) / 1000

  assert.equal(formatClockTime(timestamp, 'zh-CN', 'Asia/Shanghai'), '11:05')
  assert.match(formatClockTime(timestamp, 'en', 'America/New_York'), /10:05/)
})

test('localizes a valid time zone without exposing its IANA identifier', () => {
  const zh = formatTimeZoneName('Asia/Shanghai', 'zh-CN')
  const en = formatTimeZoneName('Asia/Shanghai', 'en')

  assert.notEqual(zh, '')
  assert.notEqual(en, '')
  assert.notEqual(zh, 'Asia/Shanghai')
  assert.notEqual(en, 'Asia/Shanghai')
})

test('returns an empty label for invalid time zones', () => {
  assert.equal(formatTimeZoneName('Local', 'zh-CN'), '')
  assert.equal(formatTimeZoneName('Not/AZone', 'en'), '')
})
