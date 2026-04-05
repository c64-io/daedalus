# CLAUDE.md

Guidance for Claude Code (and humans) working in the `c64-io/daedalus` repository.

## What is d7?

**Daedalus** (short name **d7**) is a Go CLI that unifies product planning and
AI-assisted coding into a single local workflow. Think **Claude Code for the
whole product lifecycle**: it helps a solo builder or small team go from a
vague idea, to epics, to features, to stories, to executable Gherkin
specifications, and finally to generated code in whatever language the target
project uses.

d7 is:

- **Local-first.** Everything lives in a `d7/` directory at the root of the
  user's project, backed by an embedded Clover v2 database in `d7/.db/`.
- **Human-driven, AI-assisted.** The human owns the breakdown. Claude (via the
  Anthropic API) is invoked on demand through commands like `d7 suggest`,
  `d7 expand`, `d7 refine`, `d7 generate` — never silently.
- **Language-agnostic for code generation.** d7 itself is written in Go, but
  its generation commands produce code in whatever language/framework the user
  specifies for the target project. d7 is a planning *and* coding companion.
- **Hexagonal by construction.** The core knows nothing about cobra, Clover,
  the filesystem, or the Anthropic API. Everything crosses a port.

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
- **Clover is the source of truth.** Markdown/JSON/Gherkin are *exports*, not
  the store. Exports are regeneratable; the DB is authoritative.

## Scenarios → Gherkin

Scenarios are stored as structured records but are designed to round-trip to
real [Gherkin](https://cucumber.io/docs/gherkin/) `.feature` files via
`d7 export gherkin`. That means:

- Each Scenario has a `title`, ordered `given[]`, a `when`, ordered `then[]`,
  and optional `tags[]`.
- Exported features live under `d7/exports/features/` and can be executed by
  godog, Cucumber, behave, SpecFlow, etc. — whatever matches the target
  project's language.

## AI assist

- **Model:** default is `claude-opus-4-6` for breakdown and expansion commands
  because the quality of idea-to-epic decomposition matters more than latency.
  (Haiku/Sonnet may be added later as per-command overrides.)
- **Auth:** `ANTHROPIC_API_KEY` environment variable. No config-file storage
  in v1 — keep it 12-factor and avoid secret sprawl.
- **Invocation is always opt-in.** Commands like `d7 suggest epics --idea=IDEA-001`
  or `d7 expand story STORY-047` call the API. Plain data commands
  (`d7 epic add`, `d7 story list`) never do.
- **AI output is a proposal.** It is shown to the user, diffed against current
  state if relevant, and only written on explicit confirmation (or with
  `--yes`).

## Code generation

`d7 generate` is the bridge from specs to code. Unlike scaffolders that only
stamp out a fixed template, d7 acts like a focused coding agent:

- Accepts a target (a Story, a Feature, or a whole Epic) and a language or
  framework hint (e.g. `--lang go`, `--framework nextjs`, `--lang python
  --framework fastapi`).
- Reads the relevant spec tree + Gherkin scenarios out of Clover.
- Uses the Anthropic API (Opus 4.6) to produce code changes for the target
  project directory, which may be the same repo or a sibling directory.
- Presents a plan + diffs; the user approves before anything is written.

In v1 the generator is intentionally language-agnostic: d7 does not ship with
a fixed set of templates. It composes a strong prompt from the spec tree and
the user's language choice, and drives Claude to produce idiomatic code for
that target. Over time, sharpened per-language profiles can be added as
presets.

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
  domain/                                     # Idea, Epic, Feature, Story, Spec, Scenario, Status
  port/
    workspace_initializer.go                  # driving
    workspace_repository.go                   # driven: storage
    filesystem.go                             # driven: disk side-effects
    (future) idea_service.go ... code_generator.go  # driving
    (future) ai_assistant.go                  # driven: LLM
    (future) editor.go                        # driven: $EDITOR
    (future) clock.go                         # driven: time (audit trail)
  service/                                    # pure use-case implementations
internal/adapter/
  driving/cli/                                # cobra commands
  driven/
    clover/                                   # Clover v2 implementation of storage ports
    osfs/                                     # os-backed FileSystem
    (future) anthropic/                       # Anthropic SDK implementation of ai_assistant
    (future) editorexec/                      # $EDITOR launcher
```

### Non-negotiable rules for contributors (and Claude)

1. **No I/O in `internal/core/...`.** No `os`, `net/http`, `database/*`,
   no SDK imports. `path/filepath`'s pure functions (`Join`, `Clean`) are
   fine; `os.Stat` is not. Enforced by review, and by the fact that every
   service's constructor already takes ports.
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
   The Anthropic SDK will live only in `adapter/driven/anthropic`.
6. **Commands are thin.** A cobra `RunE` function should parse args, call
   exactly one port method, format the result, and return. All branching
   logic belongs in services.

## Current state (as of this commit)

Only the foundation is in place:

- `d7 init [path]` creates `d7/.db/` and provisions an empty Clover store.
  Errors if `d7/` already exists.
- Ports: `WorkspaceInitializer`, `WorkspaceRepository`, `FileSystem`.
- Adapters: `clover` (storage), `osfs` (filesystem), `cli` (cobra).

Everything else in this document — ideas, epics, features, stories, Gherkin
export, AI assist, code generation — is planned surface area and should be
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
   filesystem, time, or an editor, express that dependency as a port
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
- Jira / Linear / GitHub Issues import/export.
- Web or TUI frontends — CLI only.
- Fine-tuned per-language code generators (the v1 generator is prompt-driven
  and language-agnostic).
- Secret management beyond `ANTHROPIC_API_KEY` in the environment.
