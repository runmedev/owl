# AGENTS.md - Owl

## Architecture Direction

Owl is a typed environment store. V2 should roll forward cleanly instead of
preserving a long-lived v1/v2 compatibility layer. Runme is the only known v1
consumer, so update Runme deliberately when the Owl API changes rather than
carrying museum code.

The core ownership boundaries are:

- `internal/model`: semantic state primitives such as type IDs, field refs,
  values, operations, bindings, diagnostics, and effective state.
- `internal/registry`: built-in type definitions and lookup. Add `core/plain`
  as the generic known non-sensitive env string type in the store cutover.
- `internal/projection/dotenv`: dotenv projection, legacy `.env.example`
  comment parsing, materialization, and dotenv rendering.
- `internal/state`: EffectiveState machine, projections, and envelopes.
- `pkg/owl`: small public API. Export what callers need; do not re-export every
  internal model detail by default.
- `cmd`: command wiring and rendering only.

Prefer cohesive structs with methods when several helpers repeatedly pass the
same conceptual state through a call chain. Avoid func soup where a small
builder, evaluator, planner, or renderer object would make ownership and
invariants clearer. This is about internal cohesion; it does not mean every
helper or intermediate type needs to become public API.

## Owl Store V2 Decisions

- CUE owns type-value validation. Go may orchestrate registry/store behavior and
  diagnostics, but must not reimplement type semantics such as URL, secret, or
  composite-field validation.
- Do not add generic `core/host` or `core/port` types yet. Network-shaped
  values carry domain policy; keep them field/domain-specific, for example
  `universe/redis.host` and `universe/redis.port`, until reusable semantics are
  clearer.
- Go owns store and contract invariants. Unknown types, unknown fields, binding
  conflicts, unresolved required dotenv values, and sensitivity mismatches are
  graph/store consistency rules rather than CUE type-value validation.
- Prefer `owl.toml` for local project config examples and default authoring.
  Keep `owl.yaml`, `owl.yml`, and `owl.json` possible because the canonical
  config shape should serialize through JSON-like inputs.
- Do not expose `owl.cue` as the local project config frontend for now. CUE
  remains the type-definition and validation language, not necessarily the user
  config language.
- Shape project config like a GraphQL input type so the same canonical model can
  later appear idiomatically in platform files such as TOML, YAML, JSON,
  `package.json`, or `pyproject.toml`.
- Keep Owl project commands flatter than Runme's store command shape when that
  better fits Owl; do not add command nesting just for symmetry.
- Env var names are projection keys. Typed fields are semantic state.
- Undeclared observed dotenv variables become `core/opaque`.
- Do not implicitly promote observed env keys into `universe/*` types during
  snapshot/check. Project config and specs are the source of semantic truth;
  key-name heuristics belong in deliberate assistive commands such as
  `owl type`, not ambient store ingestion.
- `types/universe/*` is for real ecosystem contracts only. Universe examples
  should use realistic provider conventions and official/default env keys where
  they exist. Arbitrary projection keys, odd names, collisions, and edge cases
  belong in test-only fixtures; prefer in-test fixture providers before adding
  persistent fixture CUE trees.
- Never leak sensitive values in logs, CLI output, snapshots, diffs,
  diagnostics, examples, or chat unless the user explicitly requests an
  insecure/reveal path such as `--insecure`. Unknown sensitivity must default
  to hidden, not plaintext; known-clear values must opt into plaintext through
  type or field metadata.

## Graph Engine

Do not cargo-cult against Owl's GraphQL layer.

It is an original programmable graph-engine idea, not a conventional GraphQL
data API. The query language on top of the graph engine is strategically useful
because it can provide a standardized cross-language interface with high
programmability.

The v2 work should preserve that direction while improving boundaries and code
quality. Streamline store lifecycle code that does not benefit from graph
programmability, but do not frame the graph/query layer itself as the problem.

Good target:

```text
explicit v2 model primitives
  -> store lifecycle
  -> optional graph/query execution, planning, or debug interface
```

Avoid:

```text
old v1 GraphQL-backed runtime as a second owner of state semantics
```

## Graph Facade Layering

Keep Owl operation facades layered deliberately:

- Friendly API: typed Owl input to typed Owl output. Normal Go callers should
  use methods such as `Store.Snapshot`, `Store.Source`, `Store.Check`,
  `Store.Resolve`, and `Store.ApplyPromptAnswers`.
- Operation builders: typed Owl input to canonical GraphQL document plus
  schema-shaped variables. Builders such as `BuildSnapshotOperation` should be
  pure and useful for tests, bindings, and debug tooling.
- Graph escape hatch: caller-provided GraphQL document plus variables to raw
  graph result. Expose as an advanced/debug path such as `ExecuteGraphQL`, not
  as the normal CLI, Extension, or Runme API.

Edges own I/O. CLI, Extension, Runme gRPC, and resolver/provider code should
read files, process env, protobuf streams, prompts, and external systems before
calling Owl. Graph execution receives already-materialized bytes/strings and
typed variables; it should not open project files, read process env, prompt
users, call secret managers, or speak gRPC directly.

`cmd/` is a public consumer. CLI code may use `pkg/...` APIs and command-local
wiring only; it must not import `github.com/runmedev/owl/internal/...`
packages. Move needed behavior behind `pkg/owl` or `pkg/owl/seed` instead.

Normal command/public paths must not call legacy helper shapes such as
`SnapshotItems`, `Dotenv(policy)`, `CheckState`, `LoadDotenv`,
`LoadDotenvLines`, or legacy `Update`/`Delete` helpers. Use typed graph-backed
facades such as `Snapshot`, `Source`, `Check`, and `ApplyUpdate`.

## Store Cutover Decisions

- Implement the cutover as one cohesive PR with focused commits, not separate
  delivery phases or a long-lived in-between state.
- Primary store inputs should be reader-based:
  `WithEnvFile(name string, r io.Reader)` and
  `WithSpecFile(name string, r io.Reader)`.
- Consume readers during `NewStore` option application. Do not retain readers on
  the store; callers own closing file handles.
- `owl store check` fails on error diagnostics. Unresolved required fields are
  errors and produce a non-zero exit code. Unresolved optional fields are
  non-fatal diagnostics.
- `snapshot` is the structured monitor/read API first. CLI table output is a
  renderer of that API.
- Snapshot vocabulary should use v2 `TYPE`, not old v1 `SPEC` labels.
- Accept old `.env.example` comments as migration input and lower them into v2
  state immediately:
  - `# Plain` -> `core/plain`
  - `# Secret` / `# Password` -> `core/secret`
  - `# Opaque` -> `core/opaque`
  - `!` -> required
- Unknown old custom specs should stay conservative and diagnostic-driven until
  explicitly mapped. Do not auto-promote them into `universe/*`.
## Verification

Before committing Owl code changes, run:

```bash
make check
runme run test
runme run lint
```
