const { test } = require('node:test');
const assert = require('node:assert/strict');
const { join } = require('node:path');
const { temporary } = require('./helpers.cjs');
const { saveConfig, resolveAuth } = require('../dist/config');

test('saved credentials stay bound to their origin and explicit credentials carry no stale identity', t => {
  process.env.SHIPPER_CONFIG = join(temporary(t, 'shipper-auth-test-'), 'config.json');
  delete process.env.SHIPPER_URL;
  delete process.env.SHIPPER_TOKEN;
  saveConfig({ url: 'https://one.example.test', token: 'a'.repeat(64), email: 'owner@one.test', tokenId: 'one' });
  assert.equal(resolveAuth().email, 'owner@one.test');
  assert.equal(resolveAuth({ url: 'https://one.example.test/' }).token, 'a'.repeat(64));
  for (const url of ['https://two.example.test', 'http://one.example.test', 'https://one.example.test:8443']) {
    assert.throws(() => resolveAuth({ url }), /own credentials/);
    process.env.SHIPPER_URL = url;
    assert.throws(() => resolveAuth(), /own credentials/);
  }
  const explicit = resolveAuth({ url: 'https://two.example.test', token: 'b'.repeat(64) });
  assert.equal(explicit.token, 'b'.repeat(64));
  assert.equal(explicit.email, undefined);
  assert.equal(explicit.tokenId, undefined);
  process.env.SHIPPER_TOKEN = 'c'.repeat(64);
  assert.equal(resolveAuth().token, 'c'.repeat(64));
});
