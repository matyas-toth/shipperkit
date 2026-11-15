const { test } = require('node:test');
const assert = require('node:assert/strict');
const { logs } = require('../dist/logs');

async function capture(run) {
  let output = '';
  await run(chunk => { output += chunk; });
  return output;
}

function clientFor(lines) {
  return {
    get: async () => ({ items: [{ id: 'app', name: 'Website' }] }),
    stream: async path => {
      assert.match(path, /follow=false/);
      const count = Number(new URL('http://fixture' + path).searchParams.get('tail'));
      const payload = lines.slice(-count).map(line => `event: log\ndata: ${JSON.stringify({ line })}\n\n`).join('') + 'event: end\ndata: {}\n\n';
      return new ReadableStream({ start(controller) { controller.enqueue(new TextEncoder().encode(payload)); controller.close(); } });
    },
  };
}

for (const history of [[], ['only line'], ['one', 'two', 'three', 'four']]) {
  test('finite logs finish with latest lines for history length ' + history.length, async () => {
    const output = await capture(write => logs(clientFor(history), 'Website', { lines: 2 }, true, write));
    assert.deepEqual(JSON.parse(output), { lines: history.slice(-2) });
  });
}

test('tail sets the finite snapshot size when lines is omitted', async () => {
  const output = await capture(write => logs(clientFor(['one', 'two', 'three']), 'Website', { tail: 1 }, true, write));
  assert.deepEqual(JSON.parse(output), { lines: ['three'] });
});

test('follow streams future output and releases its listener and reader when interrupted', async () => {
  const before = process.listenerCount('SIGINT');
  let aborted = false;
  let stream;
  const client = {
    get: async () => ({ items: [{ id: 'app', name: 'Website' }] }),
    stream: async (path, signal) => {
      assert.match(path, /follow=true/);
      stream = new ReadableStream({ start(controller) {
        signal.addEventListener('abort', () => { aborted = true; controller.close(); }, { once: true });
        controller.enqueue(new TextEncoder().encode('event: log\ndata: {"line":"future output"}\n\n'));
        setImmediate(() => process.emit('SIGINT'));
      } });
      return stream;
    },
  };
  const output = await capture(write => logs(client, 'Website', { follow: true }, true, write));
  assert.deepEqual(JSON.parse(output), { line: 'future output' });
  assert.equal(process.listenerCount('SIGINT'), before);
  assert.equal(aborted, true);
  assert.equal(stream.locked, false);
});

test('failed stream setup does not leak SIGINT listeners, and invalid limits never request logs', async () => {
  const before = process.listenerCount('SIGINT');
  const client = { get: async () => ({ items: [{id:'app', name:'Website'}] }), stream: async () => { throw new Error('unreachable'); } };
  await assert.rejects(logs(client, 'Website', {}), /unreachable/);
  assert.equal(process.listenerCount('SIGINT'), before);
  for (const lines of [0, -1, NaN, 501]) await assert.rejects(logs(client, 'Website', { lines }), /between 1 and 500/);
});
