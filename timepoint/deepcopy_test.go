package timepoint

import (
	"reflect"
	"testing"

	"timepointlib/internal/testx"
)

type graphNode struct {
	Value int
	Next  *graphNode
}

type compositeValue struct {
	Name   string
	Nums   []int
	Lookup map[string]*graphNode
}

// TestDeepCopyHandlesNestedStructSliceMap verifies recursive copying across common container types.
func TestDeepCopyHandlesNestedStructSliceMap(t *testing.T) {
	original := compositeValue{
		Name: "orig",
		Nums: []int{1, 2, 3},
		Lookup: map[string]*graphNode{
			"n1": {Value: 7},
		},
	}

	copiedAny, err := deepCopy(original)
	testx.NoError(t, err)

	copied, ok := copiedAny.(compositeValue)
	testx.True(t, ok, "deepCopy result must be compositeValue")

	original.Nums[0] = 99
	original.Lookup["n1"].Value = 88

	testx.Equal(t, 1, copied.Nums[0])
	testx.Equal(t, 7, copied.Lookup["n1"].Value)
}

// TestDeepCopyPreservesPointerCycles verifies cycle-safe copy behavior for recursive pointers.
func TestDeepCopyPreservesPointerCycles(t *testing.T) {
	root := &graphNode{Value: 1}
	root.Next = root

	copiedAny, err := deepCopy(root)
	testx.NoError(t, err)

	copied, ok := copiedAny.(*graphNode)
	testx.True(t, ok, "deepCopy result must be *graphNode")
	testx.False(t, copied == root, "copied pointer must not alias original pointer")
	testx.True(t, copied.Next == copied, "copied cycle must point to copied node")
}

// TestDeepCopyPreservesSharedPointers verifies aliasing consistency in copied object graphs.
func TestDeepCopyPreservesSharedPointers(t *testing.T) {
	type wrapper struct {
		A *graphNode
		B *graphNode
	}

	shared := &graphNode{Value: 4}
	orig := wrapper{A: shared, B: shared}

	copiedAny, err := deepCopy(orig)
	testx.NoError(t, err)
	copied := copiedAny.(wrapper)

	testx.False(t, copied.A == shared || copied.B == shared, "copied pointers must not alias original")
	testx.True(t, copied.A == copied.B, "shared pointer relationship must be preserved")
}

// TestDeepCopyKeepsReferenceForChannelsAndFuncs verifies intentional by-reference behavior.
func TestDeepCopyKeepsReferenceForChannelsAndFuncs(t *testing.T) {
	ch := make(chan int, 1)
	fn := func() int { return 1 }

	chCopyAny, err := deepCopy(ch)
	testx.NoError(t, err)
	fnCopyAny, err := deepCopy(fn)
	testx.NoError(t, err)

	chCopy, ok := chCopyAny.(chan int)
	testx.True(t, ok, "channel result must be chan int")
	fnCopy, ok := fnCopyAny.(func() int)
	testx.True(t, ok, "func result must be func() int")

	testx.True(t, reflect.ValueOf(chCopy).Pointer() == reflect.ValueOf(ch).Pointer(), "channel pointer should be shared")
	testx.True(t, reflect.ValueOf(fnCopy).Pointer() == reflect.ValueOf(fn).Pointer(), "function pointer should be shared")
}

// TestCoerceToTypeSupportsAssignmentAndConversion verifies assignable/convertible coercion paths.
func TestCoerceToTypeSupportsAssignmentAndConversion(t *testing.T) {
	assignTarget := reflect.TypeOf("")
	convertedTarget := reflect.TypeOf(int64(0))

	v1, err := coerceToType("abc", assignTarget)
	testx.NoError(t, err)
	testx.Equal(t, "abc", v1.Interface().(string))

	v2, err := coerceToType(int32(5), convertedTarget)
	testx.NoError(t, err)
	testx.Equal(t, int64(5), v2.Interface().(int64))
}

// TestCoerceToTypeNilRules verifies nil behavior for nilable and non-nilable targets.
func TestCoerceToTypeNilRules(t *testing.T) {
	ptrType := reflect.TypeOf((*int)(nil))
	intType := reflect.TypeOf(0)

	v, err := coerceToType(nil, ptrType)
	testx.NoError(t, err)
	testx.True(t, v.IsNil(), "nil pointer coercion should return nil")

	_, err = coerceToType(nil, intType)
	testx.Error(t, err)
	testx.Contains(t, err.Error(), "nil is not assignable")
}

// TestCoerceToTypeRejectsMismatch verifies explicit type mismatch errors.
func TestCoerceToTypeRejectsMismatch(t *testing.T) {
	_, err := coerceToType("abc", reflect.TypeOf(0))
	testx.Error(t, err)
	testx.Contains(t, err.Error(), "not assignable")
}
