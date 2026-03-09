# README de Tests

Este documento explica:

1. Cómo ejecutar los tests.
2. Cuál es el objetivo de cada suite.

## Requisitos

- Go instalado y disponible en `PATH`.
- Ejecutar desde la raíz del proyecto.

## Cómo ejecutar los tests

Ejecución completa (recomendada):

```bash
mkdir -p .gocache
GOCACHE=$(pwd)/.gocache go test ./...
```

Ejecución con detalle (`-v`):

```bash
mkdir -p .gocache
GOCACHE=$(pwd)/.gocache go test ./... -v
```

Contar escenarios ejecutados (tests + subtests):

```bash
mkdir -p .gocache
GOCACHE=$(pwd)/.gocache go test ./... -v | rg '^=== RUN|^    --- RUN' | wc -l
```

## Objetivo de los tests

### `timepoint/timepoint_test.go`
Objetivo: validar el comportamiento público principal de la librería.

Cubre:

- Creación de snapshots (`Create`) y metadata.
- Restauración por alcance (`RestoreStack`, `RestoreHeap`).
- Reanudación con callback (`Resume`).
- Manejo de errores y validaciones.
- Representación textual (`ToString`).

### `timepoint/deepcopy_test.go`
Objetivo: validar la semántica de copia profunda y coerción de tipos.

Cubre:

- Copias de estructuras anidadas.
- Preservación de ciclos y referencias compartidas.
- Casos especiales (funciones/canales por referencia).
- Reglas de `coerceToType` (éxitos y errores).

### `timepoint/timepoint_matrix_test.go`
Objetivo: cubrir una matriz amplia de escenarios para evitar regresiones.

Cubre:

- Combinaciones de conversión en `coerceToType`.
- Reglas de nilabilidad en `canBeNil`.
- Matrices de `deepCopy` para tipos primitivos y de referencia.
- Round-trip `Create` + restore con múltiples tipos de datos.

### `cmd/timepointgen/main_test.go`
Objetivo: validar el generador de instrumentación automática.

Cubre:

- Detección de imports de `timepoint`.
- Selección de scope interno.
- Inyección AST de `WithVariables(...)` en `Create(...)`.

## Librería de aserciones usada

Todos los archivos de test usan la librería interna:

- `internal/testx`

Ventaja: aserciones más legibles (`testx.Equal`, `testx.NoError`, `testx.Contains`, etc.) y estilo consistente en toda la base de tests.
