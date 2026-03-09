package timepoint

import (
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Scope describes where a variable lives from the perspective of the snapshot API.
type Scope uint8

const (
	ScopeStack Scope = 1 << iota
	ScopeHeap
	ScopeBoth = ScopeStack | ScopeHeap
)

func (s Scope) String() string {
	switch s {
	case ScopeStack:
		return "stack"
	case ScopeHeap:
		return "heap"
	case ScopeBoth:
		return "stack+heap"
	default:
		return "unknown"
	}
}

// Variable registers one in-scope variable that should be captured.
// Ptr must be a non-nil pointer to the variable.
type Variable struct {
	Name  string
	Ptr   any
	Scope Scope
}

func StackVar(name string, ptr any) Variable { return Variable{Name: name, Ptr: ptr, Scope: ScopeStack} }
func HeapVar(name string, ptr any) Variable  { return Variable{Name: name, Ptr: ptr, Scope: ScopeHeap} }
func AnyVar(name string, ptr any) Variable   { return Variable{Name: name, Ptr: ptr, Scope: ScopeBoth} }

// ResumeFunc is called by Resume after state restoration.
type ResumeFunc func(*Timepoint) error

// ProgramCounter stores symbolic resume metadata for the create call site.
type ProgramCounter struct {
	Function string
	File     string
	Line     int
	Label    string
}

type savedVariable struct {
	name  string
	ptr   any
	typ   reflect.Type
	scope Scope
	value any
}

// Timepoint is a snapshot of selected in-scope variables plus symbolic program counter metadata.
type Timepoint struct {
	mu        sync.Mutex
	name      string
	createdAt time.Time
	pc        ProgramCounter
	resumeFn  ResumeFunc
	vars      map[string]*savedVariable
}

type createConfig struct {
	name      string
	variables []Variable
	overrides map[string]any
	resumeFn  ResumeFunc
	pcLabel   string
}

// CreateOption customizes Create.
type CreateOption func(*createConfig) error

func WithName(name string) CreateOption {
	return func(c *createConfig) error {
		c.name = strings.TrimSpace(name)
		return nil
	}
}

func WithVariables(vars ...Variable) CreateOption {
	return func(c *createConfig) error {
		c.variables = append(c.variables, vars...)
		return nil
	}
}

func WithOverrides(overrides map[string]any) CreateOption {
	return func(c *createConfig) error {
		if len(overrides) == 0 {
			return nil
		}
		if c.overrides == nil {
			c.overrides = make(map[string]any, len(overrides))
		}
		for k, v := range overrides {
			c.overrides[k] = v
		}
		return nil
	}
}

func WithResume(fn ResumeFunc) CreateOption {
	return func(c *createConfig) error {
		c.resumeFn = fn
		return nil
	}
}

func WithProgramCounter(label string) CreateOption {
	return func(c *createConfig) error {
		c.pcLabel = strings.TrimSpace(label)
		return nil
	}
}

// Create captures the current value of registered in-scope variables and saves caller metadata.
//
// Go cannot capture all in-scope variables or jump to a true instruction pointer automatically.
// This API uses explicit variable registration plus a continuation callback (WithResume) as a
// practical, safe approximation.
//
// To get default "capture all locals in scope" behavior, run the source instrumentation tool:
// `go run ./cmd/timepointgen -w .`
func Create(opts ...CreateOption) (*Timepoint, error) {
	cfg := createConfig{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	tp := &Timepoint{
		name:      cfg.name,
		createdAt: time.Now(),
		pc:        captureProgramCounter(cfg.pcLabel),
		resumeFn:  cfg.resumeFn,
		vars:      make(map[string]*savedVariable, len(cfg.variables)),
	}

	for _, v := range cfg.variables {
		if err := tp.captureVariable(v, cfg.overrides[v.Name]); err != nil {
			return nil, err
		}
	}

	return tp, nil
}

func (t *Timepoint) captureVariable(v Variable, override any) error {
	name := strings.TrimSpace(v.Name)
	if name == "" {
		return fmt.Errorf("variable name cannot be empty")
	}
	if _, exists := t.vars[name]; exists {
		return fmt.Errorf("variable %q registered multiple times", name)
	}
	if v.Scope == 0 {
		v.Scope = ScopeBoth
	}

	rv := reflect.ValueOf(v.Ptr)
	if !rv.IsValid() || rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("variable %q must be a non-nil pointer", name)
	}

	typ := rv.Elem().Type()
	value := rv.Elem().Interface()
	if override != nil {
		coerced, err := coerceToType(override, typ)
		if err != nil {
			return fmt.Errorf("override for %q: %w", name, err)
		}
		value = coerced.Interface()
	}

	copied, err := deepCopy(value)
	if err != nil {
		return fmt.Errorf("copy variable %q: %w", name, err)
	}

	t.vars[name] = &savedVariable{
		name:  name,
		ptr:   v.Ptr,
		typ:   typ,
		scope: v.Scope,
		value: copied,
	}
	return nil
}

func captureProgramCounter(label string) ProgramCounter {
	pc := ProgramCounter{Label: strings.TrimSpace(label)}
	pcs := make([]uintptr, 1)
	n := runtime.Callers(3, pcs)
	if n == 0 {
		return pc
	}
	fn := runtime.FuncForPC(pcs[0] - 1)
	if fn == nil {
		return pc
	}
	file, line := fn.FileLine(pcs[0] - 1)
	pc.Function = fn.Name()
	pc.File = file
	pc.Line = line
	return pc
}

func (t *Timepoint) RestoreStack(overrides map[string]any) error {
	return t.restore(ScopeStack, overrides)
}

func (t *Timepoint) RestoreHeap(overrides map[string]any) error {
	return t.restore(ScopeHeap, overrides)
}

func (t *Timepoint) Resume(overrides map[string]any) error {
	if err := t.restore(ScopeBoth, overrides); err != nil {
		return err
	}
	if t.resumeFn != nil {
		return t.resumeFn(t)
	}
	return nil
}

func (t *Timepoint) restore(scope Scope, overrides map[string]any) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for name, sv := range t.vars {
		if sv.scope&scope == 0 {
			continue
		}

		source := sv.value
		if overrides != nil {
			if v, ok := overrides[name]; ok {
				source = v
			}
		}

		coerced, err := coerceToType(source, sv.typ)
		if err != nil {
			return fmt.Errorf("restore %q: %w", name, err)
		}
		copied, err := deepCopy(coerced.Interface())
		if err != nil {
			return fmt.Errorf("restore %q: %w", name, err)
		}
		final, err := coerceToType(copied, sv.typ)
		if err != nil {
			return fmt.Errorf("restore %q: %w", name, err)
		}

		rv := reflect.ValueOf(sv.ptr)
		rv.Elem().Set(final)
	}
	return nil
}

