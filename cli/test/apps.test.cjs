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
