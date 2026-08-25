package gonexus

import "fmt"

// NXattr represents a single NeXus/HDF5 attribute attached to a group or
// field. It is the Go equivalent of Python's NXattr class: a typed scalar
// (or, occasionally, small array) value with no children of its own.
type NXattr struct {
	// Value holds the attribute's data. Supported Go types are string,
	// bool, the signed/unsigned integer types, float32/float64, and
	// slices thereof for array-valued attributes.
	Value interface{}
	// Dtype is the NeXus/NumPy-style type name ("float64", "int32",
	// "string", ...), inferred automatically from Value unless set
	// explicitly.
	Dtype string
}

// NewAttr creates an NXattr, inferring its Dtype from the Go type of value.
func NewAttr(value interface{}) *NXattr {
	dtype, _ := inferDtype(value)
	return &NXattr{Value: value, Dtype: dtype}
}

func (a *NXattr) String() string {
	return fmt.Sprintf("%v", a.Value)
}

// AttrSet is an insertion-ordered map of attribute name to *NXattr. HDF5
// attribute order is not semantically meaningful, but preserving insertion
// order keeps NXgroup.Tree/NXfield.Tree output stable and human-friendly,
// matching the Python __repr__ behaviour.
type AttrSet struct {
	order []string
	m     map[string]*NXattr
}

// NewAttrSet returns an empty, ready-to-use AttrSet.
func NewAttrSet() *AttrSet {
	return &AttrSet{m: make(map[string]*NXattr)}
}

// Set adds or replaces the named attribute with a raw Go value, wrapping it
// in an NXattr automatically. Equivalent to Python's `obj.attrs['name'] = value`.
func (a *AttrSet) Set(name string, value interface{}) {
	if a.m == nil {
		a.m = make(map[string]*NXattr)
	}
	if attr, ok := value.(*NXattr); ok {
		a.SetAttr(name, attr)
		return
	}
	if _, exists := a.m[name]; !exists {
		a.order = append(a.order, name)
	}
	a.m[name] = NewAttr(value)
}

// SetAttr adds or replaces the named attribute with an already-built NXattr.
func (a *AttrSet) SetAttr(name string, attr *NXattr) {
	if a.m == nil {
		a.m = make(map[string]*NXattr)
	}
	if _, exists := a.m[name]; !exists {
		a.order = append(a.order, name)
	}
	a.m[name] = attr
}

// Get returns the named attribute and whether it was present.
// Equivalent to Python's `obj.attrs['name']`.
func (a *AttrSet) Get(name string) (*NXattr, bool) {
	if a.m == nil {
		return nil, false
	}
	v, ok := a.m[name]
	return v, ok
}

// GetString is a convenience accessor returning an attribute's value as a
// string (the common case for things like "units" or "long_name"), or ""
// if absent.
func (a *AttrSet) GetString(name string) string {
	attr, ok := a.Get(name)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", attr.Value)
}

// Delete removes the named attribute, if present.
// Equivalent to Python's `del obj.attrs['name']`.
func (a *AttrSet) Delete(name string) {
	if a.m == nil {
		return
	}
	if _, ok := a.m[name]; !ok {
		return
	}
	delete(a.m, name)
	for i, k := range a.order {
		if k == name {
			a.order = append(a.order[:i], a.order[i+1:]...)
			break
		}
	}
}

// Keys returns attribute names in insertion order.
func (a *AttrSet) Keys() []string {
	out := make([]string, len(a.order))
	copy(out, a.order)
	return out
}

// Len returns the number of attributes.
func (a *AttrSet) Len() int {
	return len(a.m)
}

// Clone returns an independent copy of the attribute set: mutating the
// clone (e.g. adding a temporary "NX_class" entry before writing to disk)
// never affects the original.
func (a *AttrSet) Clone() *AttrSet {
	out := NewAttrSet()
	for _, k := range a.Keys() {
		v, _ := a.Get(k)
		out.SetAttr(k, &NXattr{Value: v.Value, Dtype: v.Dtype})
	}
	return out
}
