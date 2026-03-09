# timepointlib

A Go library that provides a practical timepoint API:

1. `Create(...)`: capture registered in-scope stack/heap variables and caller program-counter metadata.
2. `p.Resume(...)`: restore variables and run a continuation callback.
3. `p.RestoreStack(...)`: restore stack-tagged variables only.
4. `p.RestoreHeap(...)`: restore heap-tagged variables only.
5. `p.ToString()`: describe the timepoint.

## Why variable registration is explicit

Go does not expose a safe API to automatically capture all in-scope locals and jump to a real machine instruction pointer. This library uses:

- Explicit variable registration (`StackVar`, `HeapVar`, `AnyVar`).
- A symbolic program counter (`file`, `line`, `function`, and optional label).
- A continuation callback (`WithResume`) executed by `Resume`.

## Option 2: automatic variable capture by instrumentation

This repository now includes a generator (`cmd/timepointgen`) that rewrites `timepoint.Create(...)` calls to inject all visible local variables automatically as `WithVariables(...)`.

How it works:

1. You write normal code with `timepoint.Create(...)` and no explicit `WithVariables(...)`.
2. Run the generator.
3. It rewrites each `Create` call into a form that captures all in-scope local variables.

Run it for the whole repo:

```bash
go run ./cmd/timepointgen -w .
```

Run it for a single folder:

```bash
go run ./cmd/timepointgen -w ./example_auto
```

## Run the example

```bash
go run ./example
```

## Run the auto-instrumented example

```bash
go generate ./example_auto
go run ./example_auto
```

## Run tests

The package includes documented unit tests for API behavior, error paths, and deep-copy semantics.
See [TESTS.md](./TESTS.md) for a user-friendly testing README (execution + objectives, in Spanish).

```bash
go test ./...
```
