package timepoint

import (
	"reflect"
	"testing"

	"timepointlib/internal/testx"
)

type aliasInt int
type aliasString string

// TestCoerceToTypeMatrix validates many assign/convert/error coercion combinations.
func TestCoerceToTypeMatrix(t *testing.T) {
	ifaceType := reflect.TypeOf((*any)(nil)).Elem()
	stringerType := reflect.TypeOf((*interface{ String() string })(nil)).Elem()

	tests := []struct {
		name        string
		value       any
		target      reflect.Type
		want        any
		wantErr     bool
		errContains string
	}{
		{name: "int to int", value: 1, target: reflect.TypeOf(0), want: 1},
		{name: "int32 to int64", value: int32(3), target: reflect.TypeOf(int64(0)), want: int64(3)},
		{name: "int to aliasInt", value: 7, target: reflect.TypeOf(aliasInt(0)), want: aliasInt(7)},
		{name: "aliasInt to int", value: aliasInt(9), target: reflect.TypeOf(0), want: 9},
		{name: "uint8 to uint16", value: uint8(4), target: reflect.TypeOf(uint16(0)), want: uint16(4)},
		{name: "float32 to float64", value: float32(1.5), target: reflect.TypeOf(float64(0)), want: float64(1.5)},
		{name: "complex64 to complex128", value: complex64(2 + 3i), target: reflect.TypeOf(complex128(0)), want: complex128(2 + 3i)},
		{name: "string to aliasString", value: "a", target: reflect.TypeOf(aliasString("")), want: aliasString("a")},
		{name: "aliasString to string", value: aliasString("b"), target: reflect.TypeOf(""), want: "b"},
		{name: "string to bytes", value: "abc", target: reflect.TypeOf([]byte{}), want: []byte("abc")},
		{name: "bytes to string", value: []byte("xyz"), target: reflect.TypeOf(""), want: "xyz"},
		{name: "bool assign", value: true, target: reflect.TypeOf(false), want: true},
		{name: "map assign", value: map[string]int{"a": 1}, target: reflect.TypeOf(map[string]int{}), want: map[string]int{"a": 1}},
		{name: "slice assign", value: []int{1, 2}, target: reflect.TypeOf([]int{}), want: []int{1, 2}},
		{name: "array assign", value: [2]int{1, 2}, target: reflect.TypeOf([2]int{}), want: [2]int{1, 2}},
		{name: "pointer assign", value: func() *int { v := 3; return &v }(), target: reflect.TypeOf((*int)(nil)), want: func() *int { v := 3; return &v }()},
		{name: "interface any", value: 11, target: ifaceType, want: 11},
		{name: "nil ptr", value: nil, target: reflect.TypeOf((*int)(nil)), want: (*int)(nil)},
		{name: "nil slice", value: nil, target: reflect.TypeOf([]int(nil)), want: []int(nil)},
		{name: "nil map", value: nil, target: reflect.TypeOf(map[string]int(nil)), want: map[string]int(nil)},
		{name: "nil chan", value: nil, target: reflect.TypeOf((chan int)(nil)), want: (chan int)(nil)},
		{name: "nil interface", value: nil, target: ifaceType, want: nil},

		{name: "string to int fails", value: "x", target: reflect.TypeOf(0), wantErr: true, errContains: "not assignable"},
		{name: "int to stringer fails", value: 3, target: stringerType, wantErr: true, errContains: "not assignable"},
		{name: "nil to int fails", value: nil, target: reflect.TypeOf(0), wantErr: true, errContains: "nil is not assignable"},
		{name: "slice type mismatch", value: []string{"a"}, target: reflect.TypeOf([]int{}), wantErr: true, errContains: "not assignable"},
		{name: "map type mismatch", value: map[string]int{"a": 1}, target: reflect.TypeOf(map[string]string{}), wantErr: true, errContains: "not assignable"},
		{name: "chan type mismatch", value: make(chan int), target: reflect.TypeOf((chan string)(nil)), wantErr: true, errContains: "not assignable"},
		{name: "array len mismatch", value: [2]int{1, 2}, target: reflect.TypeOf([3]int{}), wantErr: true, errContains: "not assignable"},
		{name: "struct mismatch", value: struct{ A int }{A: 1}, target: reflect.TypeOf(struct{ B int }{}), wantErr: true, errContains: "not assignable"},

		{name: "rune to int32", value: rune('a'), target: reflect.TypeOf(int32(0)), want: int32('a')},
		{name: "byte to int", value: byte(8), target: reflect.TypeOf(0), want: 8},
		{name: "int16 to int32", value: int16(22), target: reflect.TypeOf(int32(0)), want: int32(22)},
		{name: "uint16 to uint32", value: uint16(22), target: reflect.TypeOf(uint32(0)), want: uint32(22)},
		{name: "uintptr to uint64", value: uintptr(9), target: reflect.TypeOf(uint64(0)), want: uint64(9)},
		{name: "float64 to float32", value: float64(2.25), target: reflect.TypeOf(float32(0)), want: float32(2.25)},
		{name: "complex128 to complex64", value: complex128(1 + 2i), target: reflect.TypeOf(complex64(0)), want: complex64(1 + 2i)},
		{name: "alias string to bytes", value: aliasString("ok"), target: reflect.TypeOf([]byte{}), want: []byte("ok")},
		{name: "bytes to alias string", value: []byte("ok"), target: reflect.TypeOf(aliasString("")), want: aliasString("ok")},
		{name: "int to float64", value: 6, target: reflect.TypeOf(float64(0)), want: float64(6)},
		{name: "float32 to int", value: float32(6.9), target: reflect.TypeOf(0), want: 6},
		{name: "int to bool fails", value: 1, target: reflect.TypeOf(false), wantErr: true, errContains: "not assignable"},
		{name: "bool to int fails", value: false, target: reflect.TypeOf(0), wantErr: true, errContains: "not assignable"},
		{name: "func mismatch", value: func() {}, target: reflect.TypeOf(func(int) {}), wantErr: true, errContains: "not assignable"},
		{name: "pointer mismatch", value: func() *int { v := 1; return &v }(), target: reflect.TypeOf((*string)(nil)), wantErr: true, errContains: "not assignable"},

		{name: "uint to int", value: uint(4), target: reflect.TypeOf(0), want: 4},
		{name: "int to uint", value: 4, target: reflect.TypeOf(uint(0)), want: uint(4)},
		{name: "int to int8", value: 7, target: reflect.TypeOf(int8(0)), want: int8(7)},
		{name: "int8 to int64", value: int8(-5), target: reflect.TypeOf(int64(0)), want: int64(-5)},
		{name: "uint32 to float64", value: uint32(12), target: reflect.TypeOf(float64(0)), want: float64(12)},
		{name: "float64 to aliasInt", value: float64(4.0), target: reflect.TypeOf(aliasInt(0)), want: aliasInt(4)},
		{name: "string to rune slice", value: "ab", target: reflect.TypeOf([]rune{}), want: []rune{'a', 'b'}},
		{name: "rune slice to string", value: []rune{'a', 'b'}, target: reflect.TypeOf(""), want: "ab"},
		{name: "empty struct assign", value: struct{}{}, target: reflect.TypeOf(struct{}{}), want: struct{}{}},
		{name: "named to named same base", value: aliasInt(10), target: reflect.TypeOf(aliasInt(0)), want: aliasInt(10)},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := coerceToType(tt.value, tt.target)
			if tt.wantErr {
				testx.Error(t, err)
				if tt.errContains != "" {
					testx.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			testx.NoError(t, err)
			testx.Equal(t, tt.want, got.Interface())
		})
	}
}

