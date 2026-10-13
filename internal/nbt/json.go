package nbt

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
)

// FromJSON converts JSON codec data into a typed NBT tree.
func FromJSON(data []byte) (byte, any, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return 0, nil, err
	}
	return jsonTag(v, 0)
}

// jsonTag converts one bounded JSON value into NBT.
func jsonTag(v any, depth int) (byte, any, error) {
	if depth > 64 {
		return 0, nil, fmt.Errorf("JSON NBT nesting exceeds limit")
	}
	switch v := v.(type) {
	case bool:
		var b int8
		if v {
			b = 1
		}
		return TagByte, b, nil
	case string:
		return TagString, v, nil
	case float64:
		if v == math.Trunc(v) && v >= math.MinInt32 && v <= math.MaxInt32 {
			return TagInt, int32(v), nil
		}
		return TagDouble, v, nil
	case map[string]any:
		c := NewCompound()
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			t, x, e := jsonTag(v[key], depth+1)
			if e != nil {
				return 0, nil, e
			}
			c.PutTag(key, t, x)
		}
		return TagCompound, c, nil
	case []any:
		l := NewList(TagCompound)
		var numeric bool
		for i, x := range v {
			t, n, e := jsonTag(x, depth+1)
			if e != nil {
				return 0, nil, e
			}
			if i == 0 {
				l = NewList(t)
				numeric = t == TagInt || t == TagDouble
			}
			if l.ElementType() != t {
				if numeric && (t == TagInt || t == TagDouble) {
					promoted := NewList(TagDouble)
					for _, old := range l.Elements() {
						if n, ok := old.(int32); ok {
							promoted.Add(float64(n))
						} else {
							promoted.Add(old)
						}
					}
					l = promoted
					if n32, ok := n.(int32); ok {
						n = float64(n32)
					}
				} else {
					return 0, nil, fmt.Errorf("mixed JSON list types")
				}
			}
			l.Add(n)
		}
		return TagList, l, nil
	default:
		return 0, nil, fmt.Errorf("unsupported JSON NBT value %T", v)
	}
}

// ToJSON serializes a bounded NBT codec value as JSON.
func ToJSON(value any) ([]byte, error) {
	decoded, err := jsonValue(value, 0)
	if err != nil {
		return nil, err
	}
	return json.Marshal(decoded)
}

// jsonValue converts persistent NBT types into JSON codec values.
func jsonValue(value any, depth int) (any, error) {
	if depth > 64 {
		return nil, fmt.Errorf("NBT JSON nesting exceeds limit")
	}
	switch v := value.(type) {
	case *Compound:
		result := make(map[string]any, len(v.Keys()))
		for _, key := range v.Keys() {
			x, _, _ := v.Get(key)
			decoded, err := jsonValue(x, depth+1)
			if err != nil {
				return nil, err
			}
			result[key] = decoded
		}
		return result, nil
	case *List:
		result := make([]any, v.Len())
		for i, x := range v.Elements() {
			decoded, err := jsonValue(x, depth+1)
			if err != nil {
				return nil, err
			}
			result[i] = decoded
		}
		return result, nil
	case int8:
		return v != 0, nil
	case int16, int32, int64, float32, float64, string, []int32, []int64:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported NBT JSON value %T", value)
	}
}
