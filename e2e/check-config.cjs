const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');

function listProjects(browsers) {
  const env = { ...process.env };
  if (browsers === undefined) delete env.E2E_BROWSERS;
  else env.E2E_BROWSERS = browsers;
  return spawnSync(process.execPath, [
    require.resolve('@playwright/test/cli'),
    'test', '--config', 'playwright.config.ts', '--list', '--reporter=json'
  ], { cwd: __dirname, env, encoding: 'utf8', timeout: 15_000 });
}

for (const value of ['constructor', '__proto__', 'unknown', '', ', ,']) {
  const result = listProjects(value);
  assert.ifError(result.error);
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /E2E_BROWSERS: (unknown browser|no browser selected)/);
}

for (const [value, expected] of [
  [undefined, ['chromium']],
  [' chromium, webkit ', ['chromium', 'webkit']]
]) {
  const result = listProjects(value);
  assert.ifError(result.error);
  assert.equal(result.status, 0, result.stderr);
  const report = JSON.parse(result.stdout);
  assert.deepEqual(report.config.projects.map((project) => project.name), expected);
  assert.deepEqual(report.errors, []);
}

console.log('Browser config OK (7 selections).');
