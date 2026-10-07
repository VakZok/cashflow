## Workflow

Every change goes through a pull request. Never commit directly to `main` or push to it. Work on a feature branch (e.g. `feat/<topic>`), push it, and open a PR with `gh pr create`. Before committing or starting work, run `git pull --rebase origin main` so the branch is based on the latest `main`. Local `main` should always match `origin/main`.

## CI

Do not rename jobs in `.github/workflows/ci.yml` without updating the branch protection ruleset: required checks match on the job's display name, so a rename makes the check silently never report. Keep the `name (detail)` style for job names.

Branch protection is not enforced while the repo is private on the Free plan, so never merge a PR with failing checks.

## Testing

Storage tests use real SQLite (in-memory or a temp file), not mocks, as long as they stay fast.

## Working location

Stay inside the project folder. Never create directories, worktrees, or notes outside it. Git worktrees go in `.worktrees/` (excluded via `.git/info/exclude`), agent notes in `.worktrees/notes/`. Only use `unsandboxed` for git metadata writes, never to write outside the project.

Go build cache must live inside the project under `.gocache/`. `.gocache/` is gitignored. Never request write access to outside the project.

## Editing files

Use the Edit/Write tools to change files. Do not use Python (or other scripts) for file edits.

## Agent skills

### Issue tracker

Issues live in GitHub Issues (via the `gh` CLI). See `docs/agents/issue-tracker.md`.

### Triage labels

Default vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
