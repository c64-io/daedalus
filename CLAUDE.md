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
- **Language-agnostic for code generation.** d7 itself is written in Go, but
  its generation commands produce code in whatever language/framework the
  user specifies for the target project. d7 is a planning *and* coding
  companion.
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
  `draft → refined → ready → in-progress → review → done → archived`,
  plus an independent `blocked` flag. Transitions are validated by the core.
- **Priority and size on every work item.** Priority is an enum:
  `low | medium | high | critical`. Size is a Fibonacci point value
  (`1, 2, 3, 5, 8, 13, 21`) — chosen over t-shirts because it composes
  cleanly into rollups at the Feature and Epic level. Both default to
  unset; both can be edited at any status.
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

## Scenarios → Gherkin

Scenarios are stored as structured records but are designed to round-trip to
real [Gherkin](https://cucumber.io/docs/gherkin/) `.feature` files via
`d7 export gherkin`:

- Each Scenario has a `title`, ordered `given[]`, a `when`, ordered `then[]`,
  and optional `tags[]`.
- Exported features live under `d7/exports/features/` and can be consumed by
  godog, Cucumber, behave, SpecFlow, etc.
- **v1 generates Gherkin only.** d7 does *not* ship step-definition stubs,
  test runners, or a `d7 verify` loop in v1. The Gherkin file is the
  handoff; wiring it to a runner in the target stack is the user's call.
  Closing the spec→test→code loop is a v2 ambition, not a v1 promise.

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

- **Target selection.** Accepts a Story, a Feature, or a whole Epic, plus a
  language/framework hint (e.g. `--lang go`, `--framework nextjs`,
  `--lang python --framework fastapi`).
- **Context assembly.** Reads the relevant spec tree, cross-links, and
  Gherkin scenarios out of Clover. The happy path is **clean-slate
  generation** into a fresh target directory, but generation is also
  **codebase-aware**: if the target already contains code, d7 skims it so
  the agent can match existing conventions rather than fighting them.
- **Multi-turn agent loop.** Generation is not one prompt, one batch of
  writes. It's an agentic loop: the model proposes file reads, file
  edits, and (where appropriate) command executions; d7 runs them through
  driven ports; results feed back into the next turn until the agent
  declares the slice complete or hits a turn/budget limit. This is the
  single biggest engineering piece in v1 and the reason the AI port is
  richer than a plain "complete this prompt" call.
- **Worktree isolation.** Generated changes land in a **git worktree**
  of the target repo, not the user's main working tree. The user reviews
  the worktree's diff, merges it when satisfied, and discards it
  otherwise. The user's uncommitted work is never at risk.
- **Regeneration is first-class.** d7 tracks which Story produced which
  files (a generation ledger in Clover). When the Story's spec changes,
  `d7 regenerate story STORY-047` can redo just that slice, diffing
  against the previous generation so the user sees exactly what the spec
  change implies for the code. Regeneration reuses the same worktree
  flow; nothing is overwritten without review.

In v1 the generator is intentionally language-agnostic: d7 does not ship with
a fixed set of templates and **Go is not a privileged target**. The fact that
d7 itself is written in Go is an implementation detail of the tool; the
target project can be in any language or framework the user names. d7
composes a strong prompt from the spec tree and the user's language choice,
and drives Claude to produce idiomatic code for that target. Over time,
sharpened per-language profiles can be added as presets.

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
    workspace_initializer.go                  # driving
    workspace_repository.go                   # driven: storage
    filesystem.go                             # driven: disk side-effects
    (future) idea_service.go ... code_generator.go  # driving
    (future) ai_assistant.go                  # driven: multi-turn agent over LLM
    (future) git_worktree.go                  # driven: worktree create/commit/discard
    (future) editor.go                        # driven: $EDITOR
    (future) clock.go                         # driven: time (audit trail)
  service/                                    # pure use-case implementations
internal/adapter/
  driving/cli/                                # cobra commands
  driven/
    clover/                                   # Clover v2 implementation of storage ports
    osfs/                                     # os-backed FileSystem
    (future) anthropic/                       # Anthropic SDK impl of ai_assistant (agentic loop)
    (future) gitworktree/                     # git worktree adapter for generation
    (future) editorexec/                      # $EDITOR launcher
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
   plumbing lives only in `adapter/driven/gitworktree`.
6. **Commands are thin.** A cobra `RunE` function should parse args, call
   exactly one port method, format the result, and return. All branching
   logic belongs in services.
7. **History is write-only.** Mutations that change status, body, parent,
   or links append a `HistoryEntry`. Nothing in the service layer is
   allowed to rewrite or delete history rows. Audit trail is a contract,
   not a feature flag.

## Current state (as of this commit)

Only the foundation is in place:

- `d7 init [path]` creates `d7/.db/` and provisions an empty Clover store.
  Errors if `d7/` already exists.
- Ports: `WorkspaceInitializer`, `WorkspaceRepository`, `FileSystem`.
- Adapters: `clover` (storage), `osfs` (filesystem), `cli` (cobra).

Everything else in this document — ideas, epics, features, stories, the
sparse graph, history, Gherkin export, AI assist, the agentic generator,
worktree isolation, regeneration — is planned surface area and should be
built incrementally, each behind its own port, each with the same discipline.

## Build & verify

```sh
go mod tidy
go build ./...
go vet ./...
```

Manual smoke test:

```sh
mkdir /tmp/d7-test && cd /tmp/d7-test
go run github.com/c64-io/daedalus/cmd/d7 init
ls -la d7/.db                # expect data.db
go run github.com/c64-io/daedalus/cmd/d7 init   # expect error (already initialized)
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

## Out of scope for v1

- Multi-user / sync / server component.
- Team features: assignment, ownership, @mentions, review queues.
- Jira / Linear / GitHub Issues import/export.
- Web or TUI frontends — CLI only.
- Fine-tuned per-language code generators (the v1 generator is prompt-driven
  and language-agnostic).
- Step-definition generation, test runners, and a `d7 verify` spec→test loop.
- Secret management beyond `ANTHROPIC_API_KEY` in the environment.
