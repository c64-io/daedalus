# CLAUDE.md

Guidance for Claude Code (and humans) working in the `c64-io/daedalus` repository.

## What is d7?

**Daedalus** (short name **d7**) is a Go CLI that unifies product planning and
AI-assisted coding into a single local workflow. Think **Claude Code for the
whole product lifecycle**: it takes a solo founder from a vague idea, to
epics, to features, to stories, to executable Gherkin specifications, and
finally to generated code in whatever language the target project uses.

**Primary user:** the solo founder. One person, one product, one machine.
Every design decision — single-project workspaces, local-only storage, no
assignment/ownership fields, no team sync — exists to serve that user. If a
feature would only make sense on a team of five, it does not belong in v1.

d7 is:

- **Local-first and co-located with the code.** `d7 init` creates a `d7/`
  directory *inside the target project*, backed by an embedded Clover v2
  database in `d7/.db/`. The plan lives next to the code it plans.
- **Human-driven, AI-assisted.** The human owns the breakdown. Claude (via the
  Anthropic API) is invoked on demand through commands like `d7 suggest`,
  `d7 expand`, `d7 refine`, `d7 generate` — never silently.
- **Targeted, not universal.** v1 supports **Go and TypeScript** end-to-end
  as generation and verification targets. Other languages are a v2+
  ambition. d7 itself is written in Go, but Go is not privileged over
  TypeScript as a *target* — they are peers.
- **Hexagonal by construction.** The core knows nothing about cobra, Clover,
  the filesystem, git, or the Anthropic API. Everything crosses a port.

## Product model

Strict hierarchy, enforced by the schema. No skipping levels.

```
Idea        (the vague spark)
 └─ Epic    (a major capability)
     └─ Feature   (a coherent chunk of that capability)
         └─ Story     (a user-visible slice; INVEST)
             └─ Spec       (prose requirements + context)
                 └─ Scenario (Given/When/Then, exported as Gherkin)
```

Rules:

- **Single project per workspace.** One `d7 init` = one product. No
  multi-project workspaces in v1.
- **Strict parentage.** A Story must belong to a Feature; a Feature to an
  Epic; an Epic to an Idea. The CLI rejects orphan creation.
- **Human-readable IDs.** `IDEA-001`, `EPIC-003`, `FEAT-012`, `STORY-047`,
  `SCEN-114`. Per-type monotonic counters stored in Clover. IDs are stable;
  titles are editable.
- **Full-agile lifecycle.** Every item carries a status:
  `draft → refined → ready → in-progress → review → done → archived → blocked`.
  `blocked` is a full status (not a separate flag) — any active status
  can transition to `blocked`, and `blocked` can return to any active
  status. Transitions are validated by the core's state machine.
- **Priority and size on work items (Epic and below).** Priority is an
  enum: `low | medium | high | critical`. Size is a Fibonacci point
  value (`1, 2, 3, 5, 8, 13, 21`) — chosen over t-shirts because it
  composes cleanly into rollups at the Feature and Epic level. Both
  default to unset; both can be edited at any status. Ideas do not
  carry priority or size — they are too vague for estimation.
- **History and hierarchy are preserved.** The Clover store is append-friendly:
  every status transition, every spec body edit, every parent/child change is
  recorded with a timestamp so "why does this exist and how did it get here?"
  is answerable six months later. The hierarchy itself is never flattened —
  the store keeps the tree, not just leaves.
- **Sparse graph on top of the tree.** Beyond strict parent/child,
  work items can carry typed cross-links: `blocked-by`, `relates-to`,
  `duplicates`, plus free external references (URLs to Figma, RFCs,
  tickets, prior art). The tree is the backbone; the graph captures the
  messy reality.
- **Clover is the source of truth.** Markdown/JSON/Gherkin are *exports*, not
  the store. Exports are regeneratable; the DB is authoritative.

## Supported targets (v1)

v1 supports exactly two target ecosystems end-to-end — planning, generation,
*and* verification — and nothing else:

