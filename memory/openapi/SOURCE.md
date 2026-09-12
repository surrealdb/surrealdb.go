# Vendored spec provenance

`enduser.swagger.json` is a verbatim copy of `doc/spec/enduser.swagger.json` on
`main` of [`surrealdb/spectron`](https://github.com/surrealdb/spectron). It is
vendored so a future sync is a diff against a known baseline, not a guess.

| | |
| --- | --- |
| Source | `surrealdb/spectron@main:doc/spec/enduser.swagger.json` |
| Commit | `39ce1e632e09867e5eadac1bad4100830c62ec51` (2026-09-11) |
| Spec version | `0.3.0` |
| Vendored | 2026-09-12 |

## Re-syncing

```bash
gh api repos/surrealdb/spectron/contents/doc/spec/enduser.swagger.json \
  --jq .content | base64 -d > memory/openapi/enduser.swagger.json
```

Then diff against the previous copy and reconcile the hand-written Go types:

```bash
git diff --stat memory/openapi/enduser.swagger.json
```

Update the table above with the new commit and version.

## Coverage

Every operation in this spec is implemented. `memory/coverage_test.go` asserts
that — it maps each `operationId` to its Go method and fails when the spec
grows one that is unmapped — and `TestGoTypesMatchTheVendoredSpec` in the same
file compares every bound Go struct's json tags against the schema properties,
so an added or renamed field fails the build rather than decoding to a zero
value in silence.

`POST /scope-grants` is implemented but is a server-side stub returning 501.
