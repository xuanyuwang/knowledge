// Read-only replay of Director's actual attribute and conversation-stat transformers.
// User/group model transforms are identity stubs: this isolates list selection and count preservation.
const fs = require('node:fs');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const ts = require('/Users/xuanyu.wang/repos/director/node_modules/typescript');
const source = fs.readFileSync('/Users/xuanyu.wang/repos/director/packages/director-api/src/services/cresta-api/insights/transformers.ts', 'utf8');
const ast = ts.createSourceFile('transformers.ts', source, ts.ScriptTarget.Latest, true);
const names = new Set(['transformAttributes', 'transformConversationStatsResponse']);
const functions = ast.statements.filter((node) => ts.isFunctionDeclaration(node) && names.has(node.name?.text));
assert.equal(functions.length, 2);
const extracted = functions.map((node) => node.getText(ast).replace(/^export /, '')).join('\n');
const sandbox = {
  first: (values) => values?.[0],
  transformUsers: (values) => values?.[0],
  transformApiGroupToModel: (value) => value,
  transformApiTeamToModel: (value) => value,
  transformAverageHandleTime: (value) => value,
};
vm.createContext(sandbox);
vm.runInContext(ts.transpileModule(extracted, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, sandbox);
const cases = [
  { user: 'Sarah Young', groups: [
    ['01a0fce6-f365-7ac6-9446-600b750bac65', 'Brittni Howard', 'AGENT'],
    ['01a0fce6-f365-7a82-860a-5e3cdcc90bd2', 'Christina Gusby', 'MANAGER_2ND'],
  ], expected: 'Christina Gusby' },
  { user: 'Allison Nesbitt', groups: [
    ['01a0fce6-f365-7ab8-b55a-f08959334572', 'Miranda Adcock', 'AGENT,MANAGER_2ND'],
    ['01a0fce6-f365-7ac6-9446-600b750bac65', 'Brittni Howard', 'MANAGER_2ND'],
    ['01a0fce6-f365-7aa3-955f-2cfaf3d52767', 'Silvia Martin', 'MANAGER_2ND'],
  ], expected: 'Silvia Martin' },
];
for (const item of cases) {
  // Match buildUserGroupMappings sorting of full resource names (same customer prefix).
  const groups = item.groups.map(([id, displayName]) => ({ name: `customers/advent-health/groups/${id}`, displayName })).sort((a, b) => a.name < b.name ? -1 : a.name > b.name ? 1 : 0);
  const input = { totalConversationCount: 17, conversationStatsResults: [{ totalConversationCount: 17, attribute: { users: [{ name: item.user }], groups } }] };
  const result = sandbox.transformConversationStatsResponse(input);
  assert.equal(result.resultGroups[0].groupedByAttributes.group.displayName, item.expected);
  assert.equal(result.totalConversationCount, 17);
  assert.equal(result.resultGroups[0].totalConversationCount, 17);
  assert.equal(result.resultGroups.length, 1);
  const reversed = sandbox.transformAttributes({ groups: [...groups].reverse() });
  assert.equal(reversed.group.displayName, groups.at(-1).displayName);
  console.log(`${item.user}: selected ${item.expected}; one row; synthetic count 17 preserved; reversing groups changes label`);
}
console.log('PASS: actual Director selection/count transformers; backend sort modeled from source; counts are synthetic, not customer metrics.');
