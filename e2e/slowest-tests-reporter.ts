/**
 * Prints the ten slowest E2E specs, to stdout and to the GitHub job summary.
 *
 * The counterpart of `frontend/scripts/slowest-tests-reporter.mjs`, kept as its
 * own copy because the two packages share no dependency tree. The list reporter
 * already prints a duration per case, but it prints them in run order across a
 * few hundred lines; PSW-76 wants the expensive end visible at the tail of the
 * log.
 */
import { appendFileSync } from 'node:fs';
import { basename, relative } from 'node:path';
import type { Reporter, TestCase, TestResult } from '@playwright/test/reporter';

const TOP_N = 10;

interface TimedTest {
  name: string;
  file: string;
  /** Summed over every attempt: a retry is time the job spent too. */
  duration: number;
  attempts: number;
}

function formatMs(ms: number): string {
  return ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${Math.round(ms)}ms`;
}

/** Says why a duration is large, so a retried test is not read as a slow one. */
function attemptNote(test: TimedTest): string {
  return test.attempts > 1 ? ` (${test.attempts} attempts)` : '';
}

export default class SlowestTestsReporter implements Reporter {
  /** Keyed by test id: a retried case reports once per attempt. */
  private readonly tests = new Map<string, TimedTest>();

  onTestEnd(test: TestCase, result: TestResult): void {
    // CI retries twice, and `onTestEnd` fires once per attempt under the same
    // test id. Charging the test only its slowest attempt would report three
    // one-second tries as one second — hiding retry cost exactly when flakiness
    // is what made the job long.
    const previous = this.tests.get(test.id);
    if (previous) {
      previous.duration += result.duration;
      previous.attempts += 1;
      return;
    }
    // titlePath() leads with the root, the project and the spec file; the file
    // is already its own column and there is only ever one project here.
    const redundant = new Set(['', test.parent.project()?.name, basename(test.location.file)]);
    this.tests.set(test.id, {
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
    const all = [...this.tests.values()];
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
