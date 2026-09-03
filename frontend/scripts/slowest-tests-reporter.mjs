/**
 * Prints the ten slowest cases of a Vitest run, to stdout and to the GitHub job
 * summary.
 *
 * PSW-76 gave every CI job a timeout that fails the build. A cap alone only says
 * *that* the suite got slower; this says *where*, without downloading an
 * artifact or re-running anything locally. Vitest's own `slowTestThreshold`
 * marks slow cases inline, which is unreadable at 1,400 cases — a fixed top ten
 * stays the same size however far the suite grows.
 */
import { appendFileSync } from 'node:fs';
import { relative } from 'node:path';

const TOP_N = 10;

/** Milliseconds, in whichever of the two units reads without arithmetic. */
function formatMs(ms) {
  return ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${Math.round(ms)}ms`;
}

export default class SlowestTestsReporter {
  /** @type {{ name: string, file: string, duration: number }[]} */
  #tests = [];

  onTestCaseResult(testCase) {
    const duration = testCase.diagnostic()?.duration;
    if (typeof duration !== 'number') return;
    this.#tests.push({
      name: testCase.fullName,
      file: relative(process.cwd(), testCase.module.moduleId),
      duration
    });
  }

  onTestRunEnd() {
    if (this.#tests.length === 0) return;

    const slowest = [...this.#tests].sort((a, b) => b.duration - a.duration).slice(0, TOP_N);
    // Test bodies only: the wall clock of the job also carries collection,
    // transform and environment setup, which no single case can be charged for.
    const total = this.#tests.reduce((sum, test) => sum + test.duration, 0);
    const heading = `Slowest ${slowest.length} of ${this.#tests.length} frontend unit tests`;

    process.stdout.write(`\n${heading} (${formatMs(total)} in test bodies):\n`);
    for (const test of slowest) {
      process.stdout.write(`  ${formatMs(test.duration).padStart(8)}  ${test.file} > ${test.name}\n`);
    }
    process.stdout.write('\n');

    const summaryPath = process.env.GITHUB_STEP_SUMMARY;
    if (!summaryPath) return;
    appendFileSync(
      summaryPath,
      [
        `### ${heading}`,
        '',
        `${formatMs(total)} spent in test bodies.`,
        '',
        '| Duration | File | Test |',
        '|---:|---|---|',
        ...slowest.map(
          (test) => `| ${formatMs(test.duration)} | \`${test.file}\` | ${test.name.replaceAll('|', '\\|')} |`
        ),
        '',
        ''
      ].join('\n')
    );
  }
}
