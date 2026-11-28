<!-- AIHUB-INVIOLABLE-LAW-PRELUDE v1 -->
# AIHUB Inviolable Law — Strict Prelude

1. Truth: never claim done/green/resolved without command, exit code, decisive output.
2. Root cause: exterminate bypass, fallback, shim, suppression, stub, hardcode, catch-based normalization, retry, compatibility, partial execution, keyring, or old+new coexistence.
3. Tracker boundary: without repository `.beads/`, invoke neither `bd` nor `gc` and create no substitute. Otherwise use only the selected, available canonical tracker. If explicitly suspended, preserve authorized Git/PR/CI evidence and never declare DONE.
4. Research first: inspect code, docs, canonical sources before acting; never invent APIs, flags, facts, or behavior.
5. Owner first: use the project's declared facades/primitives; do not reimplement them locally.
6. Gate persistence: a failure stops only that invocation. Correct its owner,
   republish, and rerun until green; never switch phase or repository because a
   check, review, approval, or merge is pending. Escalate only after every
   authorized technical action is exhausted and the remaining condition is
   genuinely external or requires new authority.
7. Landing: native gates, commit, fast-forward push, bead evidence.
8. Divergence: FF push rejected → integrate by cooperation: `git merge --no-ff` the integration base into your lane, resolve conflicts, revalidate, land. Never rebase or force-push an authorized change or integration branch; adopt all current worktree state and fix it forward.
9. Escalation: impossible rule → exact error. Rule conflict → present both with numbers. Unclear → one targeted question. Never guess.
10. Precedence: NEWEST > OLDEST. USER REQUEST > BEADS > ADRs > SKILLs > DOCS > default. Adjust lower/older to higher/newer. Doubt → ASK USER FIRST.
11. Workspaces: follow `rules/coordination/gascity.md`. Every manual task uses a dedicated Git worktree, branch, and physical `.venv`, never the primary checkout. Gas City suspension keeps orchestration inactive. Worktrees and staging stay on the destination filesystem, never `/tmp`; no borrowed environment, backup, or archive. Retire worktrees after verified integration.
12. Phase closure: keep the phase active through check repair, review resolution,
    independent approval, merge into the configured integration branch, and
    post-merge proof. Only then, with its Bead closed with evidence, is it DONE.
    When the operator states that no independent reviewer exists and authorizes
    an administrative merge, that authorization replaces the approval row alone;
    every other row stays mandatory and closure records the approval as
    operator-authorized, never as satisfied.
13. Root Make only: diagnostics, validation, generation, tests, Waza,
    publication, and deployment run only through selector-free verbs in the
    repository root Makefile; bare verbs perform their declared operation. Test
    verbs: `rules/workflow/canonical-commands.md`.
14. Red means red: a warning, skip, empty output, missing tool, missing report,
    zero collection, caught exception, retry, or normalized failure is RED. The
    only acceptable zero-execution test result is a typed `make test` cache hit
    with an integrity-checked database and complete deselection
    accounting; it is never reported as tests passed. The first exception and
    raw traceback escape unchanged.
<!-- /AIHUB-INVIOLABLE-LAW-PRELUDE -->

# AGENTS.md — gascity

> Packaged governance `agents-governance` `0.6.3` owns the capability indexes: 71 agents, 112 rules, 141 skills. Consume them through `GovernanceBundle`; do not copy their bodies here.

<!-- AIHUB-AGENTS-SCOPE-LOCAL-BEGIN -->
# Gas City

Gas City is an orchestration-builder SDK — a Go toolkit for composing
multi-agent coding workflows. It extracts the battle-tested subsystems from
Steve Yegge's Gas Town (github.com/steveyegge/gastown) into a configurable
SDK where **all role behavior is user-supplied configuration** and the SDK
provides only infrastructure. The core principle: **ZERO hardcoded roles.**
The SDK has no built-in Mayor, Deacon, Polecat, or any other role. If a
line of Go references a specific role name, it's a bug.

You can build Gas Town in Gas City, or Ralph, or Claude Code Agent Teams,
or any other orchestration pack — via specific configurations.