// TestCanBeNilMatrix validates nilability decisions for many kinds.
func TestCanBeNilMatrix(t *testing.T) {
	tests := []struct {
		name   string
		target reflect.Type
		want   bool
	}{
		{name: "ptr", target: reflect.TypeOf((*int)(nil)), want: true},
		{name: "map", target: reflect.TypeOf(map[string]int(nil)), want: true},
		{name: "slice", target: reflect.TypeOf([]int(nil)), want: true},
		{name: "interface", target: reflect.TypeOf((*any)(nil)).Elem(), want: true},
		{name: "func", target: reflect.TypeOf((func())(nil)), want: true},
		{name: "chan", target: reflect.TypeOf((chan int)(nil)), want: true},
		{name: "int", target: reflect.TypeOf(0), want: false},
		{name: "string", target: reflect.TypeOf(""), want: false},
		{name: "bool", target: reflect.TypeOf(false), want: false},
		{name: "struct", target: reflect.TypeOf(struct{}{}), want: false},
		{name: "array", target: reflect.TypeOf([1]int{}), want: false},
		{name: "uintptr", target: reflect.TypeOf(uintptr(0)), want: false},
		{name: "float", target: reflect.TypeOf(float64(0)), want: false},
		{name: "complex", target: reflect.TypeOf(complex128(0)), want: false},
		{name: "alias int", target: reflect.TypeOf(aliasInt(0)), want: false},
		{name: "alias string", target: reflect.TypeOf(aliasString("")), want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			testx.Equal(t, tt.want, canBeNil(tt.target))
		})
	}
}

