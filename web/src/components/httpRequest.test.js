import assert from 'node:assert/strict'
import test from 'node:test'
import { encodeForm, headersToText, parsePairs, pairsToText, previewBody, textToHeaders } from './httpRequest.js'

test('parses header lines and skips blanks', () => {
  assert.deepEqual(textToHeaders('Authorization: Bearer token\n\nX-Job: cron'), {
    Authorization: 'Bearer token',
    'X-Job': 'cron',
  })
  assert.equal(headersToText({ Authorization: 'Bearer token' }), 'Authorization: Bearer token')
  assert.throws(() => textToHeaders('not a header'), /请求头写成/)
})

test('keeps form values that contain ampersands', () => {
  const text = pairsToText([{ key: 'name', value: 'a&b' }])
  assert.equal(text, 'name=a&b\n')
  assert.deepEqual(parsePairs(text), [{ key: 'name', value: 'a&b' }])
  assert.equal(encodeForm('a=1\nb=two words'), 'a=1&b=two+words')
  assert.deepEqual(parsePairs('ok=1&tenant=141'), [
    { key: 'ok', value: '1' },
    { key: 'tenant', value: '141' },
  ])
})

test('previews json and leaves invalid json unchanged', () => {
  assert.equal(previewBody('json', '{"ok":true}'), '{\n  "ok": true\n}')
  assert.equal(previewBody('json', '{'), '{')
  assert.equal(previewBody('form', 'b=2\na=1'), 'a=1&b=2')
  assert.equal(previewBody('xml', '<root/>'), '<root/>')
})