**Why Gas City exists:** Gas Town proved multi-agent orchestration works,
but all its roles are hardwired in Go code. Steve realized the way work is
expressed — as beads composed into formulas — was powerful enough to abstract
roles into configuration. Gas City extracts that insight into an SDK where Gas
Town becomes one configuration among many.

This file holds only what applies everywhere. Rules for a specific area live
next to that code; read the matching file before changing it.

## Current integration mission

This fork is integrating **Gas City + T3 Code + a DoltLite-backed beads
store**. The target is not a permanent divergence from upstream Gas City.
The target is a maintainable integration branch whose useful changes can
track upstream easily and whose fork-specific behavior is isolated behind
small, obvious ownership boundaries.

When working here, assume three codebases matter:

- **This repo** (`/data/projects/gascity`): the Gas City SDK and the fork
  integration layer.
- **T3 Code** (`/data/projects/t3code` when present): the UI/runtime that
  hosts visible agent threads through the `t3bridge` runtime provider.
- **Beads / bd with DoltLite**: the work ledger backend. Gas City should use
  the normal beads abstractions and keep DoltLite-specific read/write behavior
  contained in beads/provider boundaries.

### Upstream alignment rules

- Keep `upstream/main` easy to merge. Prefer new files, small adapters,
  and fork-owned packages over broad edits to upstream-owned code.
- If upstream code must change, make the patch minimal and idiomatic so it
  can be rebased, dropped, or proposed upstream cleanly.
- Do not bury T3 Code or DoltLite assumptions in generic SDK paths. Put
  provider-specific behavior behind the existing runtime, config, or beads
  backend boundaries.
- Before rebuilding a missing feature from scratch, search history. This
  fork has repeatedly lost working code during branch churn; older branches
  and commits often already contain the fix.
- Treat archived plans and audits as evidence, not gospel. Confirm against
  current code and current upstream before porting.

## Read first

| When you touch | Read first |
|---|---|
| Asked to run or try Gas City, not change it | README "Get it running" (no build) |
| `internal/api/`, `internal/events/`, `internal/extmsg/`, CLI code that constructs events or calls the API client, the OpenAPI spec, or the dashboard | `internal/api/AGENTS.md` |
| `internal/config/` (agent or rig config fields) | `internal/config/AGENTS.md` |
| `internal/session/` | `internal/session/AGENTS.md` |
| `internal/sessionlog/` | `internal/sessionlog/AGENTS.md` |
| `internal/runtime/acp/` | `internal/runtime/acp/AGENTS.md` |
| Session creation or lifecycle from `cmd/gc` or `internal/api` | `internal/worker/AGENTS.md` |
| `internal/cliauth/`, `gc login`, `gc whoami` | `internal/cliauth/AGENTS.md` |
| `scripts/`, test runners, or git hooks | `scripts/AGENTS.md` |
| Any test | `TESTING.md` |
| Anything under `docs/` | `.claude/skills/gascity-docs/SKILL.md` |
| New SDK surface or a new primitive | `engdocs/contributors/primitive-test.md` |
| A controller or session reconciler incident | `engdocs/contributors/reconciler-debugging.md` (use `gc trace`) |
| Worktree isolation or worker cleanup | `engdocs/archive/backlogs/worktree-roadmap.md` (lifecycle analysis and cleanup-bug lessons) |
| A `release-gates/*.md` deploy gate | `engdocs/contributors/release-gate-criteria-conventions.md` |
| Bazel builds | `engdocs/bazel-quickstart.md` |
| Where a package lives | `engdocs/contributors/codebase-map.md`, then the package's `go doc` |
| What is planned | `ROADMAP.md` |

## How work flows here

- **GitHub Issues is the public tracker; use an issue when it adds
  context.** Open or link one for a user-visible bug, a behavior or design
  change that needs discussion, or work that spans several pull requests:
  the issue carries the reproduction, impact, and evidence for a bug, or the
  motivation, risk, and verification plan for a change. A small,
  self-explanatory fix (a typo, a flake, a refactor, a CI or docs tweak) can
  go straight to a pull request whose description covers the why. Every pull
  request shows end-to-end evidence that the change works; when an issue
  exists, the pull request says `Closes #<issue>`. See `CONTRIBUTING.md`.
