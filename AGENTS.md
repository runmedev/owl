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
- `internal/store`: target home for the v2 store lifecycle.
- `pkg/owl`: small public API. Export what callers need; do not re-export every
  internal model detail by default.
- `cmd`: command wiring and rendering only.

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
- Move dotenv-comment parser helpers into `internal/projection/dotenv` before
  deleting or shrinking `internal/owl`.

## Verification

Before committing Owl code changes, run:

```bash
make check
runme run test
runme run lint
```

