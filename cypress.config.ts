import { mkdirSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";

import { defineConfig } from "cypress";

type CypressRun = CypressCommandLine.CypressRunResult;
type CypressFailedRun = CypressCommandLine.CypressFailedRunResult;

const reportDirectory = resolve("reports/cypress");

function percentage(numerator: number, denominator: number) {
  return denominator === 0 ? 0 : Number(((numerator / denominator) * 100).toFixed(2));
}

function seconds(milliseconds: number) {
  return `${(milliseconds / 1_000).toFixed(2)} s`;
}

function escapeTableCell(value: string) {
  return value.replaceAll("|", "\\|").replaceAll("\n", " ");
}

function writeCypressReport(results: CypressRun | CypressFailedRun) {
  mkdirSync(reportDirectory, { recursive: true });

  if (results.status === "failed") {
    const report = {
      generatedAt: new Date().toISOString(),
      status: "failed-to-run",
      failures: results.failures,
    };
    writeFileSync(resolve(reportDirectory, "results.json"), `${JSON.stringify(report, null, 2)}\n`);
    writeFileSync(
      resolve(reportDirectory, "summary.md"),
      `# Cypress QA report\n\nCypress не смог начать или завершить прогон. Подробности сохранены в \`results.json\`.\n`,
    );
    return;
  }

  const scenarios = results.runs.flatMap((run) =>
    run.tests.map((test) => ({
      spec: run.spec.relative,
      title: test.title.join(" › "),
      state: test.state,
      durationMs: test.duration ?? 0,
      attempts: test.attempts.length,
    })),
  );
  const executed = results.totalPassed + results.totalFailed;
  const report = {
    generatedAt: new Date().toISOString(),
    status: results.totalFailed === 0 ? "passed" : "failed",
    metrics: {
      discoveredScenarios: results.totalTests,
      executedScenarios: executed,
      scenarioExecutionCoveragePercent: percentage(executed, results.totalTests),
      passed: results.totalPassed,
      failed: results.totalFailed,
      pending: results.totalPending,
      skipped: results.totalSkipped,
      passRatePercent: percentage(results.totalPassed, executed),
      totalDurationMs: results.totalDuration,
    },
    specs: results.runs.map((run) => ({
      spec: run.spec.relative,
      tests: run.stats.tests,
      passes: run.stats.passes,
      failures: run.stats.failures,
      pending: run.stats.pending,
      skipped: run.stats.skipped,
      durationMs: run.stats.duration,
    })),
    scenarios,
  };

  writeFileSync(resolve(reportDirectory, "results.json"), `${JSON.stringify(report, null, 2)}\n`);

  const scenarioRows = scenarios
    .map(
      (scenario) =>
        `| ${escapeTableCell(scenario.spec)} | ${escapeTableCell(scenario.title)} | ${scenario.state} | ${seconds(scenario.durationMs)} | ${scenario.attempts} |`,
    )
    .join("\n");
  const markdown = `# Cypress QA report

Сформирован: ${report.generatedAt}

## Итог

| Метрика | Значение |
| --- | ---: |
| Покрытие исполнения UI-сценариев | ${report.metrics.scenarioExecutionCoveragePercent}% (${executed}/${results.totalTests}) |
| Успешно | ${results.totalPassed} |
| Ошибки | ${results.totalFailed} |
| Пропущено / ожидает | ${results.totalSkipped + results.totalPending} |
| Доля успешных среди выполненных | ${report.metrics.passRatePercent}% |
| Общее время Cypress | ${seconds(results.totalDuration)} |

> Покрытие исполнения UI-сценариев показывает, какая доля обнаруженных Cypress-сценариев была выполнена. Это не покрытие строк и ветвей приложения; оно формируется отдельно командой \`npm run test:coverage\`.

## Сценарии

| Spec | Сценарий | Статус | Время | Попытки |
| --- | --- | --- | ---: | ---: |
${scenarioRows || "| — | Сценарии не обнаружены | — | 0.00 s | 0 |"}
`;
  writeFileSync(resolve(reportDirectory, "summary.md"), markdown);
}

export default defineConfig({
  allowCypressEnv: false,
  e2e: {
    baseUrl: "http://127.0.0.1:3100",
    specPattern: "cypress/e2e/**/*.cy.js",
    supportFile: "cypress/support/e2e.js",
    setupNodeEvents(on) {
      on("after:run", writeCypressReport);
    },
  },
  retries: { openMode: 0, runMode: 1 },
  screenshotOnRunFailure: true,
  video: false,
});