- **Agents:** when you do file an issue, use the bug or feature form fields,
  fill each from evidence, and answer `NOT_ENOUGH_INFO` where the evidence
  runs out.
- Branch from `main` (`fix/*`, `feat/*`, `refactor/*`, `docs/*`), use
  Conventional Commits, and push to your branch — never to `main`.
- A fix that changes an invariant or boundary updates the owning `AGENTS.md`
  in the same pull request.
- Maintainers also keep an internal bd ledger; it is optional and never
  required for contributions. See
  `engdocs/contributors/maintainer-environment.md`.

## Where new capability belongs

### Fork lane, process, and language boundary

This fork integrates on `dc-use` (rig `gct` in the Gas City at `~/gc`). Every change
uses a dedicated worktree and a change branch cut from `origin/dc-use`, a PR against
`dc-use`, and `merge --no-ff`; never rebase or force-push (`~/agents` ADR-0036 and
`rules/coordination/flext-gascity-roe.md`). Code here is idiomatic Go
validated by this repository's root Makefile and `TESTING.md`; no flext/Python facade
rule applies.

### Feature archaeology workflow

Use git history deliberately when a feature appears missing or regressed:

```bash
git remote -v
git fetch upstream
git log --all --oneline --decorate --grep '<keyword>'
git log --all --oneline --decorate -- <path>
git show <commit>:<path>
git diff upstream/main...HEAD -- <path>
git range-diff upstream/main...HEAD
```

Useful search targets:

- T3 bridge/runtime: `t3bridge`, `T3Bridge`, `internal/runtime/t3bridge`,
  `cmd/gc/template_resolve_t3bridge.go`
- DoltLite/beads backend: `doltlite`, `DoltLite`, `internal/beads`,
  `providers.go`, `beads_provider_lifecycle`
- Prior parity work: `engdocs/archive/analysis/gastown-upstream-audit.md`,
  `engdocs/archive/analysis/feature-parity.md`,
  `engdocs/contributors/dolt-regression-audit.md`

If history contains working code, prefer porting the smallest proven slice
instead of inventing a parallel mechanism.

Prefer the lowest rung that works: a prompt or template change → pack
configuration (agents, formulas, orders) → composing existing formula and
order machinery → an adapter behind an existing provider interface → new SDK
surface. New SDK surface must pass the Primitive Test
(`engdocs/contributors/primitive-test.md`).

## Development approach

**TDD.** Write the test first, watch it fail, make it pass. Every package
has `*_test.go` files next to the code. Integration tests that need real
infrastructure (tmux, filesystem) go in `test/` with build tags.

**The architecture docs are a reference, not a blueprint.** When the DX
conflicts with the docs, DX wins. We update the docs to match.

**Search history before rebuilding.** If a feature looks missing or
regressed, check `git log -S <symbol>` / `git log --all --grep <keyword>`
first; working code is often already in history, and a removal may have been
deliberate. Treat archived plans and audits as evidence, not gospel — confirm
against current code.

## Architecture

**Orchestration is the value.** A formula is a method for how a job gets
done, and the controller (`engdocs/architecture/controller.md`) runs it as a
graph — decomposing the job into beads, fanning the ready ones out to many
agents at once, gating each step on its dependencies, retrying failures, and
draining convoys in parallel, driving the work to completion outside the
user's session. The control dispatcher (`internal/dispatch`) executes control
beads — check, retry, fan-out, tally, drain, scope-check, workflow-finalize
(`docs/reference/specs/formula-spec-v2.md` sec 0) — and that dispatcher is
the engine. Orders trigger formulas on a schedule or event; health patrol
keeps the fleet alive.

**This orchestration is composed from primitives, with ZERO hardcoded
roles.** Beads are the universal persistence substrate — work survives
sessions — and every orchestration mechanism is provably composable from the
six primitives (**Agent** = WHO, **Bead** = WHAT, **Formula** = HOW,
**Rig** = WHERE, **Pack** = CONFIGURES, **Event** = OBSERVE), defined in
`docs/getting-started/how-gas-city-works.md`. That composability is what lets
the same SDK be configured as Gas Town, Ralph, or any other pack; no role
name appears in Go. (A single agent running a formula's steps in sequence in
the user's own session — formula v1 — is still supported as a peer shape,
but graph orchestration is why Gas City exists.)