// TestDeepCopyPrimitiveMatrix validates deep-copy identity for scalar values.
func TestDeepCopyPrimitiveMatrix(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{name: "bool true", value: true},
		{name: "bool false", value: false},
		{name: "int", value: 1},
		{name: "int8", value: int8(-2)},
		{name: "int16", value: int16(-3)},
		{name: "int32", value: int32(-4)},
		{name: "int64", value: int64(-5)},
		{name: "uint", value: uint(2)},
		{name: "uint8", value: uint8(3)},
		{name: "uint16", value: uint16(4)},
		{name: "uint32", value: uint32(5)},
		{name: "uint64", value: uint64(6)},
		{name: "uintptr", value: uintptr(7)},
		{name: "float32", value: float32(1.25)},
		{name: "float64", value: float64(2.5)},
		{name: "complex64", value: complex64(1 + 2i)},
		{name: "complex128", value: complex128(3 + 4i)},
		{name: "string", value: "hello"},
		{name: "empty string", value: ""},
		{name: "alias int", value: aliasInt(9)},
		{name: "alias string", value: aliasString("x")},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := deepCopy(tt.value)
			testx.NoError(t, err)
			testx.Equal(t, tt.value, got)
		})
	}
}

// TestDeepCopyReferenceIsolationMatrix validates deep-copy isolation for reference-containing values.
func TestDeepCopyReferenceIsolationMatrix(t *testing.T) {
	type payload struct {
		A []int
		B map[string]int
		C *int
	}

	cases := []struct {
		name   string
		value  payload
		mutate func(*payload)
		check  func(t *testing.T, copied payload)
	}{
		{
			name:  "slice mutation isolated",
			value: payload{A: []int{1, 2, 3}},
			mutate: func(p *payload) {
				p.A[0] = 99
			},
			check: func(t *testing.T, copied payload) {
				testx.Equal(t, []int{1, 2, 3}, copied.A)
			},
		},
		{
			name:  "map mutation isolated",
			value: payload{B: map[string]int{"a": 1}},
			mutate: func(p *payload) {
				p.B["a"] = 42
			},
			check: func(t *testing.T, copied payload) {
				testx.Equal(t, 1, copied.B["a"])
			},
		},
		{
			name: "pointer mutation isolated",
			value: func() payload {
				v := 5
				return payload{C: &v}
			}(),
			mutate: func(p *payload) {
				*p.C = 88
			},
			check: func(t *testing.T, copied payload) {
				testx.Equal(t, 5, *copied.C)
			},
		},
		{
			name: "combined mutation isolated",
			value: func() payload {
				v := 3
				return payload{A: []int{7}, B: map[string]int{"k": 2}, C: &v}
			}(),
			mutate: func(p *payload) {
				p.A[0] = 70
				p.B["k"] = 20
				*p.C = 30
			},
			check: func(t *testing.T, copied payload) {
				testx.Equal(t, []int{7}, copied.A)
				testx.Equal(t, 2, copied.B["k"])
				testx.Equal(t, 3, *copied.C)
			},
		},
	}

	for i := 0; i < 5; i++ {
		for _, base := range cases {
			base := base
			name := base.name + " #" + string(rune('A'+i))
			t.Run(name, func(t *testing.T) {
				copiedAny, err := deepCopy(base.value)
				testx.NoError(t, err)
				copied, ok := copiedAny.(payload)
				testx.True(t, ok, "deepCopy should preserve payload type")

				originalAny, err := deepCopy(base.value)
				testx.NoError(t, err)
				original, ok := originalAny.(payload)
				testx.True(t, ok, "deepCopy should preserve payload type")
				base.mutate(&original)
				base.check(t, copied)
			})
		}
	}
}

