package timepoint

import (
	"fmt"
	"reflect"
)

func deepCopy(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	seen := make(map[visit]reflect.Value)
	copied, err := deepCopyValue(reflect.ValueOf(v), seen)
	if err != nil {
		return nil, err
	}
	return copied.Interface(), nil
}

type visit struct {
	ptr uintptr
	t   reflect.Type
}

func deepCopyValue(v reflect.Value, seen map[visit]reflect.Value) (reflect.Value, error) {
	if !v.IsValid() {
		return v, nil
	}

	switch v.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128,
		reflect.String:
		return v, nil
	case reflect.Ptr:
		if v.IsNil() {
			return reflect.Zero(v.Type()), nil
		}
		key := visit{ptr: v.Pointer(), t: v.Type()}
		if found, ok := seen[key]; ok {
			return found, nil
		}
		clone := reflect.New(v.Type().Elem())
		seen[key] = clone
		elem, err := deepCopyValue(v.Elem(), seen)
		if err != nil {
			return reflect.Value{}, err
		}
		clone.Elem().Set(elem)
		return clone, nil
	case reflect.Interface:
		if v.IsNil() {
			return reflect.Zero(v.Type()), nil
		}
		elem, err := deepCopyValue(v.Elem(), seen)
		if err != nil {
			return reflect.Value{}, err
		}
		out := reflect.New(v.Type()).Elem()
		out.Set(elem)
		return out, nil
	case reflect.Struct:
		clone := reflect.New(v.Type()).Elem()
		clone.Set(v)
		for i := 0; i < v.NumField(); i++ {
			field := clone.Field(i)
			if !field.CanSet() {
				continue
			}
			copied, err := deepCopyValue(v.Field(i), seen)
			if err != nil {
				return reflect.Value{}, err
			}
			field.Set(copied)
		}
		return clone, nil
	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type()), nil
		}
		clone := reflect.MakeSlice(v.Type(), v.Len(), v.Cap())
		for i := 0; i < v.Len(); i++ {
			copied, err := deepCopyValue(v.Index(i), seen)
			if err != nil {
				return reflect.Value{}, err
			}
			clone.Index(i).Set(copied)
		}
		return clone, nil
	case reflect.Array:
		clone := reflect.New(v.Type()).Elem()
		for i := 0; i < v.Len(); i++ {
			copied, err := deepCopyValue(v.Index(i), seen)
			if err != nil {
				return reflect.Value{}, err
			}
			clone.Index(i).Set(copied)
		}
		return clone, nil
	case reflect.Map:
		if v.IsNil() {
			return reflect.Zero(v.Type()), nil
		}
		clone := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			k, err := deepCopyValue(iter.Key(), seen)
			if err != nil {
				return reflect.Value{}, err
			}
			val, err := deepCopyValue(iter.Value(), seen)
			if err != nil {
				return reflect.Value{}, err
			}
			clone.SetMapIndex(k, val)
		}
		return clone, nil
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		// Non-copyable runtime values are shared by reference.
		return v, nil
	default:
		return reflect.Value{}, fmt.Errorf("unsupported kind: %s", v.Kind())
	}
}