The code-layering view — how sessions, the bead store, the event bus,
config, prompt templates, and the dispatch/health machinery implement the
six primitives, plus the progressive capability levels — is
`engdocs/architecture/nine-concepts.md`.

### Layering invariants

1. **No upward dependencies.** Layer N never imports Layer N+1.
2. **Beads is the universal persistence substrate** for domain state.
3. **Events are the universal outbound-notification mechanism** — fired by
   activity so humans and agents can watch; the bus is delivery machinery.
4. **Config is the universal activation mechanism.**
5. **Side effects (I/O, process spawning) are confined to Layer 0.**
6. **The controller drives all SDK infrastructure operations.**
   No SDK mechanism may require a specific user-configured agent role.

### Invariants enforced by CI

Violating any fails the build. Details live in the linked files.

- **Object model at the center.** The CLI (`cmd/gc/`) and the HTTP+SSE API
  (`internal/api/`) are projections over the domain packages; neither
  re-implements domain logic (`internal/api/AGENTS.md`).
- **Typed wire and typed events.** No hand-written JSON or untyped wire
  types; the OpenAPI spec is generated (`TestOpenAPISpecInSync`); every
  event type has a registered payload
  (`TestEveryKnownEventTypeHasRegisteredPayload`).
- **Vendor-neutral hosted-service wire.** Commercial policy is never a wire
  field (`internal/cliauth/AGENTS.md`, `scripts/check-core-boundary.sh`).
- **Worker boundary (active migration).** Production `cmd/gc` code reaches
  sessions through `worker.Handle`
  (`TestGCNonTestFilesStayOnWorkerBoundary`; `internal/worker/AGENTS.md`).
- **Typed store boundary (active migration).** Outside the residency
  layer, code takes typed class handles, not a raw `beads.Store`, and uses
  `beads.As` for capabilities (`storeclass` lint, shrink-only baseline).
- **Agent config field sync.** A new `config.Agent` field is wired in every
  copy and merge site (`TestAgentFieldSync` and siblings;
  `internal/config/AGENTS.md`).

**Session-first (completed `dd90ac0a` on Mar 8 2026).** The former Agent
Protocol primitive was removed; responsibilities moved to
`internal/session/` (lifecycle) and `internal/runtime/` (providers).
`internal/agent/` is now a helper package with session-name utilities and
startup hints — not a primitive. Do not reconstruct the `Agent` / `Handle`
interfaces.

## Design decisions (settled)

These decisions are final. Do not revisit them.

- **City-as-directory model.** A city is a directory on disk containing
  `city.toml`, `.gc/` runtime state, and `rigs/` infrastructure.
- **Fresh binary, not a Gas Town fork.** We build `gc` from scratch.
- **TOML for config.** `pack.toml` (definition) and `city.toml` (deployment) are the config files.
- **Tutorials win over architecture docs.** When the docs disagree, we update the docs.
- **No premature abstraction.** Don't build interfaces until two
  implementations exist.
- **Mayor is overseer, not worker.** The mayor plans; coding agents work.
- **`internal/` packages for now.** SDK exports (`pkg/`) are future work.
  Everything is private to the `gc` binary until the API stabilizes.
- **ZERO hardcoded roles.** Roles are pure configuration. No role name
  appears in Go source code.

## Key design principles

- **Keep judgment out of Go.** Go handles transport, not reasoning. The
  framework moves work; it doesn't reason about it. If a line of Go contains
  a judgment call, it's a violation. **The test:** does any line of Go contain
  a judgment call? An `if stuck then restart` is framework intelligence. Move
  the decision to the prompt.
- **A primitive must become more useful as models improve.** Every primitive
  should grow MORE useful as models improve, not less. Don't build heuristics
  or decision trees.
- **If you find work on your hook, you run it.** No confirmation, no waiting.
  The hook having work IS the assignment. This is rendered into agent prompts
  via templates, not enforced by Go code.
- **The system converges because work persists.** The system converges to
  correct outcomes because work (beads), hooks, and molecules are all
  persistent. Sessions come and go; the work survives. Multiple independent
  observers check the same state idempotently and converge on it. Redundancy
  is the reliability mechanism.
