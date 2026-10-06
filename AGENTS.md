## CI

Do not rename jobs in `.github/workflows/ci.yml` without updating the branch protection ruleset: required checks match on the job's display name, so a rename makes the check silently never report. Keep the `name (detail)` style for job names.

Branch protection is not enforced while the repo is private on the Free plan, so never merge a PR with failing checks.

## Testing

Storage tests use real SQLite (in-memory or a temp file), not mocks, as long as they stay fast.

## Agent skills

### Issue tracker

Issues live in GitHub Issues (via the `gh` CLI). See `docs/agents/issue-tracker.md`.

### Triage labels

Default vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