func (t *Timepoint) Name() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.name
}

func (t *Timepoint) ProgramCounter() ProgramCounter {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.pc
}

func (t *Timepoint) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	names := make([]string, 0, len(t.vars))
	for name := range t.vars {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	fmt.Fprintf(&b, "Timepoint{name=%q, createdAt=%s, pc=%s:%d, function=%q", t.name, t.createdAt.Format(time.RFC3339), t.pc.File, t.pc.Line, t.pc.Function)
	if t.pc.Label != "" {
		fmt.Fprintf(&b, ", label=%q", t.pc.Label)
	}
	b.WriteString(", vars=[")
	for i, n := range names {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s(%s)", n, t.vars[n].scope.String())
	}
	b.WriteString("]}")
	return b.String()
}

// ToString is provided for API symmetry with languages that use toString().
func (t *Timepoint) ToString() string {
	return t.String()
}

func coerceToType(value any, target reflect.Type) (reflect.Value, error) {
	if value == nil {
		if canBeNil(target) {
			return reflect.Zero(target), nil
		}
		return reflect.Value{}, fmt.Errorf("nil is not assignable to %s", target)
	}

	rv := reflect.ValueOf(value)
	rt := rv.Type()

	if rt.AssignableTo(target) {
		return rv, nil
	}
	if rt.ConvertibleTo(target) {
		return rv.Convert(target), nil
	}
	if target.Kind() == reflect.Interface && rt.Implements(target) {
		return rv, nil
	}
	return reflect.Value{}, fmt.Errorf("type %s is not assignable to %s", rt, target)
}

func canBeNil(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface, reflect.Func, reflect.Chan:
		return true
	default:
		return false
	}
}
