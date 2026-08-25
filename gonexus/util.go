package gonexus

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// inferDtype determines a NeXus/NumPy-style dtype string and shape for a Go
// value being wrapped into an NXfield or NXattr. Scalars get a nil shape;
// slices get a one-dimensional shape (nested slices for true multi-
// dimensional arrays are supported via NXfield.Shape being set explicitly).
func inferDtype(value interface{}) (string, []int) {
	if value == nil {
		return "none", nil
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		if rv.Len() == 0 {
			return elemDtype(rv.Type().Elem()), []int{0}
		}
		elemType := rv.Type().Elem()
		return elemDtype(elemType), []int{rv.Len()}
	default:
		return elemDtype(rv.Type()), nil
	}
}

func elemDtype(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		return elemDtype(t.Elem())
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int64:
		return "int64"
	case reflect.Int32:
		return "int32"
	case reflect.Int16:
		return "int16"
	case reflect.Int8:
		return "int8"
	case reflect.Uint, reflect.Uint64:
		return "uint64"
	case reflect.Uint32:
		return "uint32"
	case reflect.Uint16:
		return "uint16"
	case reflect.Uint8:
		return "uint8"
	case reflect.Float64:
		return "float64"
	case reflect.Float32:
		return "float32"
	default:
		return "object"
	}
}

// formatValue renders a scalar or small array compactly for tree/dir
// output, similar to Python's NXfield.__repr__.
func formatValue(value interface{}) string {
	if value == nil {
		return "None"
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		n := rv.Len()
		if n == 0 {
			return "[]"
		}
		const maxShown = 3
		parts := []string{}
		show := n
		truncated := false
		if show > maxShown*2 {
			show = maxShown
			truncated = true
		}
		for i := 0; i < show; i++ {
			parts = append(parts, formatScalar(rv.Index(i).Interface()))
		}
		if truncated {
			parts = append(parts, "...")
			for i := n - maxShown; i < n; i++ {
				parts = append(parts, formatScalar(rv.Index(i).Interface()))
			}
		}
		return "[" + strings.Join(parts, " ") + "]"
	}
	return formatScalar(value)
}

func formatScalar(v interface{}) string {
	switch x := v.(type) {
	case float32:
		return strconv.FormatFloat(float64(x), 'g', 6, 32)
	case float64:
		return strconv.FormatFloat(x, 'g', 6, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// shapeString renders a shape as HDF5/nexusformat does, e.g. "631x461x4".
func shapeString(shape []int) string {
	parts := make([]string, len(shape))
	for i, s := range shape {
		parts[i] = strconv.Itoa(s)
	}
	return strings.Join(parts, "x")
}

var naturalSortRe = regexp.MustCompile(`(\d+)`)

// naturalSortKeys sorts strings so that embedded numbers compare
// numerically (e.g. "label_9" before "label_10"), matching the Python
// natural_sort() helper used when ordering NXfield/NXgroup children.
func naturalSortKeys(keys []string) {
	sort.Slice(keys, func(i, j int) bool {
		return naturalLess(keys[i], keys[j])
	})
}

func naturalLess(a, b string) bool {
	as := naturalSortRe.Split(a, -1)
	an := naturalSortRe.FindAllString(a, -1)
	bs := naturalSortRe.Split(b, -1)
	bn := naturalSortRe.FindAllString(b, -1)
	// Interleave text/number tokens for comparison.
	amerged := interleave(as, an)
	bmerged := interleave(bs, bn)
	n := len(amerged)
	if len(bmerged) < n {
		n = len(bmerged)
	}
	for i := 0; i < n; i++ {
		av, aIsNum := amerged[i], isNumeric(amerged[i])
		bv, bIsNum := bmerged[i], isNumeric(bmerged[i])
		if aIsNum && bIsNum {
			ai, _ := strconv.Atoi(av)
			bi, _ := strconv.Atoi(bv)
			if ai != bi {
				return ai < bi
			}
			continue
		}
		if av != bv {
			return av < bv
		}
	}
	return len(amerged) < len(bmerged)
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func interleave(text, nums []string) []string {
	out := make([]string, 0, len(text)+len(nums))
	for i := 0; i < len(text); i++ {
		if text[i] != "" {
			out = append(out, text[i])
		}
		if i < len(nums) {
			out = append(out, nums[i])
		}
	}
	return out
}
