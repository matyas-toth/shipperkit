const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const { join } = require('node:path');
const { temporary } = require('./helpers.cjs');
const tar = require('tar');
const { archive, deployLocal } = require('../dist/local');

function project(t) {
  const root = temporary(t, 'shipper-source-test-');
  fs.writeFileSync(join(root, 'index.html'), 'local version two');
  return root;
}

test('source selection honors nested gitignore and shipperignore and excludes credential files', async t => {
  const root = project(t);
  fs.mkdirSync(join(root, 'nested'));
  for (const name of ['.npmrc', '.netrc', '.pypirc', '.git-credentials', '.env.local', 'private.key']) fs.writeFileSync(join(root, name), 'DUMMY_SECRET');
  fs.writeFileSync(join(root, '.env.example'), 'TOKEN=');
  fs.writeFileSync(join(root, '.yarnrc.yml'), 'nodeLinker: node-modules\n');
  fs.writeFileSync(join(root, '.gitignore'), '*.log\n!keep.log\n');
  fs.writeFileSync(join(root, '.shipperignore'), 'output/\n');
  fs.writeFileSync(join(root, 'debug.log'), 'private log');
  fs.writeFileSync(join(root, 'keep.log'), 'keep');
  fs.writeFileSync(join(root, 'nested', '.gitignore'), 'private.txt\n');
  fs.writeFileSync(join(root, 'nested', 'private.txt'), 'private');
  fs.writeFileSync(join(root, 'nested', 'public.txt'), 'public');
  fs.mkdirSync(join(root, 'output'));
  fs.writeFileSync(join(root, 'output', 'bundle.js'), 'generated');
  const source = await archive(root);
  const names = [];
  await new Promise((resolve, reject) => tar.t({ onReadEntry: entry => names.push(entry.path) }).on('end', resolve).on('error', reject).end(source.body));
  assert.ok(names.includes('index.html'));
  assert.ok(names.includes('.env.example'));
  assert.ok(names.includes('.yarnrc.yml'), 'ordinary package-manager build settings remain available');
  assert.ok(names.includes('keep.log'));
  assert.ok(names.includes('nested/public.txt'));
  for (const name of ['.npmrc', '.netrc', '.pypirc', '.git-credentials', '.env.local', 'private.key', 'debug.log', 'nested/private.txt', 'output/bundle.js']) assert.ok(!names.includes(name), name);
});

test('existing local application update uploads a bound snapshot and applies its reviewed revision', async t => {
  const root = project(t);
  const calls = [];
  const app = { id: 'local-app', name: 'Website', method: 'static', config: { method: 'static', localSource: 'original', publishDirectory: 'public', variables: [{ name: 'TOKEN', secret: true, keep: true }], memoryMb: 256 } };
  const client = {
    get: async path => path === '/applications' ? { items: [app] } : { application: app },
    upload: async (path, body) => { calls.push({ path, bytes: body.length }); return { id: 'replacement' }; },
    post: async (path, body) => {
      calls.push({ path, body });
      return path.endsWith('/changes') ? { revision: 'reviewed-inputs' } : { id: 'deployment', state: 'succeeded', operation: 'apply' };
    },
  };
  await deployLocal(client, root, { app: 'Website', noCache: true }, true);
  assert.match(calls[0].path, /application=local-app/);
  assert.ok(calls[0].bytes > 0);
  assert.equal(calls[1].path, '/applications/local-app/changes');
  assert.deepEqual(calls[1].body.config, { ...app.config, localSource: 'replacement' });
  assert.equal(calls[2].path, '/applications/local-app/deployments');
  assert.equal(calls[2].body.revision, 'reviewed-inputs');
  assert.equal(calls[2].body.noCache, true);
  assert.equal(app.config.localSource, 'original', 'original release remains immutable');
});

test('local deployment does not overwrite a same-name application or convert a Git application implicitly', async t => {
  const root = project(t);
  const name = require('node:path').basename(root);
  const client = { get: async path => path === '/applications' ? { items: [{ id: 'git', name }] } : { application: { id: 'git', config: { repository: 'https://example.test/source' } } }, upload: async () => { throw new Error('must not upload'); } };
  await assert.rejects(deployLocal(client, root, {}, true), /--app/);
  await assert.rejects(deployLocal(client, root, { app: name }, true), /created from local files/);
  await assert.rejects(deployLocal(client, root, { app: 'missing' }, true), /not found/);
});
