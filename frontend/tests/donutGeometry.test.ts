import assert from 'node:assert/strict'
import test from 'node:test'

import {
  buildDonutSectors,
  fullRingPath,
  sectorPath,
  type DonutSlice,
} from '../src/views/Timeline/donut'

test('fullRingPath generates two semicircles per radius without degenerate zero-length arcs', () => {
  const path = fullRingPath(102.5, 102.5, 102.5, 76.875)

  // Verify it contains two subpaths (M ... Z M ... Z)
  const subpaths = path.split('Z').map((s) => s.trim()).filter(Boolean)
  assert.equal(subpaths.length, 2)

  // Outer circle subpath
  assert.match(subpaths[0], /^M 102.50 0.00 A 102.5 102.5 0 1 1 102.50 205.00 A 102.5 102.5 0 1 1 102.50 0.00$/)

  // Inner circle subpath (counter-clockwise)
  assert.match(subpaths[1], /^M 102.50 25.63 A 76.875 76.875 0 1 0 102.50 179.38 A 76.875 76.875 0 1 0 102.50 25.63$/)
})

test('buildDonutSectors returns empty array when there are no slices or zero minutes', () => {
  assert.deepEqual(buildDonutSectors([]), [])
  assert.deepEqual(buildDonutSectors([{ label: 'Focus', minutes: 0, color: '#35c3a2' }]), [])
})

test('buildDonutSectors fills the entire circle when there is only one category', () => {
  const slices: DonutSlice[] = [
    { label: '工作', minutes: 45, color: '#35c3a2' },
  ]

  const sectors = buildDonutSectors(slices)
  assert.equal(sectors.length, 1)
  assert.equal(sectors[0].color, '#35c3a2')
  assert.equal(sectors[0].path, fullRingPath(102.5, 102.5, 102.5, 102.5 * 0.75))
})

test('buildDonutSectors ignores zero-minute slices and treats a single positive slice as a full ring', () => {
  const slices: DonutSlice[] = [
    { label: '工作', minutes: 30, color: '#35c3a2' },
    { label: '学习', minutes: 0, color: '#f59e0b' },
  ]

  const sectors = buildDonutSectors(slices)
  assert.equal(sectors.length, 1)
  assert.equal(sectors[0].color, '#35c3a2')
  assert.equal(sectors[0].path, fullRingPath(102.5, 102.5, 102.5, 102.5 * 0.75))
})

test('buildDonutSectors proportions multiple categories by total recorded time', () => {
  const slices: DonutSlice[] = [
    { label: '工作', minutes: 60, color: '#35c3a2' },
    { label: '学习', minutes: 30, color: '#f59e0b' },
  ]

  const sectors = buildDonutSectors(slices)
  assert.equal(sectors.length, 2)
  assert.equal(sectors[0].color, '#35c3a2')
  assert.equal(sectors[1].color, '#f59e0b')

  // Both should have valid sectorPath commands (M ... A ... L ... A ... Z)
  for (const sector of sectors) {
    assert.match(sector.path, /^M [\d.]+ [\d.]+ A [\d.]+ [\d.]+ 0 [01] 1 [\d.]+ [\d.]+ L [\d.]+ [\d.]+ A [\d.]+ [\d.]+ 0 [01] 0 [\d.]+ [\d.]+ Z$/)
  }
})