- **No status files — query live state.** Never write PID files, lock files,
  or state files to track running processes. Always discover state by querying
  the system directly (process table, port scans, `ps`, `lsof`). Status files
  go stale on crash and create false positives. The process table is the
  single source of truth for "what is running."
- **SDK self-sufficiency.** Every SDK infrastructure operation (gate
  evaluation, health patrol, bead lifecycle, order dispatch) must
  function with only the controller running. No SDK operation may
  depend on a specific user-configured agent role existing. The
  controller drives infrastructure; user agents execute work. Test:
  if removing a `[[agent]]` entry breaks an SDK feature, it's a
  violation.

## What Gas City does NOT contain

These are permanent exclusions, not "not yet." Each fails the test of
becoming more useful as models improve — it becomes LESS useful instead.

- **No skills system** — the model IS the skill system
- **No capability flags** — a sentence in the prompt is sufficient
- **No MCP/tool registration** — if a tool has a CLI, the agent uses it
- **No decision logic in Go** — the agent decides from prompt and reality
- **No hardcoded role names** — roles are pure configuration

## Code conventions

- Unit tests next to code: `config.go` → `config_test.go`
- `t.TempDir()` for filesystem tests
- Integration tests use `//go:build integration`
- `cobra` for CLI, `github.com/BurntSushi/toml` for config
- Atomic file writes: temp file → `os.Rename`
- No panics in library code — return errors
- Error messages include context: `fmt.Errorf("adding rig %q: %w", name, err)`
- Don't swallow errors: no silently filled-in defaults, masked decode
  failures, or ignored timeouts.
- Comments describe current behavior; don't narrate removed functionality.
- Role names never appear in Go code. If you're writing `if role == "mayor"`,
  it's a design error.
- **Tmux safety:** Never run bare `tmux kill-server` as cleanup. Never kill the
  default tmux server. If tmux cleanup is required, target only the known
  city/test socket explicitly with `tmux -L <socket> ...`, or prefer `gc stop`
  for city shutdown. Treat personal tmux servers as out of bounds.
- **Git safety:** Never run `git checkout <ref> -- .` (or any pathspec
  checkout) in a worktree you do not own — above all the shared rig root
  (`$GC_RIG_ROOT`). Unlike `git checkout <ref>`, the pathspec form overwrites
  the index and worktree for every tracked path, moves no HEAD (so no reflog
  entry) and stages nothing (so no dangling blob): overwritten uncommitted
  work is unrecoverable. To read a file at a ref use `git show <ref>:<path>`.
  To check something out, use your own worktree or a disposable
  `git worktree add`.
- **Non-interactive shell:** `cp`, `mv`, and `rm` may be aliased to `-i` and
  hang an agent; use `cp -f`, `mv -f`, `rm -f`, `rm -rf`, `cp -rf`. Use
  `-o BatchMode=yes` for `ssh`/`scp`, `-y` for `apt-get`, and
  `HOMEBREW_NO_AUTO_UPDATE=1` for `brew`.

## Build and test

**Bazel is the build and test system, and `bazel test` is authoritative:**
it is what CI gates on (`.github/workflows/bazel.yml`). Agents run tests
only with `bazel test`, also while iterating on one package. Do not run
`go test`: it skips nogo lint/vet, formatting, generated-artifact and policy
targets, and a green `go test` is not evidence a change passes CI.

| Tier | Command (`make` alias) |
|---|---|
| Unit + nogo + format + generated + policy | `bazel test //...` (`make test`) |
| One package while iterating | `bazel test //internal/config:config_test` |
| Acceptance (Tier A) | `bazel test --config=acceptance //test/acceptance:acceptance_test` (`make test-acceptance`) |
| Integration-tagged packages (gating) | `bazel test --config=integration //test:integration_packages` |
| `test/integration` (evidence-only in CI) | `bazel test --config=integration //test/integration:integration_test` |
| Docs sync | `bazel test //test/docsync:docsync_test` (`make check-docs`) |

