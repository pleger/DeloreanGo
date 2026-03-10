# Timepoint Web IDE for Go

This web app is focused on **Go** and provides:

- Go code editor,
- timepoint creation (explicit or implicit),
- execution timeline visualization,
- variable editing at selected timepoints,
- branched timelines when resuming with edited variables.

Project repository: <https://github.com/pleger/Continuaciones_Go>

## Run locally

```bash
cd web_ide_timepoint
python3 -m http.server 8080
```

Open: `http://localhost:8080`

## Go support in the browser

The browser runtime currently executes a practical **Go subset** for debugging/timeline simulation:

- `package main`
- `func main() { ... }`
- `var x = ...`
- `x := ...`
- `x = ...`
- `if ... { ... } else { ... }`
- `fmt.Println(...)` / `fmt.Printf(...)`

This keeps the workflow Go-oriented while running fully client-side on GitHub Pages.

## Resume + branch behavior

- If no timepoint variable was edited: resume replays the same timeline from the selected timepoint.
- If variables were edited: resume creates a **new timeline branch** and executes with the new variable context.

## Panels

- **Panel 1**: Go source editor.
- **Panel 2**: output/errors.
- **Panel 3**: watch variables.
- **Panel 4**: timeline(s) with branch visualization.
- **Panel 5**: editable variables of selected timepoint.
- **Panel 6**: settings and run controls.
