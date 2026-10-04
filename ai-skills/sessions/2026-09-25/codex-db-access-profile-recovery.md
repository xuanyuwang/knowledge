# DB access skill profile recovery

- Source repo: `/Users/xuanyu.wang/repos/coaching-qm-skills`.
- Isolated worktree: `/Users/xuanyu.wang/repos/coaching-qm-skills-db-access`, branch `xw/db-access-profile-recovery`, based on `origin/main` `98fc17576e87bb27334a076b6f405ea08bd8f3a4`.
- Commit: `5b555751c911836a538c272b1f09456408a7acb6`.
- PR: [coaching-qm-skills#8](https://github.com/cresta/coaching-qm-skills/pull/8), titled `CONVI-7657 Improve customer DB access skill recovery`. No repository or organization PR template was found. Initial checks: Codeowner, donotmerge, and linear-ticket passed; CodeRabbit pending.
- Config evidence: `config` `origin/master` `e0d6dd6dfdbcc8e1ac03114159da300e19cc65b7`, Comcast app cluster/account/database `comcast-prod/comcast-prod/comcast`, auth cluster `auth-comcast-prod`, AWS account `233896340203`.

## Changes

- App and auth PG launchers now detect a missing selected AWS SSO profile and print the targeted `cresta-cli setup-aws-profiles --only-account-name <account>` and `aws sso login --profile <account>_dev` commands. They stop so the newly generated profile can be inspected before use.
- PG launchers accept `--allow-unlisted-account` only for a config-derived AWS Secrets Manager ARN whose account ID matches the selected SSO profile, then pass `cresta-cli --force --profile` with `--read-only` retained. `--refresh-cli-cache` maps to `--clear-cache` for a stale profile list.
- Auth PG derives dedicated `auth-<account>-prod` clusters to the matching account, including `auth-comcast-prod` → `comcast-prod`.
- ClickHouse launcher distinguishes an absent profile from an expired SSO session and prints targeted setup steps.
- All three launchers discard ambient AWS credential environment variables and use SSO config profiles without loading the shared AWS credentials file. Skills require manual clearance of each selected profile and customer access/location checks.

## Validation

- `zsh -n` passed for all three launchers.
- `quick_validate.py` passed for all three `SKILL.md` files using an isolated `uv --with pyyaml` runtime.
- Live app PG `SELECT 1` passed with `--allow-unlisted-account --refresh-cli-cache`; no URI printed.
- With empty AWS config and credentials files, app PG, auth PG, and ClickHouse stopped with the precise single-account setup command. Auth PG resolved account `comcast-prod` and cluster `auth-comcast-prod` before stopping.
- No live auth DB connection was attempted. ClickHouse's normal connection path was previously verified in the same task; this edit only changes its missing-profile guard.

## Follow-up

- Consider provisioning a read-only ClickHouse principal so diagnostic queries need not use the admin secret.
- Consider an explicit resolve-only mode that reports config-derived and live-validated non-secret endpoints without opening an interactive DB client.

## PR review follow-up

- Verified two inline findings against the current PR code: app and auth PG launchers required `aws` before choosing AWS versus GCP; all three launchers left inherited `AWS_CONFIG_FILE` in place.
- Commit `a123a28` moves the PG `aws` dependency check into each AWS provider branch and unsets `AWS_CONFIG_FILE` with the other ambient AWS variables in all three launchers. Pushed to [coaching-qm-skills#8](https://github.com/cresta/coaching-qm-skills/pull/8).
- `zsh -n`, `git diff --check`, and skill validation passed. A local config fixture with command stubs verified both GCP paths reach `cresta-cli` without AWS installed, both AWS PG paths require AWS only after provider selection, and all three AWS paths clear `AWS_CONFIG_FILE` before profile lookup. No live DB call was made for this review fix.
