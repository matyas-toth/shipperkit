const { mkdtempSync, rmSync } = require('node:fs');
const { tmpdir } = require('node:os');
const { join, resolve, sep } = require('node:path');

exports.temporary = (t, prefix) => {
  const directory = mkdtempSync(join(tmpdir(), prefix));
  t.after(() => {
    if (!resolve(directory).startsWith(resolve(tmpdir()) + sep)) throw new Error('Invalid test directory');
    rmSync(directory, { recursive: true, force: true });
  });
  return directory;
};