// TestCreateRestoreRoundTripMatrix validates many value types through Create + RestoreStack.
func TestCreateRestoreRoundTripMatrix(t *testing.T) {
	types := []struct {
		name    string
		initial any
		mutated any
	}{
		{name: "int", initial: 1, mutated: 9},
		{name: "int32", initial: int32(2), mutated: int32(8)},
		{name: "int64", initial: int64(3), mutated: int64(7)},
		{name: "uint", initial: uint(4), mutated: uint(6)},
		{name: "float32", initial: float32(1.5), mutated: float32(2.5)},
		{name: "float64", initial: float64(2.5), mutated: float64(3.5)},
		{name: "bool", initial: true, mutated: false},
		{name: "string", initial: "a", mutated: "b"},
		{name: "slice", initial: []int{1, 2}, mutated: []int{9, 9}},
		{name: "map", initial: map[string]int{"x": 1}, mutated: map[string]int{"x": 7}},
		{name: "array", initial: [2]int{1, 2}, mutated: [2]int{9, 9}},
		{name: "struct", initial: struct{ A int }{A: 1}, mutated: struct{ A int }{A: 9}},
		{name: "aliasInt", initial: aliasInt(5), mutated: aliasInt(6)},
		{name: "aliasString", initial: aliasString("x"), mutated: aliasString("y")},
		{name: "bytes", initial: []byte("ab"), mutated: []byte("zz")},
		{name: "runes", initial: []rune{'a', 'b'}, mutated: []rune{'z'}},
		{name: "pointer", initial: func() *int { v := 4; return &v }(), mutated: func() *int { v := 7; return &v }()},
		{name: "nested struct", initial: struct{ X []int }{X: []int{1}}, mutated: struct{ X []int }{X: []int{8}}},
		{name: "map slice", initial: map[string][]int{"k": []int{1, 2}}, mutated: map[string][]int{"k": []int{9}}},
		{name: "slice map", initial: []map[string]int{{"a": 1}}, mutated: []map[string]int{{"a": 9}}},
	}

	for _, tt := range types {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cell := reflect.New(reflect.TypeOf(tt.initial))
			cell.Elem().Set(reflect.ValueOf(tt.initial))

			tp, err := Create(WithVariables(StackVar("v", cell.Interface())))
			testx.NoError(t, err)

			cell.Elem().Set(reflect.ValueOf(tt.mutated))
			testx.NoError(t, tp.RestoreStack(nil))
			testx.Equal(t, tt.initial, cell.Elem().Interface())
		})
	}
}
