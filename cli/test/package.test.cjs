const { test } = require('node:test');
const assert = require('node:assert/strict');
const { execFileSync } = require('node:child_process');
const { existsSync } = require('node:fs');
const { join, resolve } = require('node:path');
const { temporary: makeTemporary } = require('./helpers.cjs');

test('the CLI tarball installs and runs outside the repository', { timeout: 120000 }, t => {
  const temporary = makeTemporary(t, 'shipper-package-test-');
  const root = resolve(__dirname, '..');
  const npm = process.env.npm_execpath;
  assert.ok(npm, 'run the packaging test through npm test');
  const [packed] = JSON.parse(execFileSync(process.execPath, [npm, 'pack', root, '--ignore-scripts', '--json', '--pack-destination', temporary], { encoding:'utf8' }));
  assert.ok(packed.files.some(file => file.path === 'dist/source-policy.json'));
  const target = join(temporary, 'installed');
  execFileSync(process.execPath, [npm, 'install', '--prefix', target, '--ignore-scripts', '--offline', '--no-audit', '--no-fund', join(temporary, packed.filename)], { stdio:'pipe', timeout:90000 });
  const bin = join(target, 'node_modules', '@shipperkit', 'cli', 'dist', 'index.js');
  assert.ok(existsSync(bin));
  const output = execFileSync(process.execPath, [bin, '--help'], { cwd:temporary, encoding:'utf8' });
  assert.match(output, /deploy/);
  assert.match(output, /--json/);
  if (process.platform !== 'win32') {
    assert.match(execFileSync(join(target, 'node_modules', '.bin', 'shipper'), ['--version'], { encoding:'utf8' }), /0\.1\.0/);
  }
});