- **Targets.** A package's default test target is `//<dir>:<leaf>_test`,
  `<leaf>` being the last element of `<dir>` (`//cmd/gc:gc_test`); a subtree
  is `//internal/beads/...`. Some packages add targets the default does not
  cover, such as `//test/integration:integration_unsharded_test` and cmd/gc's
  `gc_sim_*` corpus: the package's `BUILD.bazel` lists them. Bazel runs only
  the test files `BUILD.bazel` lists; run `make bazel-sync` after adding one.
- **Flags.** `go test`'s `-run X` is `--test_filter=X` (one per invocation,
  for every target named), `-v` is
  `--test_env=GO_TEST_WRAP_TESTV=1 --test_output=all` (rules_go's test
  wrapper then runs the binary with `-test.v`, and the log and `test.xml`
  list every test that ran), `-count=N` is `--test_arg=-test.count=N`
  (`--runs_per_test=N` runs N separate actions instead), `VAR=x go test` is
  `--test_env=VAR=x` and `-tags integration` is `--config=integration`. On a
  sharded target, `--test_sharding_strategy=disabled` runs a narrow filter
  as one action.
- **Evidence.** Results are cached by input: an unchanged target returns its
  cached result, which is the same evidence, so do not add
  `--nocache_test_results` to re-prove a pass (timing runs are the
  exception: TESTING.md "Timing objectives and resource ratchets"). A target
  also passes when its filter matches nothing, its tests skip, or its
  `BUILD.bazel` omits a test file: when a filtered run is your evidence, run
  it with `--test_env=GO_TEST_WRAP_TESTV=1 --test_output=all` and check the
  log for `--- PASS: <TestName>`. Re-run with the same flags: `--test_output`
  and `--test_summary` only change what is printed, but any other change to
  `--test_arg`, `--test_env` or `--test_filter` is a new cache key, and
  `--test_arg=-test.v` keys the same verbose run apart from everyone else's.
- **Regenerating checked files** (`-update`, `-update-golden`,
  `UPDATE_GOLDEN=1`) is not a test run: `bazel test` runs the test in a
  sandbox, so `--test_arg=-update` can pass while changing nothing. Run the
  generator with `bazel run` from the repository root (its own flag in place
  of `-update` below, or its environment switch, such as `UPDATE_GOLDEN=1`,
  in front), check that `git status` lists every new and changed file, then
  `bazel test` the result:

  ```bash
  GC_TEST_REPO_ROOT="$PWD" bazel run --test_sharding_strategy=disabled \
    //<dir>:<leaf>_test -- -test.run='^<TestName>$' -update
  ```

  A relative write reaches your checkout only through a runfiles link, and
  those exist only for files already there: `touch` each new golden first,
  or the generator writes it under `bazel-out` and the run still passes.

- **Where it runs.** Agent hosts' `~/.bazelrc` names rbe-west's executor, so
  plain `bazel test` executes remotely. Contributors use
  `--config=fork-cache` (anonymous read-only cache, local execution of
  misses); maintainers with an rbe-west client certificate use
  `--config=remote-exec` (ask your human before adding either to
  `.bazelrc.local`). Details: TESTING.md "Bazel cache tiers" and
  `engdocs/bazel-quickstart.md`.
- **BUILD files.** After changing imports or adding packages or files, run
  `make bazel-sync` and commit the result; the `BUILD files in sync` CI gate
  fails otherwise. Never commit machine-specific endpoints; they belong in
  `.bazelrc.local`.
- **Go-native twins** (the `-go` make targets and the sharded runners of
  TESTING.md "Cross-category runners, timing, and resource isolation") are
  for humans working offline and for hosts Bazel does not serve: the macOS
  CI jobs, and Linux hosts that cannot load the pinned ICU runtime
  (TESTING.md). An agent runs them only on such a host.
- **Never run `go clean -cache`** — it corrupts shared build caches.
  `go clean -testcache` is fine. Maintainers on the shared build hosts: read
  `engdocs/contributors/maintainer-environment.md` before touching `GOCACHE`
  or `TMPDIR`.
- **Git hooks:** `make setup` installs `.githooks` as `core.hooksPath`;
  `make check-hooks` verifies it. Pre-commit runs nogo on staged packages;
  pre-push runs `bazel test //...` and says loudly when it falls back to
  `go test`. Beads' installer can silently take the path over and skip every
  gate — see "Git hook ownership" in `CONTRIBUTING.md`.

