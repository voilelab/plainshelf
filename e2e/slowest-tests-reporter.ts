import { appendFileSync } from 'node:fs';
import { basename, relative } from 'node:path';
import type { Reporter, TestCase, TestResult } from '@playwright/test/reporter';

const TOP_N = 10;

interface TimedTest {
  name: string;
  file: string;
  duration: number;
  attempts: number;
}

function formatMs(ms: number): string {
  return ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${Math.round(ms)}ms`;
}

function attemptNote(test: TimedTest): string {
  return test.attempts > 1 ? ` (${test.attempts} attempts)` : '';
}

export default class SlowestTestsReporter implements Reporter {
  private readonly timingsByTestId = new Map<string, TimedTest>();

  onTestEnd(test: TestCase, result: TestResult): void {
    // Playwright 1.62.1 reports onTestEnd for each attempt under the same test ID.
    const previous = this.timingsByTestId.get(test.id);
    if (previous) {
      previous.duration += result.duration;
      previous.attempts += 1;
      return;
    }
    // Playwright 1.62.1 TestCase.titlePath includes its root, project and file suites (lib/common/index.js).
    const redundant = new Set(['', test.parent.project()?.name, basename(test.location.file)]);
    this.timingsByTestId.set(test.id, {
      name: test
        .titlePath()
        .filter((segment) => !redundant.has(segment))
        .join(' > '),
      file: relative(process.cwd(), test.location.file),
      duration: result.duration,
      attempts: 1
    });
  }

  onEnd(): void {
    const all = [...this.timingsByTestId.values()];
    if (all.length === 0) return;

    const slowest = [...all].sort((a, b) => b.duration - a.duration).slice(0, TOP_N);
    const total = all.reduce((sum, test) => sum + test.duration, 0);
    const heading = `Slowest ${slowest.length} of ${all.length} E2E tests`;

    process.stdout.write(`\n${heading} (${formatMs(total)} in test bodies):\n`);
    for (const test of slowest) {
      process.stdout.write(
        `  ${formatMs(test.duration).padStart(8)}${attemptNote(test)}  ${test.file} > ${test.name}\n`
      );
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
          (test) =>
            `| ${formatMs(test.duration)}${attemptNote(test)} | \`${test.file}\` | ${test.name.replaceAll('|', '\\|')} |`
        ),
        '',
        ''
      ].join('\n')
    );
  }
}