| Language   | Runner library                                   | Install surface         |
| ---------- | ------------------------------------------------ | ----------------------- |
| Go         | [`github.com/cucumber/godog`](https://github.com/cucumber/godog) | `go get` + `go test`    |
| TypeScript | [`@cucumber/cucumber`](https://github.com/cucumber/cucumber-js)  | `npm`/`pnpm` devDependency + `npx cucumber-js` |

Both runners are maintained by the Cucumber organization, both emit
[Cucumber JSON](https://github.com/cucumber/cucumber-json-schema) via a
documented `--format` flag, and both preserve Gherkin tags verbatim in their
output — which is what makes the scenario-ID mapping described below work
identically across both.

### Target language is declared at `d7 init` and is immutable

A v1 workspace has **exactly one** target language. It is set once, at
workspace creation:

```sh
d7 init --lang go
d7 init --lang typescript
```

Once chosen, **the target is immutable for the life of the workspace**.
There is no `d7 target set` command, and this is deliberate:

- The generation ledger becomes incoherent if half the Stories were
  generated under one target and half under another.
- Step-definition layout, runner invocation, and verification semantics are
  target-specific; changing mid-stream guarantees drift.
- A solo founder who truly picked wrong can re-`init` a fresh workspace
  faster than d7 could correctly migrate one. v1 takes the simple rule.

The chosen target is stored in a workspace metadata record in Clover,
validated by every command that touches code generation or verification,
and surfaced in `d7 status`.

**Monorepo note.** Projects with both a Go backend and a TypeScript
frontend are a real solo-founder shape and are explicitly *not* a v1
concern. In v1, if you want d7 to cover both sides, run `d7 init` twice —
once in each subdirectory — and live with two sibling `d7/` workspaces.
Native multi-target workspaces (one `d7/` covering several target
ecosystems, with per-Story target assignment) are a v2+ ambition; adding
them later is a widening of the schema, not a breaking change, because
v1 Stories all belong to the single declared target by construction.

## Scenarios → Gherkin

Scenarios are stored as structured records but are designed to round-trip to
real [Gherkin](https://cucumber.io/docs/gherkin/) `.feature` files via
`d7 export gherkin`:

- Each Scenario has a `title`, ordered `given[]`, a `when`, ordered `then[]`,
  and optional user `tags[]`.
- Exported features live under `d7/exports/features/` and are consumed by
  the target's runner (godog or cucumber-js).

### Mandatory scenario-ID tagging

Every scenario d7 exports is automatically prefixed with a tag carrying its
stable d7 ID:

```gherkin
@d7:SCEN-114
Scenario: User can redeem a coupon at checkout
  Given ...
```

This tag is **not optional and not user-editable**. It is the contract that
lets d7 map runner output back to Scenario records in Clover: both godog
and cucumber-js preserve tags verbatim in their JSON output, so
verification results flow back via exact tag match rather than fuzzy title
matching. Users are free to rename scenarios at any time — the
verification ledger and generation ledger track by `SCEN-XXX`, never by
title. This is cheap to enforce now and impossible to retrofit cleanly
later.

## Verification (`d7 verify`)

`d7 verify` runs the Gherkin scenarios of a target (Story, Feature, or
Epic) through the declared runner for that target's language and records
structured pass/fail results in a **verification ledger** in Clover. It is
a first-class command for two reasons:

1. It is the **terminator of the `d7 generate` agent loop** (see the next
   section). The generator is not "done" until verify returns green.
2. It is available as a **standalone command** so the user can re-verify
   after editing code by hand, after pulling changes from collaborators,
   or before merging a worktree back to main. Manual edits are expected
   and supported; the verifier is not exclusively for the AI's benefit.

Verification results are append-only, keyed by
`(scenario_id, generation_id | "manual", timestamp)`, so the history of
"SCEN-114 flipped red when we regenerated Story STORY-047 from spec v7 to
v8" is queryable for the life of the project.

## AI assist

- **Model:** default is `claude-opus-4-6` for breakdown, expansion, and
  generation commands — the quality of idea-to-epic decomposition and of
  generated code matters more than latency. (Haiku/Sonnet may be added
  later as per-command overrides.)
- **Auth:** `ANTHROPIC_API_KEY` environment variable. No config-file storage
  in v1 — keep it 12-factor and avoid secret sprawl.
- **Invocation is always opt-in.** Commands like `d7 suggest epics --idea=IDEA-001`
  or `d7 expand story STORY-047` call the API. Plain data commands
  (`d7 epic add`, `d7 story list`, `d7 status set`) never do.
- **AI output is a proposal.** It is shown to the user, diffed against current
  state if relevant, and only written on explicit confirmation (or with
  `--yes`).
- **Interactive refinement, not one-shot.** AI-driven commands like
  `d7 suggest`, `d7 expand`, and `d7 refine` run an interactive loop: the
  model produces a proposal, the user critiques or edits it, the model
  revises, and so on until the user accepts. These commands are dialogues,
  not batch jobs.
- **Finalize before descending.** Each layer of the hierarchy must reach an
  agreed-upon state before work begins on the next. `d7 expand story` will
  not generate Given/When/Then scenarios until the Story's prose Spec is
  locked; `d7 suggest features` will not propose Features for an Epic that is
  still `draft`. This rule is enforced in the service layer, not just
  suggested in docs.
- **Uncertainty bubbles up.** If the model cannot produce a confident
  proposal at a given layer because of ambiguity in the parent, the command
  halts with the open questions surfaced to the user at the *parent* level,
  rather than guessing downward. A fuzzy Epic must be sharpened before its
  Features are drafted; a fuzzy Spec must be sharpened before its scenarios
  are written. The user is always the arbiter of the ambiguity.

## Code generation

`d7 generate` is the bridge from specs to code. Unlike scaffolders that stamp
out a fixed template, d7 acts like a focused coding agent — in the same
spirit as Claude Code itself:

- **Target selection.** Accepts a Story, a Feature, or a whole Epic. The
  target language is *not* a per-invocation flag: it is read from the
  workspace metadata set at `d7 init`, which in v1 is always a single
  target. `d7 generate story STORY-047` is unambiguous by construction.
- **Context assembly.** Reads the relevant spec tree, cross-links, and
  Gherkin scenarios out of Clover. The happy path is **clean-slate
  generation** into a fresh target directory, but generation is also
  **codebase-aware**: if the target already contains code, d7 skims it so
  the agent can match existing conventions rather than fighting them.
- **Three artifact classes per Story.** Every successful generation
  produces (1) implementation code, (2) step definitions binding the
  Story's scenarios to that code (godog step funcs for Go targets,
  cucumber-js step modules for TypeScript targets), and (3) whatever
  runner bootstrap the target needs (a `features_test.go` + godog harness,
  a `cucumber.js` config plus `package.json` script, etc.). All three are
  the agent's output, not a template.
- **Multi-turn agent loop with verify as the terminator.** Generation is
  not one prompt, one batch of writes. It's an agentic loop: the model
  proposes file reads, file edits, and command executions; d7 runs them
  through driven ports; *then d7 invokes `ScenarioRunner.Run` against the
  updated worktree and feeds the structured results back into the next
  turn*. The loop ends only when verification returns all-green for the
  target Story, or when the turn/token budget is exhausted. Red scenarios
  at exhaustion are reported to the user with the worktree left intact
  for inspection — never silently accepted. This is the single biggest
  engineering piece in v1 and the reason the AI port is richer than a
  plain "complete this prompt" call.
- **Worktree isolation.** Generated changes land in a **git worktree**
  of the target repo, not the user's main working tree. The user reviews
  the worktree's diff, merges it when satisfied, and discards it
  otherwise. The user's uncommitted work is never at risk.
- **Regeneration is first-class.** d7 tracks which Story produced which
  files and which scenarios verified green at that point (a generation
  ledger in Clover). When the Story's spec changes,
  `d7 regenerate story STORY-047` can redo just that slice, diffing
  against the previous generation and against the previous verification
  state so the user sees exactly what the spec change implies for the
  code *and* for the scenario outcomes. Regeneration reuses the same
  worktree flow; nothing is overwritten without review.

## Interaction model (hybrid)

All three styles are first-class; users pick per command:

1. **Interactive wizard** — `d7 new idea` walks through questions and creates
   the Idea, prompts to draft Epics, etc. Good for first-time capture.
2. **Flag-based one-shots** — `d7 epic add --idea=IDEA-001 --title="Billing"`.
   Scriptable, CI-safe, diffable.
3. **Editor-based long-form** — `d7 edit story STORY-047` opens `$EDITOR` on a
   templated markdown buffer for Spec bodies and Scenario drafts. On save, d7
   parses and persists.

## Architecture

Hexagonal / ports & adapters. **The core imports nothing that does I/O.**

```
cmd/d7/main.go                                # composition root
internal/core/
  domain/                                     # Idea, Epic, Feature, Story, Spec, Scenario,
                                              #   Status, Priority, Size, Link, HistoryEntry
  port/
    driving/                                  # inbound ports (CLI → service)
      workspace.go                            # InitRequest, WorkspaceInitializer,
                                              #   WorkspaceStatusReader, WorkspaceDescription{Reader,Writer}
      idea.go                                 # CreateIdeaRequest, IdeaCreator, IdeaReader, IdeaSetter
      epic.go                                 # CreateEpicRequest, EpicCreator, EpicReader, EpicSetter
      feature.go                              # CreateFeatureRequest, FeatureCreator, FeatureReader, FeatureSetter
      story.go                                # CreateStoryRequest, StoryCreator, StoryReader, StorySetter
      (future) code_generator.go              # generation use case
      (future) verifier.go                    # d7 verify use case
    driven/                                   # outbound ports (service → adapter)
      workspace.go                            # WorkspaceRepository, ErrMetadataNotFound
      idea.go                                 # IdeaRepository, ErrIdeaNotFound
      epic.go                                 # EpicRepository, ErrEpicNotFound
      feature.go                              # FeatureRepository, ErrFeatureNotFound
      story.go                                # StoryRepository, ErrStoryNotFound
      history.go                              # HistoryRepository
      filesystem.go                           # FileSystem
      editor.go                               # Editor ($EDITOR)
      (future) ai_assistant.go                # multi-turn agent over LLM
      (future) scenario_runner.go             # run Gherkin, return structured results
      (future) git_worktree.go                # worktree create/commit/discard
      (future) clock.go                       # time (audit trail)
  service/                                    # pure use-case implementations
internal/adapter/
  driving/cli/                                # cobra commands
  driven/
    clover/                                   # Clover v2 implementation of storage ports
    osfs/                                     # os-backed FileSystem
    editorexec/                               # $EDITOR launcher
    (future) anthropic/                       # Anthropic SDK impl of ai_assistant (agentic loop)
    (future) godog/                           # ScenarioRunner for Go targets
    (future) cucumberjs/                      # ScenarioRunner for TypeScript targets
    (future) gitworktree/                     # git worktree adapter for generation
```

### Non-negotiable rules for contributors (and Claude)

1. **No I/O in `internal/core/...`.** No `os`, `net/http`, `database/*`,
   no SDK imports, no `os/exec`. `path/filepath`'s pure functions (`Join`,
   `Clean`) are fine; `os.Stat` is not. Enforced by review, and by the
   fact that every service's constructor already takes ports.
2. **Accept interfaces, return structs.** Adapters expose concrete types;
   services depend on port interfaces. Add a compile-time assertion
   (`var _ port.Foo = (*Bar)(nil)`) next to every adapter and service.
3. **One use case = one method on a driving port.** Don't stuff multiple
   verbs into one interface. New verbs get new ports (or new methods only
   when they genuinely cohere).
4. **Errors wrap with context.** `fmt.Errorf("create database: %w", err)`.
   Sentinel errors (`ErrWorkspaceExists`, etc.) live in the `service`
   package and are matched with `errors.Is`.
5. **Adapters own their libraries.** Cobra types live only in
   `adapter/driving/cli`. Clover types live only in `adapter/driven/clover`.
   The Anthropic SDK will live only in `adapter/driven/anthropic`. Git
   plumbing lives only in `adapter/driven/gitworktree`. godog imports
   live only in `adapter/driven/godog`; cucumber-js invocation (via
   `os/exec`) lives only in `adapter/driven/cucumberjs`. The core sees
   only `port.ScenarioRunner`.
6. **Commands are thin.** A cobra `RunE` function should parse args, call
   exactly one port method, format the result, and return. All branching
   logic belongs in services.
7. **History is write-only.** Mutations that change status, body, parent,
   or links append a `HistoryEntry`. Nothing in the service layer is
   allowed to rewrite or delete history rows. Audit trail is a contract,
   not a feature flag.

## Current state (as of this commit)

Implemented:

- `d7 init [path] --lang <go|typescript>` creates `d7/.db/`,
  provisions an empty Clover store with immutable target metadata and
  a default project description in the database. Errors if `d7/`
  already exists. No files are created outside `d7/.db/`.
- `d7 workspace status` shows workspace dir, target, and project
  description state.
- `d7 workspace show` prints the project description from the database;
  hints on stderr if it is still the default template.
- `d7 workspace edit` opens `$EDITOR` with a YAML front-matter header
  (target, read-only) followed by the project description body. On save,
  the description is updated in the database.
- `d7 idea new --title "..." [--description "..."] [--expand]` creates
  an Idea in `draft` status with a stable IDEA-XXX ID.
- `d7 idea list` / `d7 idea show <id>` for reading Ideas.
- `d7 idea set <id> --status <status> [--title] [--description]`
  updates fields with state-machine validation and history tracking.
- `d7 idea edit <id>` opens `$EDITOR` with YAML front-matter (id,
  status, title, created) plus the description body. Changed fields
  are applied via `SetIdea`.
- `d7 epic new --idea IDEA-XXX --title "..." [--description] [--priority] [--size] [--expand]`
  creates an Epic under a parent Idea. The parent must be at least
  `refined`; draft and archived Ideas are rejected.
- `d7 epic list [--idea IDEA-XXX]` lists all epics or filters by parent.
- `d7 epic show <id>` shows epic details including parent Idea info.
- `d7 epic set <id> --status <status> [--title] [--description] [--priority] [--size]`
  updates fields with state-machine validation and history tracking.
- `d7 epic edit <id>` opens `$EDITOR` with YAML front-matter (id, idea,
  status, title, priority, size, created) plus the description body.
  Changed fields are applied via `SetEpic`.
- `d7 feature new --epic EPIC-XXX --title "..." [--description] [--priority] [--size] [--expand]`
  creates a Feature under a parent Epic. The parent must be at least
  `refined`; draft and archived Epics are rejected.
- `d7 feature list [--epic EPIC-XXX]` lists all features or filters
  by parent.
- `d7 feature show <id>` shows feature details including parent Epic info.
- `d7 feature set <id> --status <status> [--title] [--description] [--priority] [--size]`
  updates fields with state-machine validation and history tracking.
- `d7 feature edit <id>` opens `$EDITOR` with YAML front-matter (id,
  epic, status, title, priority, size, created) plus the description
  body. Changed fields are applied via `SetFeature`.
- Editor-based editing uses a shared edit loop: if YAML parsing or
  validation fails, the editor re-opens with the error prepended as
  a comment.
- `d7 story new --feature FEAT-XXX --title "..." [--description] [--priority] [--size] [--expand]`
  creates a Story under a parent Feature. The parent must be at least
  `refined`; draft and archived Features are rejected.
- `d7 story list [--feature FEAT-XXX]` lists all stories or filters
  by parent.
- `d7 story show <id>` shows story details including parent Feature info.
- `d7 story set <id> --status <status> [--title] [--description] [--priority] [--size]`
  updates fields with state-machine validation and history tracking.
- `d7 story edit <id>` opens `$EDITOR` with YAML front-matter (id,
  feature, status, title, priority, size, created) plus the description
  body. Changed fields are applied via `SetStory`.
- Domain types: `Idea`, `Epic`, `Feature`, `Story`, `Status` (with
  transition state machine including `blocked`), `Target`, `Priority`,
  `Size`, `HistoryEntry`, `ProjectDescription`, `FrontMatterField`.
- Ports are split into `port/driving` (inbound, CLI → service) and
  `port/driven` (outbound, service → adapter). Driving:
  `WorkspaceInitializer`, `WorkspaceStatusReader`,
  `WorkspaceDescriptionReader`, `WorkspaceDescriptionWriter`,
  `IdeaCreator`, `IdeaReader`, `IdeaSetter`,
  `EpicCreator`, `EpicReader`, `EpicSetter`,
  `FeatureCreator`, `FeatureReader`, `FeatureSetter`,
  `StoryCreator`, `StoryReader`, `StorySetter`.
  Driven: `WorkspaceRepository`, `IdeaRepository`, `EpicRepository`,
  `FeatureRepository`, `StoryRepository`, `HistoryRepository`,
  `FileSystem`, `Editor`.
- Adapters: `clover` (storage), `osfs` (filesystem), `editorexec`
  (`$EDITOR` launcher), `cli` (cobra).

Not yet implemented: specs, scenarios, the sparse graph, Gherkin export,
AI assist, the agentic generator, worktree isolation, regeneration, the
ScenarioRunner port, and the verify loop. All are planned surface area
and should be built incrementally, each behind its own port, each with
the same discipline.

## Build & verify

```sh
go mod tidy
go build ./...
go vet ./...
go test ./...
```

Manual smoke test:

```sh
go build -o /tmp/d7 ./cmd/d7
rm -rf /tmp/d7-test && mkdir /tmp/d7-test && cd /tmp/d7-test
/tmp/d7 init --lang go
/tmp/d7 workspace status
/tmp/d7 workspace show
EDITOR=cat /tmp/d7 workspace edit
/tmp/d7 idea new --title "My SaaS"
/tmp/d7 idea list
/tmp/d7 idea show IDEA-001
/tmp/d7 idea set IDEA-001 --status refined
EDITOR=cat /tmp/d7 idea edit IDEA-001
/tmp/d7 epic new --idea IDEA-001 --title "Billing"
/tmp/d7 epic set EPIC-001 --status refined
EDITOR=cat /tmp/d7 epic edit EPIC-001
/tmp/d7 feature new --epic EPIC-001 --title "Payment Processing"
/tmp/d7 feature list --epic EPIC-001
/tmp/d7 feature show FEAT-001
EDITOR=cat /tmp/d7 feature edit FEAT-001
/tmp/d7 feature set FEAT-001 --status refined
/tmp/d7 story new --feature FEAT-001 --title "User can log in"
/tmp/d7 story list --feature FEAT-001
/tmp/d7 story show STORY-001
/tmp/d7 story set STORY-001 --status refined
EDITOR=cat /tmp/d7 story edit STORY-001
```

## Working in this repo (for Claude Code sessions)

When adding a new feature:

1. **Start from the use case, not the command.** Define the driving port in
   `internal/core/port/` first. What is the verb? What does it return?
2. **Define or reuse driven ports.** If the use case needs storage, AI,
   filesystem, git, time, or an editor, express that dependency as a port
   parameter. Do not reach for globals or `os.*`.
3. **Implement the service.** Pure logic. Unit-testable with in-memory
   fakes. Add the compile-time assertion.
4. **Wire an adapter on each side.** Driven adapter (e.g. a new Clover
   repository method) and driving adapter (a new cobra subcommand).
5. **Compose in `cmd/d7/main.go`.** The only place concrete types meet
   interfaces.
6. **Verify:** `go build ./... && go vet ./...`, then a manual or scripted
   end-to-end run against a throwaway `d7/` workspace in `/tmp`.

When in doubt about scope: if a change feels like it wants to touch three
adapters and the core at once, it is probably two changes. Split it.

### Always link commits back to GitHub

After every code change that results in a commit on a pushed branch, end the
reply with an up-to-date GitHub link so the user can click through without
hunting. Prefer the **commit** URL for single-commit changes
(`https://github.com/c64-io/daedalus/commit/<sha>`) and the **branch compare**
URL for multi-commit updates
(`https://github.com/c64-io/daedalus/compare/main...<branch>`). If the change
is to a single reviewable file, a direct file link on the branch is also
welcome
(`https://github.com/c64-io/daedalus/blob/<branch>/<path>`).
This is a standing instruction, not a per-request ask.

## Out of scope for v1

- Multi-user / sync / server component.
- Team features: assignment, ownership, @mentions, review queues.
- Jira / Linear / GitHub Issues import/export.
- Web or TUI frontends — CLI only.
- Target languages beyond Go and TypeScript. Additional `ScenarioRunner`
  adapters (pytest-bdd, cucumber-jvm, etc.) are v2+ work and do not
  require any core changes when added.
- Multi-target workspaces (one `d7/` covering both a Go backend and a
  TypeScript frontend). v1 workspaces are single-target; monorepo
  founders run `d7 init` twice, once per subdirectory. Native
  multi-target support is a v2+ schema widening.
- Mutating a workspace's declared target after `d7 init`. If the user
  picked wrong, the answer in v1 is to re-`init` a fresh workspace.
- Secret management beyond `ANTHROPIC_API_KEY` in the environment.
