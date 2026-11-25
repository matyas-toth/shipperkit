const { test } = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const { execFile } = require('node:child_process');
const { promisify } = require('node:util');
const { join } = require('node:path');
const { temporary } = require('./helpers.cjs');

test('open prints the configured protocol and primary domain through the real CLI', async t => {
  const directory = temporary(t, 'shipper-application-url-');
  let application;
  const server = http.createServer((request, response) => {
    assert.equal(request.url, '/api/v1/applications');
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify({ items: [application] }));
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  t.after(() => new Promise(resolve => server.close(resolve)));
  const environment = {
    ...process.env,
    SHIPPER_URL: `http://127.0.0.1:${server.address().port}`,
    SHIPPER_TOKEN: 'a'.repeat(64),
    SHIPPER_CONFIG: join(directory, 'config.json'),
  };
  const run = promisify(execFile);
  for (const scenario of [
    { domains: [{ hostname: 'plain.example.com', https: false }], expected: 'http://plain.example.com' },
    { domains: [{ hostname: 'secure.example.com', https: true }], expected: 'https://secure.example.com' },
    { domains: [{ hostname: 'older.example.com' }], expected: 'https://older.example.com' },
    { domains: [{ hostname: 'secondary.example.com', https: true }, { hostname: 'primary.example.com', primary: true, https: false }], expected: 'http://primary.example.com' },
    { config: { domain: 'legacy.example.com', https: true }, expected: 'https://legacy.example.com' },
    { config: { domain: 'legacy.example.com', https: false }, expected: 'http://legacy.example.com' },
    { domains: [{ hostname: 'website.localhost', https: false }], expected: 'http://website.localhost' },
  ]) {
    application = { id: 'website', name: 'Website', domains: scenario.domains, config: scenario.config };
    const result = await run(process.execPath, [join(__dirname, '..', 'dist', 'index.js'), 'open', 'Website'], {
      env: environment,
      timeout: 10000,
      windowsHide: true,
    });
    assert.equal(result.stdout.trim(), scenario.expected);
  }
});

test('apps shows the operation in progress instead of a missing container', async t => {
  const directory = temporary(t, 'shipper-application-state-');
  const now = Math.floor(Date.now() / 1000);
  const items = [
    { id: 'blog', name: 'Blog', state: 'missing', latestDeployment: { id: 'd1', operation: 'deploy', state: 'building', createdAt: now - 11 } },
    { id: 'store', name: 'Store', state: 'missing', activeDeploymentId: 'd2', latestDeployment: { id: 'd2', operation: 'deploy', state: 'succeeded', createdAt: now - 90, finishedAt: now - 60 } },
    { id: 'draft', name: 'Draft', state: 'missing' },
  ];
  const server = http.createServer((request, response) => {
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify({ items }));
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  t.after(() => new Promise(resolve => server.close(resolve)));
  const { stdout } = await promisify(execFile)(process.execPath, [join(__dirname, '..', 'dist', 'index.js'), 'apps'], {
    env: { ...process.env, NO_COLOR: '1', SHIPPER_URL: `http://127.0.0.1:${server.address().port}`, SHIPPER_TOKEN: 'a'.repeat(64), SHIPPER_CONFIG: join(directory, 'config.json') },
    timeout: 10000,
    windowsHide: true,
  });
  const row = name => stdout.split('\n').find(line => line.includes(name)) || '';
  assert.match(row('Blog'), /building/);
  assert.match(row('Blog'), /in progress/);
  assert.match(row('Store'), /unavailable/);
  assert.match(row('Draft'), /not deployed/);
  assert.doesNotMatch(stdout, /missing/);
});
