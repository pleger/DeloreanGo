package timepoint

import (
	"errors"
	"strings"
	"testing"

	"timepointlib/internal/testx"
)

type testSession struct {
	User  string
	Quota int
	Tags  []string
}

//go:noinline
func createWithPC(opts ...CreateOption) (*Timepoint, error) {
	return Create(opts...)
}

// TestCreateCapturesMetadata verifies that Create stores name and symbolic PC info.
func TestCreateCapturesMetadata(t *testing.T) {
	x := 1
	tp, err := createWithPC(
		WithName("  checkpoint-1  "),
		WithProgramCounter("  after-step  "),
		WithVariables(StackVar("x", &x)),
	)
	testx.NoError(t, err)

	testx.Equal(t, "checkpoint-1", tp.Name())
	pc := tp.ProgramCounter()
	testx.Equal(t, "after-step", pc.Label)
	testx.True(t, pc.File != "", "program counter file should not be empty")
	testx.True(t, pc.Function != "", "program counter function should not be empty")
	testx.True(t, pc.Line > 0, "line should be > 0")
}

// TestCreateAppliesSnapshotOverrides verifies that creation-time overrides become snapshot state.
func TestCreateAppliesSnapshotOverrides(t *testing.T) {
	count := 3
	tp, err := Create(
		WithVariables(StackVar("count", &count)),
		WithOverrides(map[string]any{"count": 10}),
	)
	testx.NoError(t, err)

	count = 99
	testx.NoError(t, tp.RestoreStack(nil))
	testx.Equal(t, 10, count)
}

// TestCreatePropagatesOptionError verifies that failing options are returned directly.
func TestCreatePropagatesOptionError(t *testing.T) {
	sentinel := errors.New("option failed")
	bad := func(*createConfig) error { return sentinel }

	_, err := Create(bad)
	testx.True(t, errors.Is(err, sentinel), "Create should return option error")
}