## Code quality gates

Before considering any task complete:

- `make check` passes (`bazel test //...` plus the shell guards; nogo is
  lint and vet)
- Acceptance or integration behavior changed: the matching `--config` tier
  above passes
- `.githooks/pre-commit` is active locally (verify with `make check-hooks`)
  and has run for the staged change
- `make dashboard-check` passes (the dashboard's Bazel targets: typecheck,
  Vitest, build, and drift checks, which `bazel test //...` also runs) and the
  dashboard serves locally (a manual `npm run preview` step) for any change
  touching the API, the OpenAPI spec, or the dashboard
  (`internal/api/AGENTS.md`)
- Every exported function has a doc comment
- No premature abstractions
- Tests cover happy path AND edge cases

## Git hook ownership

**`.githooks` is the single owner of `core.hooksPath`.** Install it with
`make setup`; verify it with `make check-hooks`.

Only one directory can own `core.hooksPath`, and beads' installer claims it
for `.beads/hooks`. Those hooks exec `bd hooks run <hook>` without chaining
onward, so while beads owns the path every gate in `.githooks` — staged-Go
formatting, `lint-changed`, the three codegen+stage steps, `make vet`, and the
push-time suite — is skipped on every commit. Nothing reports this: git simply
stops invoking the hooks, so commits look clean while spec-derived drift lands
on the mainline until a later suite failure surfaces the drift.

Reclaiming the path does not disable beads. Each `.githooks` hook forwards to
`.githooks/lib/beads-chain.sh`, which runs `bd hooks run <hook>` with the same
timeout and exit-code carve-outs beads' own integration block used. Adding a
hook that beads manages means adding its `.githooks` counterpart too —
`TestGitHooksCoverEveryBeadsManagedHook` in `scripts/` fails otherwise.

Beads' installer can reclaim `core.hooksPath` at any time. When it does,
`make check-hooks` fails and `make setup` puts it back.

`make spec-ci` (run by the required `preflight-generated` CI job) is the
backstop for spec/client drift, but it only sees work that reaches a PR —
locally merged branches depend on the pre-commit gate actually running.

## Non-Interactive Shell Commands

**ALWAYS use non-interactive flags** with file operations to avoid hanging on confirmation prompts.

Shell commands like `cp`, `mv`, and `rm` may be aliased to include `-i` (interactive) mode on some systems, causing the agent to hang indefinitely waiting for y/n input.

**Use these forms instead:**

```bash
# Force overwrite without prompting
cp -f source dest           # NOT: cp source dest
mv -f source dest           # NOT: mv source dest
rm -f file                  # NOT: rm file

# For recursive operations
rm -rf directory            # NOT: rm -r directory
cp -rf source dest          # NOT: cp -r source dest
```

**Other commands that may prompt:**

- `scp` - use `-o BatchMode=yes` for non-interactive
- `ssh` - use `-o BatchMode=yes` to fail instead of prompting
- `apt-get` - use `-y` flag
- `brew` - use `HOMEBREW_NO_AUTO_UPDATE=1` env var

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->

## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files
- For controller or session reconciler incidents, use `gc trace` and follow `engdocs/contributors/reconciler-debugging.md` for the artifact collection workflow.
- When a bead needs to pause on a specific actor or condition, only `hold:mayor` and `hold:external` are canonical (set via `bd set-state <id> hold=mayor|external --reason "..."`) — never invent a new ad hoc hold/blocked label. See `engdocs/contributors/hold-label-conventions.md`.

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git fetch origin
   git merge --no-ff origin/dc-use   # integrate the lane; never rebase
   git push                          # a failed push is red: report it, never loop
   git status  # MUST show "up to date with origin"
   ```
   NOTE: gascity Dolt is LOCAL-ONLY (no remote). Do NOT run `bd dolt push`,
   `bd dolt pull`, or `bd dolt remote add` here -- they fail and re-introduce
   a doomed `origin` remote (ga-9wsri). Use `git push` only.

   That same no-remote shape is why bd >= 1.3.0 refuses to auto-apply pending
   schema migrations to gascity's shared Dolt sql-server: migrating would lock
   out every co-resident bd still on the old schema. If a bd WRITE fails with a
   refusal naming pending migrations, the sanctioned fix is `bd migrate schema`
   run once by a designated migrator after every bd client is upgraded -- NOT
   `bd dolt pull`, and not an ad-hoc `BD_ALLOW_REMOTE_MIGRATE=1`.
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**

- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, it is red: report the exact error, integrate with
  `git merge --no-ff origin/dc-use`, revalidate, and push again — never rebase or
  force-push
<!-- END BEADS INTEGRATION -->

## Architecture Best Practices

These apply to all code in this project — frontend and server:

- **TDD (Test-Driven Development)** - write the tests first; the implementation
  code isn't done until the tests pass.
- **Consider First Principles** to assess your current architecture against the
  one you'd use if you started over from scratch.
- **Leverage Types** using statically typed languages (TypeScript, Rust, etc) so
  that we can leverage the power of the compiler as guardrails and immediate
  feedback on our code at build-time instead of waiting until run-time.
- **DRY (Don't Repeat Yourself)** – eliminate duplicated logic by extracting
  shared utilities and modules.
- **Separation of Concerns** – each module should handle one distinct
  responsibility.
- **Single Responsibility Principle (SRP)** – every class/module/function/file
  should have exactly one reason to change.
- **Clear Abstractions & Contracts** – expose intent through small, stable
  interfaces and hide implementation details.
- **Low Coupling, High Cohesion** – keep modules self-contained, minimize
  cross-dependencies.
- **Scalability & Statelessness** – design components to scale horizontally and
  prefer stateless services when possible.
- **Observability & Testability** – build in logging, metrics, tracing, and
  ensure components can be unit/integration tested.
- **KISS (Keep It Simple, Sir)** - keep solutions as simple as possible.
- **YAGNI (You're Not Gonna Need It)** – avoid speculative complexity or
  over-engineering.
- **Don't Swallow Errors** by catching exceptions, silently filling in required
  but missing values, masking deserialization with nulls or empty lists, or
  ignoring timeouts when something hangs. All of those are errors (client-side
  and server-side) and must be tracked in a centralized log so it can be used to
  improve the app over time. Also, inform the user as appropriate so that they
  can take necessary action.
- **No Placeholder Code** - we're building production code here, not toys.
- **No Comments for Removed Functionality** - the source is not the place to
  keep history of what's changed; it's the place to implement the current
  requirements only.
- **Layered Architecture** - organize code into clear tiers where each layer
  depends only on the one(s) below it, keeping logic cleanly separated.
- **Use Non-Nullable Variables** when possible; use nullability only when there
  is NO other possibility.
- **Use Async Notifications** when possible over inefficient polling.
- **Eliminate Race Conditions** that might cause dropped or corrupted data
- **Write for Maintainability** so that the code is clear and readable and easy
  to maintain by future developers.
- **Arrange Project Idiomatically** for the language and framework being used,
  including recommended lints, static analysis tools, folder structure and
  gitignore entries.
- **Keep Serialization/Deserialization At The Edges** to make full use of
  type-safe objects in the app itself and to centralize error handling for
  type-system translation. Do NOT allow untyped data with known shapes to flow
  through the system and subvert the type system.
- **Prefer Well-Known, High Quality OSS Libraries** instead of hand-rolling your
  own behavior to get more robust, better maintained and better tested results.
- **Treat Static Warnings And Info As Errors To Be Fixed**. The whole point of
  static checking (linting, compilers, etc) is that they surface issues at
  build-time so that they can be fixed now instead of lead to errors at runtime.
  Take advantage of that feedback to fix those errors!
- **Use Centralized Semantic Constant Values** using enums and constants instead
  of spreading magic numbers throughout the code.

<!-- migrated from CLAUDE.md -->
@AGENTS.md

## Claude Code

Keep only Claude-specific guidance here. Prefer shared project rules in `AGENTS.md`.

<!-- migrated from GEMINI.md -->
@AGENTS.md

## Gemini CLI

Keep only Gemini-specific guidance here. Prefer shared project rules in `AGENTS.md`.
<!-- AIHUB-AGENTS-SCOPE-LOCAL-END -->
