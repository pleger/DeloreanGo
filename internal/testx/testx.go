package testx

import (
	"reflect"
	"strings"
	"testing"
)

// Equal fails the test when got != want (using reflect.DeepEqual).
func Equal(t testing.TB, want, got any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("not equal: want=%#v got=%#v", want, got)
	}
}

// True fails the test when v is false.
func True(t testing.TB, v bool, msg string) {
	t.Helper()
	if !v {
		if msg == "" {
			msg = "expected true"
		}
		t.Fatalf(msg)
	}
}

// False fails the test when v is true.
func False(t testing.TB, v bool, msg string) {
	t.Helper()
	if v {
		if msg == "" {
			msg = "expected false"
		}
		t.Fatalf(msg)
	}
}

// NoError fails the test when err is not nil.
func NoError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Error fails the test when err is nil.
func Error(t testing.TB, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Contains fails the test when s does not contain substr.
func Contains(t testing.TB, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("expected %q to contain %q", s, substr)
	}
}

// NotContains fails the test when s contains substr.
func NotContains(t testing.TB, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Fatalf("expected %q not to contain %q", s, substr)
	}
}

// Nil fails the test when v is not nil.
func Nil(t testing.TB, v any) {
	t.Helper()
	if !isNil(v) {
		t.Fatalf("expected nil, got %#v", v)
	}
}

// NotNil fails the test when v is nil.
func NotNil(t testing.TB, v any) {
	t.Helper()
	if isNil(v) {
		t.Fatal("expected non-nil value")
	}
}

func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface, reflect.Func, reflect.Chan:
		return rv.IsNil()
	default:
		return false
	}
}