// TestCreateRejectsInvalidVariableRegistrations verifies input validation for variable registration.
func TestCreateRejectsInvalidVariableRegistrations(t *testing.T) {
	value := 1
	var nilPtr *int

	tests := []struct {
		name    string
		opts    []CreateOption
		wantErr string
	}{
		{name: "empty name", opts: []CreateOption{WithVariables(StackVar(" ", &value))}, wantErr: "variable name cannot be empty"},
		{name: "duplicate name", opts: []CreateOption{WithVariables(StackVar("v", &value), StackVar("v", &value))}, wantErr: "registered multiple times"},
		{name: "nil pointer", opts: []CreateOption{WithVariables(StackVar("v", nilPtr))}, wantErr: "must be a non-nil pointer"},
		{name: "non pointer", opts: []CreateOption{WithVariables(Variable{Name: "v", Ptr: 123, Scope: ScopeStack})}, wantErr: "must be a non-nil pointer"},
		{name: "incompatible override", opts: []CreateOption{WithVariables(StackVar("v", &value)), WithOverrides(map[string]any{"v": "bad-type"})}, wantErr: "override for \"v\""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, err := Create(tt.opts...)
			testx.Error(t, err)
			testx.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestRestoreStackAndRestoreHeap verifies scope-specific restoration behavior.
func TestRestoreStackAndRestoreHeap(t *testing.T) {
	step := 1
	msg := "start"
	sess := &testSession{User: "ana", Quota: 5, Tags: []string{"x", "y"}}

	tp, err := Create(WithVariables(StackVar("step", &step), StackVar("msg", &msg), HeapVar("sess", &sess)))
	testx.NoError(t, err)

	step = 99
	msg = "mutated"
	sess.Quota = 0
	sess.Tags[0] = "changed"

	testx.NoError(t, tp.RestoreStack(nil))
	testx.Equal(t, 1, step)
	testx.Equal(t, "start", msg)
	testx.Equal(t, 0, sess.Quota)
	testx.Equal(t, "changed", sess.Tags[0])

	testx.NoError(t, tp.RestoreHeap(nil))
	testx.Equal(t, 5, sess.Quota)
	testx.Equal(t, "x", sess.Tags[0])
}

// TestRestoreUsesOverrides verifies runtime overrides for both stack and heap restoration.
func TestRestoreUsesOverrides(t *testing.T) {
	count := 2
	items := []string{"A", "B"}

	tp, err := Create(WithVariables(StackVar("count", &count), HeapVar("items", &items)))
	testx.NoError(t, err)

	count = 77
	items = []string{"x"}

	testx.NoError(t, tp.RestoreStack(map[string]any{"count": 42}))
	testx.Equal(t, 42, count)

	testx.NoError(t, tp.RestoreHeap(map[string]any{"items": []string{"N", "M"}}))
	testx.Equal(t, []string{"N", "M"}, items)
}

// TestRestoreRejectsIncompatibleOverride verifies type checking during restoration.
func TestRestoreRejectsIncompatibleOverride(t *testing.T) {
	n := 7
	tp, err := Create(WithVariables(StackVar("n", &n)))
	testx.NoError(t, err)

	err = tp.RestoreStack(map[string]any{"n": "bad"})
	testx.Error(t, err)
	testx.Contains(t, err.Error(), "not assignable")
}

// TestRestoreAllowsNilForNilableType verifies nil override support for pointer-typed variables.
func TestRestoreAllowsNilForNilableType(t *testing.T) {
	v := 9
	ptr := &v

	tp, err := Create(WithVariables(StackVar("ptr", &ptr)))
	testx.NoError(t, err)
	testx.NoError(t, tp.RestoreStack(map[string]any{"ptr": nil}))
	testx.Nil(t, ptr)
}

// TestRestoreRejectsNilForNonNilableType verifies nil safety for non-nilable values.
func TestRestoreRejectsNilForNonNilableType(t *testing.T) {
	v := 9
	tp, err := Create(WithVariables(StackVar("v", &v)))
	testx.NoError(t, err)

	err = tp.RestoreStack(map[string]any{"v": nil})
	testx.Error(t, err)
	testx.Contains(t, err.Error(), "nil is not assignable")
}

// TestVariableDefaultScopeIsBoth verifies zero-value scope defaults to restoring in both APIs.
func TestVariableDefaultScopeIsBoth(t *testing.T) {
	v := 5
	tp, err := Create(WithVariables(Variable{Name: "v", Ptr: &v}))
	testx.NoError(t, err)

	v = 100
	testx.NoError(t, tp.RestoreStack(nil))
	testx.Equal(t, 5, v)

	v = 200
	testx.NoError(t, tp.RestoreHeap(nil))
	testx.Equal(t, 5, v)
}

// TestResumeRestoresStateAndCallsContinuation verifies full restore + callback execution.
func TestResumeRestoresStateAndCallsContinuation(t *testing.T) {
	step := 1
	message := "snap"
	called := false

	tp, err := Create(
		WithVariables(StackVar("step", &step), StackVar("message", &message)),
		WithResume(func(*Timepoint) error {
			called = true
			if step != 1 {
				return errors.New("step not restored before callback")
			}
			if message != "override" {
				return errors.New("message override not applied")
			}
			return nil
		}),
	)
	testx.NoError(t, err)

	step = 50
	message = "mutated"
	testx.NoError(t, tp.Resume(map[string]any{"message": "override"}))
	testx.True(t, called, "resume callback must be called")
}

// TestResumePropagatesCallbackError verifies callback failures are returned unchanged.
func TestResumePropagatesCallbackError(t *testing.T) {
	v := 1
	sentinel := errors.New("resume failed")

	tp, err := Create(
		WithVariables(StackVar("v", &v)),
		WithResume(func(*Timepoint) error { return sentinel }),
	)
	testx.NoError(t, err)

	v = 99
	err = tp.Resume(nil)
	testx.True(t, errors.Is(err, sentinel), "resume should return callback error")
	testx.Equal(t, 1, v)
}

// TestToStringIncludesSortedVariables verifies text output contains stable variable ordering.
func TestToStringIncludesSortedVariables(t *testing.T) {
	a := 1
	b := 2

	tp, err := Create(
		WithName("tp"),
		WithProgramCounter("label"),
		WithVariables(StackVar("b", &b), StackVar("a", &a)),
	)
	testx.NoError(t, err)

	out := tp.ToString()
	for _, want := range []string{"Timepoint{", "name=\"tp\"", "label=\"label\"", "a(stack)", "b(stack)"} {
		testx.Contains(t, out, want)
	}
	testx.True(t, strings.Index(out, "a(stack)") < strings.Index(out, "b(stack)"), "variables should be sorted in ToString")
}

// TestScopeString verifies string output for all scope enum values.
func TestScopeString(t *testing.T) {
	tests := []struct {
		scope Scope
		want  string
	}{
		{scope: ScopeStack, want: "stack"},
		{scope: ScopeHeap, want: "heap"},
		{scope: ScopeBoth, want: "stack+heap"},
		{scope: Scope(255), want: "unknown"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.want, func(t *testing.T) {
			testx.Equal(t, tt.want, tt.scope.String())
		})
	}
}
