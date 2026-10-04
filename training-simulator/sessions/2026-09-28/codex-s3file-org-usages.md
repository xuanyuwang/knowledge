# GitHub organization usage search: GenerateTempUploadURI

- Date: 2026-09-28, America/Toronto
- Primary source repo: `/Users/xuanyu.wang/repos/cresta-proto`
- Worktree/branch context: `/Users/xuanyu.wang/repos/cresta-proto-convi-7707-s3file`, `xw/convi-7707-s3-file-lifecycle` (read-only)
- Query: GitHub REST legacy code search `GenerateTempUploadURI org:cresta`, 100-result page, full single page; `total_count=68`, `incomplete_results=false`. Search of exact HTTP route `s3file:generateTempUploadURI org:cresta` returned 10 matching files. Search is of GitHub-indexed code, principally default branches; not every branch or runtime traffic.

## Counts

- 68 matching files across 8 repos; 47 under `cresta-proto/gen/`, 21 other files including source, tests, documentation, monitoring and schema references. File count is not invocation count.
- Four handwritten workflow entry points invoke the upload-preparation RPC: two Director hooks through one shared Director API wrapper, one direct Go client call in `external-cli`, and one direct HTTP call in `cresta-agent-custom-code-example` GitHub Action. If counting source-level invocation expressions, there are five: the two hooks, the wrapper's generated-client call, CLI client call, and Action curl route. The Action is an example integration, not evidence of production traffic.
- Director hooks: https://github.com/cresta/director/blob/HEAD/packages/director-app/src/hooks/s3file/useUploadTempFileToS3.ts#L46 and https://github.com/cresta/director/blob/HEAD/packages/director-app/src/hooks/s3file/useUploadFileToSyntheticConversationsFolder.ts#L36 ; wrapper https://github.com/cresta/director/blob/HEAD/packages/director-api/src/services/cresta-api/s3file/s3fileApi.ts#L24-L30 .
- Direct CLI: https://github.com/cresta/external-cli/blob/HEAD/cmd/customcode.go#L237 , uploads a custom-code bundle ZIP after requesting a URL.
- Example Action: https://github.com/cresta/cresta-agent-custom-code-example/blob/HEAD/.github/actions/deploy-custom-code-bundle/action.yaml#L124 , calls the HTTP route directly.
- Other handwritten matches are the backend handler/tests, speechlab web comments documenting a returned path/URI but not calling the RPC in those matched files, Python AI service text, Datadog mapping, proto/docs/test references. No other actual call was found among the GitHub-indexed matching files.

## Practical implication

The existing RPC has at least the Director flows and CLI caller to preserve if repurposed. The example Action also uses the current HTTP route/response. This strengthens the case for preserving legacy behavior if adding a separate namespace-scoped upload/commit flow. It does not measure runtime request volume or prove an exhaustive organization-wide call graph: legacy GitHub code search can miss dynamic callers or unindexed/non-default branches.

GitHub CLI/HTTPS token from keyring used only for read-only GitHub search and Contents API. No AWS/Okta/Azure/SSH credentials used. No source or PR changes.
