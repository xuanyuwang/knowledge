const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { createRequire } = require('node:module');
const { execFileSync } = require('node:child_process');
const repo = '/Users/xuanyu.wang/repos/director-convi-7941-export-errors';
const localRequire = createRequire(path.join(repo, 'package.json'));
const ts = localRequire('typescript');
const RealJSZip = localRequire('jszip');
const sourcePath = 'packages/director-app/src/features/insights/performance/performance-progression/PerformanceProgression.tsx';
const revision = '6d5b707a21ab607378e4ed34d2a917d61f6f0efb';
const source = execFileSync('git', ['show', `${revision}:${sourcePath}`], { cwd: repo, encoding: 'utf8' });
const parsed = ts.createSourceFile(sourcePath, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
let callback;
function visit(node) {
  if (ts.isVariableDeclaration(node) && node.name.getText(parsed) === 'handleClickCsvExport') {
    assert(ts.isCallExpression(node.initializer));
    callback = node.initializer.arguments[0].getText(parsed);
  }
  ts.forEachChild(node, visit);
}
visit(parsed);
assert(callback, 'Extract the actual production callback, rather than reconstructing it');
const tick = () => new Promise(resolve => setImmediate(resolve));
function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
const failures = [];
process.on('unhandledRejection', error => failures.push(error.message));
async function run(scenario) {
  const summary = deferred(), agent = deferred(), saved = deferred();
  const events = [], handledErrors = [], saves = [];
  const summaryParams = { groupBy: ['criterion', 'time_range'] };
  const agentParams = { groupBy: ['agent', 'criterion', 'time_range'] };
  function InstrumentedZip() {
    const zip = new RealJSZip();
    const file = zip.file.bind(zip), generate = zip.generateAsync.bind(zip);
    zip.file = (...args) => { if (args.length > 1) events.push(`add:${args[0]}`); return file(...args); };
    zip.generateAsync = (...args) => { events.push(`generate:${Object.keys(zip.files).join(',')}`); return generate(...args); };
    return zip;
  }
  const request = params => {
    assert(params === summaryParams || params === agentParams);
    const name = params === summaryParams ? 'summary' : 'agent';
    events.push(`request:${name}`);
    return params === summaryParams ? summary.promise : agent.promise;
  };
  const saveAs = (blob, filename) => { events.push(`save:${filename}`); saves.push({ blob, filename }); saved.resolve(blob); };
  const factory = new Function('JSZip', 'scoreStatsForCSVExecute', 'requestParams', 'requestParamsByAgent',
    'exportCSVForPerformanceProgression', 'filtersState', 'scorecardTemplate', 'saveAs', 'handleError', 'ErrorHandlerType',
    `return (${callback});`);
  const handler = factory(InstrumentedZip, request, summaryParams, agentParams,
    (result, frequency, template, kind) => { assert.equal(frequency, 'weekly'); assert.equal(result.kind, kind); return result.csv; },
    { frequency: 'weekly' }, { template: {} }, saveAs, error => handledErrors.push(error.message), { Toast: 2, ErrorConsole: 1 });
  const response = kind => ({ qaScoreResult: { kind, csv: kind === 'by-criterion-and-date' ? 'Criterion,Overwritten\nGreeting,1\n' : 'Agent,Overwritten\nTest Agent,1\n' } });
  const rejectCount = failures.length;
  handler();
  let archive;
  if (scenario === 'summary-first') {
    summary.resolve(response('by-criterion-and-date')); await tick();
    agent.resolve(response('by-criterion-and-agent')); archive = await saved.promise;
  } else if (scenario === 'agent-first-summary-late') {
    agent.resolve(response('by-criterion-and-agent')); archive = await saved.promise;
    summary.resolve(response('by-criterion-and-date')); await tick();
  } else if (scenario === 'agent-first-summary-before-save') {
    agent.resolve(response('by-criterion-and-agent')); await Promise.resolve();
    summary.resolve(response('by-criterion-and-date')); archive = await saved.promise;
  } else if (scenario === 'summary-fails') {
    summary.reject(new Error('summary request failed')); await tick();
    agent.resolve(response('by-criterion-and-agent')); archive = await saved.promise;
  } else if (scenario === 'agent-fails') {
    summary.resolve(response('by-criterion-and-date')); await tick();
    agent.reject(new Error('agent request failed')); await tick();
  }
  await tick();
  let entries = [];
  if (archive) {
    const bytes = Buffer.from(await archive.arrayBuffer());
    const zip = await RealJSZip.loadAsync(bytes);
    entries = Object.keys(zip.files).sort();
    fs.writeFileSync(path.join(__dirname, `convi-7941-controlled-${scenario}.zip`), bytes);
    for (const entry of entries) assert.match(await zip.file(entry).async('string'), /,1\n$/);
  }
  const result = { scenario, entries, downloads: saves.length, handledErrors, unhandledErrors: failures.slice(rejectCount), events };
  if (scenario === 'summary-first') assert.deepEqual(entries, ['performance_progression.csv', 'performance_progression_by_agent.csv']);
  if (scenario.startsWith('agent-first-summary-') || scenario === 'summary-fails') assert.deepEqual(entries, ['performance_progression_by_agent.csv']);
  if (scenario === 'agent-fails') assert.equal(saves.length, 0);
  if (scenario.includes('fails')) { assert.equal(handledErrors.length, 0); assert.equal(result.unhandledErrors.length, 1); }
  return result;
}
(async () => {
  const results = [];
  for (const scenario of ['summary-first', 'agent-first-summary-late', 'agent-first-summary-before-save', 'summary-fails', 'agent-fails']) results.push(await run(scenario));
  const output = { sourceRevision: revision, sourcePath, method: 'Actual callback extracted using TypeScript AST; real JSZip; controlled request promises; CSV formatter stubbed to isolate packaging', results };
  fs.writeFileSync(path.join(__dirname, 'convi-7941-export-race-results.json'), JSON.stringify(output, null, 2) + '\n');
  console.log(JSON.stringify(output, null, 2));
})().catch(error => { console.error(error); process.exitCode = 1; });
