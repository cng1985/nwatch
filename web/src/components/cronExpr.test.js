import assert from 'node:assert/strict'
import test from 'node:test'
import { canonicalExpr, parseExpression, replaceField } from './cronExpr.js'

test('parses presets and aliases into five fields', () => {
  const daily = parseExpression('@daily')
  assert.equal(daily.map((field) => field.mode).join(','), 'pick,pick,every,every,every')
  assert.equal(canonicalExpr('@hourly'), '0 * * * *')
  assert.equal(canonicalExpr('0 9 * * MON'), '0 9 * * 1')
  assert.equal(canonicalExpr('0 0 1 JAN *'), '0 0 1 1 *')
  assert.equal(canonicalExpr('0 0 * * 7'), '0 0 * * 0')
})

test('keeps ranges, steps, and lists', () => {
  const ranged = parseExpression('1-5 0 1-10/2 JAN-MAR 1,3,5')
  assert.equal(ranged[0].mode, 'range')
  assert.equal(ranged[2].mode, 'step')
  assert.deepEqual(ranged[2], expectStep(1, 10, 2))
  assert.equal(ranged[3].mode, 'range')
  assert.deepEqual(ranged[4].values, [1, 3, 5])
  assert.equal(canonicalExpr('*/15 * * * *'), '*/15 * * * *')
  assert.equal(canonicalExpr('10-40/10 * * * *'), '10-40/10 * * * *')
})

test('replaces one field and leaves the others', () => {
  assert.equal(replaceField('0 2 * * *', 'minute', { mode: 'step', start: 0, end: 59, step: 10 }), '*/10 2 * * *')
  assert.equal(replaceField('0 9 * * 1', 'dow', { mode: 'pick', values: [1, 5] }), '0 9 * * 1,5')
  assert.equal(replaceField('@weekly', 'hour', { mode: 'pick', values: [8] }), '0 8 * * 0')
  assert.equal(replaceField('not cron', 'minute', { mode: 'every' }), '* * * * *')
  assert.equal(replaceField('0 2 * * *', 'hour', { mode: 'pick', values: [] }), '0 2 * * *')
})

function expectStep(start, end, step) {
  return { raw: '1-10/2', mode: 'step', start, end, step, values: [] }
}
