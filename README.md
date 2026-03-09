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

## Ejemplos completos (con transformación AST)

Los 3 ejemplos siguientes muestran el flujo end-to-end:

1. Escribes código con `timepoint.Create(...)` sin `WithVariables(...)`.
2. Ejecutas el transformador AST (`timepointgen`).
3. Ejecutas el programa ya instrumentado.

### Ejemplo 1: checkpoint + `Resume` con override

Código fuente (antes de transformar):

```go
package main

import (
	"fmt"
	"timepointlib/timepoint"
)

func main() {
	step := 1
	status := "new"

	p, _ := timepoint.Create(
		timepoint.WithName("order-checkpoint"),
		timepoint.WithResume(func(*timepoint.Timepoint) error {
			fmt.Println("resume:", step, status)
			return nil
		}),
	)

	step = 99
	status = "mutated"
	_ = p.Resume(map[string]any{"status": "overridden"})
}
```

Transformación AST:

```bash
go run ./cmd/timepointgen -w ./ruta/del/ejemplo1
```

Resultado esperado de la transformación (simplificado):

```go
p, _ := timepoint.Create(
	timepoint.WithVariables(
		timepoint.AnyVar("step", &step),
		timepoint.AnyVar("status", &status),
	),
	timepoint.WithName("order-checkpoint"),
	timepoint.WithResume(...),
)
```

Ejecución:

```bash
go run ./ruta/del/ejemplo1
```

### Ejemplo 2: `RestoreStack` + `RestoreHeap`

Código fuente (antes de transformar):

```go
package main

import "timepointlib/timepoint"

type Session struct{ Quota int }

func main() {
	counter := 10
	session := &Session{Quota: 3}

	p, _ := timepoint.Create(timepoint.WithName("partial-restore"))

	counter = 50
	session.Quota = 0

	_ = p.RestoreStack(nil) // restaura variables marcadas para stack
	_ = p.RestoreHeap(nil)  // restaura variables marcadas para heap
}
```

Transformación AST:

```bash
go run ./cmd/timepointgen -w ./ruta/del/ejemplo2
```

Instrumentación generada (simplificada):

```go
p, _ := timepoint.Create(
	timepoint.WithVariables(
		timepoint.AnyVar("counter", &counter),
		timepoint.AnyVar("session", &session),
	),
	timepoint.WithName("partial-restore"),
)
```

Ejecución:

```bash
go run ./ruta/del/ejemplo2
```

### Ejemplo 3: flujo automático con `go generate`

Código fuente:

```go
package main

import "timepointlib/timepoint"

//go:generate go run ../cmd/timepointgen -w .

func main() {
	value := 7
	p, _ := timepoint.Create(timepoint.WithName("auto-generate"))
	value = 100
	_ = p.Resume(nil)
}
```

Proceso completo:

```bash
go generate ./ruta/del/ejemplo3
go run ./ruta/del/ejemplo3
```

Con este patrón, el paso AST queda integrado en el flujo de build del ejemplo/proyecto.

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
