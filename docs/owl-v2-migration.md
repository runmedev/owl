# Owl v2 Migration Notes

## Owl v1 CRD Resolver Compatibility

Owl v2 intentionally does not carry forward the Owl v1 CRD-based resolver
surface. The old `.runme/owl.yaml` `EnvResolution` shape is not accepted as Owl
v2 migration input.

This is a deliberate compatibility break. Owl v2 moves forward with typed
environment state, projections, validation, safe snapshots, provenance, and a
cleaner GraphQL-backed core. New resolver behavior is built on Owl v2's resolver
model instead of preserving the old provider-specific GraphQL path.

If you depend on Owl v1 CRD / `EnvResolution` behavior, keep using the v1 path or
resolve those values outside Owl until equivalent v2 resolver support exists.
Owl v2 will not translate legacy resolver configuration automatically.

Release-note wording:

```text
Owl v2 intentionally does not carry forward the Owl v1 CRD-based resolver
surface. The old `.runme/owl.yaml` `EnvResolution` shape is not accepted as v2
migration input. The v2 cutover focuses on typed environment state, projections,
validation, safe snapshots, provenance, and a cleaner GraphQL-backed core. New
resolver behavior will be built on Owl v2's resolver model instead of preserving
the old provider-specific GraphQL path.
```
