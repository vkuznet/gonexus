package gonexus

import "reflect"

func toFloat64Slice(v interface{}) ([]float64, error) {
	if v == nil {
		return nil, newError("field has no data")
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		f, ok := scalarToFloat64(v)
		if !ok {
			return nil, newError("field data is not numeric (%T)", v)
		}
		return []float64{f}, nil
	}
	out := make([]float64, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		f, ok := scalarToFloat64(rv.Index(i).Interface())
		if !ok {
			return nil, newError("field data is not numeric (%T)", v)
		}
		out[i] = f
	}
	return out, nil
}

func scalarToFloat64(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	default:
		return 0, false
	}
}

func toInt64Slice(v interface{}) ([]int64, error) {
	if v == nil {
		return nil, newError("field has no data")
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		i, ok := scalarToInt64(v)
		if !ok {
			return nil, newError("field data is not an integer type (%T)", v)
		}
		return []int64{i}, nil
	}
	out := make([]int64, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		val, ok := scalarToInt64(rv.Index(i).Interface())
		if !ok {
			return nil, newError("field data is not an integer type (%T)", v)
		}
		out[i] = val
	}
	return out, nil
}

func scalarToInt64(v interface{}) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int8:
		return int64(x), true
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case uint:
		return int64(x), true
	case uint8:
		return int64(x), true
	case uint16:
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint64:
		return int64(x), true
	default:
		return 0, false
	}
}
