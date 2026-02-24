// bench-comment.js — Parse benchstat output into a glanceable PR comment.
// Called by bench-compare.yml with: node scripts/bench-comment.js .perf/base.txt .perf/pr.txt

const { execSync } = require('child_process');
const fs = require('fs');

const baseFile = process.argv[2];
const prFile = process.argv[3];
const outFile = process.argv[4] || '.perf/comment.md';

const raw = execSync(`benchstat ${baseFile} ${prFile} 2>&1`, { encoding: 'utf8' });

const lines = raw.split('\n');
const results = [];
let inSecPerOpSection = false;

for (const line of lines) {
  // Parse only sec/op blocks to avoid duplicate entries from B/op and allocs/op.
  if (/\bsec\/op\b/.test(line)) {
    inSecPerOpSection = true;
    continue;
  }
  if (/\b(B\/op|allocs\/op)\b/.test(line)) {
    inSecPerOpSection = false;
    continue;
  }
  if (!inSecPerOpSection) {
    continue;
  }

  // benchstat rows can contain either:
  // - "± 5%" confidence intervals (enough samples)
  // - "± ∞" footnotes (too few samples)
  // So parse from the stable row tail: "<delta> (p=<value> ...)".
  const match = line.match(
    /^(\S+)\s+.*?\s+([~]|[+\-−]\d+(?:\.\d+)?%)\s+\(p=([\d.]+)[^)]*\)\s*(?:\S+)?\s*$/
  );
  if (!match) continue;

  const benchmarkName = match[1];
  if (benchmarkName === 'geomean') {
    continue;
  }

  const name = benchmarkName.replace(/^Benchmark/, '').replace(/-\d+$/, '');
  const change = match[2].trim().replace('−', '-');
  const pValue = parseFloat(match[3]);

  let icon, verdict, deltaDisplay;
  if (change === '~') {
    icon = '~';
    verdict = 'no change';
    deltaDisplay = '~';
  } else {
    const pct = parseFloat(change);
    if (isNaN(pct)) {
      icon = '~';
      verdict = 'no change';
      deltaDisplay = change;
    } else {
      const magnitude = `${Math.abs(pct).toFixed(2)}%`;
      if (pct < 0) {
        deltaDisplay = `${magnitude} faster`;
      } else if (pct > 0) {
        deltaDisplay = `${magnitude} slower`;
      } else {
        deltaDisplay = magnitude;
      }

      if (pct > 5 && pValue < 0.05) {
        icon = ':warning:';
        verdict = `**${magnitude} slower**`;
      } else if (pct < -5 && pValue < 0.05) {
        icon = ':rocket:';
        verdict = `**${magnitude} faster**`;
      } else {
        icon = ':white_check_mark:';
        verdict = 'within noise';
      }
    }
  }

  results.push({ icon, name, deltaDisplay, pValue, verdict });
}

let body;

if (results.length === 0) {
  body = [
    '## Benchmark Comparison',
    '',
    'No benchmark results could be parsed. Raw output:',
    '',
    '```',
    raw.trim(),
    '```',
  ].join('\n');
} else {
  const hasRegression = results.some(r => r.icon === ':warning:');
  const hasImprovement = results.some(r => r.icon === ':rocket:');

  let summary;
  if (hasRegression) {
    summary = ':warning: **Performance regression detected**';
  } else if (hasImprovement) {
    summary = ':rocket: **Performance improved**';
  } else {
    summary = ':white_check_mark: **No significant performance change**';
  }

  const table = [
    '| | Benchmark | Delta | Verdict |',
    '|---|---|---|---|',
    ...results.map(
      r => `| ${r.icon} | \`${r.name}\` | ${r.deltaDisplay} (p=${r.pValue.toFixed(3)}) | ${r.verdict} |`
    ),
  ].join('\n');

  body = [
    `## Benchmark Comparison`,
    '',
    summary,
    '',
    table,
    '',
    '<details>',
    '<summary>Raw benchstat output</summary>',
    '',
    '```',
    raw.trim(),
    '```',
    '',
    '</details>',
  ].join('\n');
}

fs.writeFileSync(outFile, body);
console.log(`Wrote comment to ${outFile}`);
