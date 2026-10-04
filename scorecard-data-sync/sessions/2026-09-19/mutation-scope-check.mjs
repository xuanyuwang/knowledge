// Uses the installed TanStack Query implementation, no network or app changes.
import assert from 'node:assert/strict';
import { QueryClient } from '/Users/xuanyu.wang/repos/director/node_modules/@tanstack/query-core/build/modern/index.js';

const deferred = () => {
  let resolve;
  const promise = new Promise(r => { resolve = r; });
  return { promise, resolve };
};
const client = new QueryClient();
const cache = client.getMutationCache();
const events = [];
const gate = deferred();
const started = deferred();
const mutation = fn => cache.build(client, {
  scope: { id: 'saveScorecard' }, retry: false, mutationFn: fn,
});
const first = mutation(async () => {
  events.push('save-start'); started.resolve();
  await gate.promise; events.push('save-end');
}).execute();
await started.promise;
const second = mutation(async () => { events.push('submit'); }).execute();
await Promise.resolve();
assert.deepEqual(events, ['save-start']);
gate.resolve();
await Promise.all([first, second]);
assert.deepEqual(events, ['save-start', 'save-end', 'submit']);
console.log('PASS: same scope serializes already enqueued saves and submit');

// Deterministically represent a debounce callback that has not fired yet.
events.length = 0;
const pendingDebounce = () => mutation(async () => { events.push('last-edit-save'); }).execute();
await mutation(async () => { events.push('submit'); }).execute();
await pendingDebounce();
assert.deepEqual(events, ['submit', 'last-edit-save']);
console.log('PASS: scope cannot order an edit that has not entered the queue');

// A shared scope serializes failures too, but does not make submit depend on success.
events.length = 0;
const saveStarted = deferred();
const failGate = deferred();
const failing = mutation(async () => {
  saveStarted.resolve(); await failGate.promise;
  events.push('save-failed'); throw new Error('synthetic save failure');
}).execute().catch(() => {});
await saveStarted.promise;
const afterFailure = mutation(async () => { events.push('submit'); }).execute();
failGate.resolve();
await Promise.all([failing, afterFailure]);
assert.deepEqual(events, ['save-failed', 'submit']);
console.log('PASS: save-before-submit needs a success barrier, not just a shared scope');
client.clear();
