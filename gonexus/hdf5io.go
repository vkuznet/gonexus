package gonexus

// This file is the sole place in gonexus that calls into
// gonum.org/v1/hdf5 directly. Everything else in the package works with
// NXgroup/NXfield/NXattr. Isolating the cgo-backed calls here means that
// swapping HDF5 bindings (or adding a pure-Go one) only requires rewriting
// this file.
//
// IMPORTANT CAVEAT: gonum.org/v1/hdf5 (as of the version pinned in go.mod)
// does not expose a way to enumerate the attributes attached to a group or
// dataset (no H5Aiterate wrapper) - only OpenAttribute(name) for an
// already-known name. Because of that, readAttrsInto below can only recover
// attributes whose names it already knows to ask for. It asks for the set
// of attribute names defined by the NeXus standard and commonly written by
// nexusformat (NX_class, signal, axes, units, long_name, target, ...); any
// custom attribute with a name outside that list will not be read back.
// See the README "Known Limitations" section for how to extend this list
// or plug in a different binding.

// IMPORTANT CAVEAT #2 - variable-length string datasets: the pinned
// gonum/hdf5 version has a real bug in Dataset.WriteSubset's handling of
// Go strings (confirmed by reading its source at
// github.com/gonum/hdf5/blob/master/h5d_dataset.go): for a variable-length
// string datatype (T_GO_STRING), it writes a pointer directly to the Go
// string's raw byte data, but HDF5's C API requires the write buffer for a
// vlen string element to *contain* a `char*` pointer (one more level of
// indirection) - the mismatch corrupts memory and crashes the process
// (SIGSEGV inside H5Dwrite). Attribute.Write does NOT have this bug (it
// correctly C.CString()s the value and passes its address), so attributes
// are unaffected.
//
// gonexus works around this by never writing string *datasets* (i.e.
// NXfield string values) using T_GO_STRING at all: writeFixedStringDataset
// below uses a fixed-length HDF5 string type instead, which goes through
// the (correctly implemented) generic byte-buffer code path already used
// for numeric arrays. The corresponding reader, readFixedStringDataset,
// only knows how to decode that same fixed-length convention. This means
// gonexus round-trips its own files correctly, but reading a string
// *dataset* (not attribute) from a file written by another NeXus tool
// (Python's nexusformat/h5py default to variable-length strings) may not
// decode correctly - see the doc comment on readFixedStringDataset, and
// the README's "Known Limitations" section.

// IMPORTANT CAVEAT #3 - scalar dataspace detection: Dataspace.IsSimple()
// (which wraps H5Sis_simple) is not a reliable way to detect a scalar
// field/attribute - it returns true for H5S_SCALAR dataspaces too, not
// just H5S_SIMPLE (N-dimensional array) ones. Calling
// Dataspace.SimpleExtentDims() on a genuinely scalar (0-rank) dataspace
// panics in this binding version (it indexes a zero-length slice
// internally). datasetShape() below therefore checks
// Dataspace.SimpleExtentType() first and only calls SimpleExtentDims()
// for a confirmed H5S_SIMPLE dataspace.

import (
	"fmt"
	"reflect"
	"strings"
	"unsafe"

	"gonum.org/v1/hdf5"
)

// wellKnownAttrs is the set of attribute names readAttrsInto probes for.
// Add to this slice (or, better, extend the binding with attribute
// enumeration) if your files carry custom attributes you need read back.
var wellKnownAttrs = []string{
	"NX_class", "signal", "axes", "primary", "units", "long_name",
	"target", "url", "default", "interpretation", "short_name",
	"description", "note", "start_time", "end_time", "definition",
	"version", "file_name", "file_time", "HDF5_Version", "NeXus_version",
	"h5py_version",
}

// container is satisfied by both *hdf5.File and *hdf5.Group (both embed
// hdf5.CommonFG), letting readObject/writeObject work uniformly on the
// file root and on nested groups.
type container interface {
	CreateGroup(name string) (*hdf5.Group, error)
	OpenGroup(name string) (*hdf5.Group, error)
	CreateDataset(name string, dtype *hdf5.Datatype, dspace *hdf5.Dataspace) (*hdf5.Dataset, error)
	OpenDataset(name string) (*hdf5.Dataset, error)
	NumObjects() (uint, error)
	ObjectNameByIndex(idx uint) (string, error)
	ObjectTypeByIndex(idx uint) (hdf5.GType, error)
}

// attributable is satisfied by both *hdf5.Group and *hdf5.Dataset, the only
// two gonum/hdf5 types that expose attribute creation/access.
type attributable interface {
	CreateAttribute(name string, dtype *hdf5.Datatype, dspace *hdf5.Dataspace) (*hdf5.Attribute, error)
	OpenAttribute(name string) (*hdf5.Attribute, error)
}

func groupChildNames(c container) ([]string, error) {
	n, err := c.NumObjects()
	if err != nil {
		return nil, newError("could not enumerate children: %v", err)
	}
	names := make([]string, 0, n)
	for i := uint(0); i < n; i++ {
		name, err := c.ObjectNameByIndex(i)
		if err != nil {
			return nil, newError("could not read child name %d: %v", i, err)
		}
		names = append(names, name)
	}
	return names, nil
}

func childObjectType(c container, name string) (hdf5.GType, error) {
	n, err := c.NumObjects()
	if err != nil {
		return hdf5.H5G_UNKNOWN, err
	}
	for i := uint(0); i < n; i++ {
		nm, err := c.ObjectNameByIndex(i)
		if err != nil {
			return hdf5.H5G_UNKNOWN, err
		}
		if nm == name {
			return c.ObjectTypeByIndex(i)
		}
	}
	return hdf5.H5G_UNKNOWN, newError("child %q not found", name)
}

func openChildGroup(c container, name string) (*hdf5.Group, error) {
	t, err := childObjectType(c, name)
	if err != nil {
		return nil, err
	}
	if t != hdf5.H5G_GROUP {
		return nil, newError("%q is not a group", name)
	}
	return c.OpenGroup(name)
}

func openChildDataset(c container, name string) (*hdf5.Dataset, error) {
	t, err := childObjectType(c, name)
	if err != nil {
		return nil, err
	}
	if t != hdf5.H5G_DATASET {
		return nil, newError("%q is not a dataset", name)
	}
	return c.OpenDataset(name)
}

func createChildGroup(c container, name string) (*hdf5.Group, error) {
	return c.CreateGroup(name)
}

// createSoftLink is a placeholder: gonum/hdf5 does not expose H5Lcreate_soft
// in the version pinned here, so internal NXlinkfield/NXlinkgroup entries
// are currently written as an ordinary copy of their (already-resolved)
// data rather than a true HDF5 soft link. See README "Known Limitations".
func createSoftLink(c container, name, target string) error {
	return newError(
		"writing HDF5 soft links is not supported by the pinned hdf5 "+
			"binding; link %q -> %q was not written", name, target)
}

// ---------------------------------------------------------------------
// Attributes
// ---------------------------------------------------------------------

func (nf *NXFile) readAttrsInto(loc attributable, attrs *AttrSet) error {
	for _, name := range wellKnownAttrs {
		attr, err := loc.OpenAttribute(name)
		if err != nil {
			continue // attribute not present under this name
		}
		value, err := readAttributeValue(attr)
		attr.Close()
		if err != nil {
			return newError("could not read attribute %q: %v", name, err)
		}
		attrs.Set(name, value)
	}
	return nil
}

func (nf *NXFile) writeAttrs(loc attributable, attrs *AttrSet) error {
	for _, name := range attrs.Keys() {
		a, _ := attrs.Get(name)
		if err := writeAttributeValue(loc, name, a.Value); err != nil {
			return newError("could not write attribute %q: %v", name, err)
		}
	}
	return nil
}

func readAttributeValue(attr *hdf5.Attribute) (interface{}, error) {
	// hdf5.Attribute.GetType() returns a bare Identifier rather than a
	// *Datatype with GoType(), so there is no direct way to introspect an
	// attribute's Go type before reading it. Instead, try the common
	// NeXus attribute encodings in order: string (by far the most common
	// case - "units", "NX_class", "long_name", ...), then float64, then
	// int64.
	var s string
	if err := attr.Read(&s, hdf5.T_GO_STRING); err == nil {
		return s, nil
	}
	var f float64
	if err := attr.Read(&f, hdf5.T_NATIVE_DOUBLE); err == nil {
		return f, nil
	}
	var i int64
	if err := attr.Read(&i, hdf5.T_NATIVE_LLONG); err == nil {
		return i, nil
	}
	return nil, newError("unsupported attribute datatype")
}

func writeAttributeValue(loc attributable, name string, value interface{}) error {
	dtype, err := hdf5.NewDatatypeFromValue(value)
	if err != nil {
		return err
	}
	defer dtype.Close()
	space, err := hdf5.CreateDataspace(hdf5.S_SCALAR)
	if err != nil {
		return err
	}
	defer space.Close()
	attr, err := loc.CreateAttribute(name, dtype, space)
	if err != nil {
		return err
	}
	defer attr.Close()
	return attr.Write(writeReady(value), dtype)
}

// ---------------------------------------------------------------------
// Datasets
// ---------------------------------------------------------------------

func datasetShape(ds *hdf5.Dataset) ([]int, error) {
	space := ds.Space()
	defer space.Close()

	// Do NOT rely on IsSimple() to detect a scalar dataspace: it wraps
	// H5Sis_simple, which in modern HDF5 returns true even for
	// H5S_SCALAR dataspaces (not just H5S_SIMPLE ones). Calling
	// SimpleExtentDims() on a genuinely 0-rank (scalar) dataspace panics
	// in this specific gonum/hdf5 version - it indexes into a
	// zero-length dims slice internally. So check the dataspace's actual
	// class first via SimpleExtentType(), and only call
	// SimpleExtentDims() for a true H5S_SIMPLE (N-dimensional array)
	// dataspace.
	switch space.SimpleExtentType() {
	case hdf5.S_SCALAR, hdf5.S_NULL:
		return nil, nil // scalar (or empty) field
	}

	dims, _, err := space.SimpleExtentDims()
	if err != nil {
		return nil, err
	}
	shape := make([]int, len(dims))
	for i, d := range dims {
		shape[i] = int(d)
	}
	return shape, nil
}

func numElements(shape []int) int {
	n := 1
	for _, s := range shape {
		n *= s
	}
	if len(shape) == 0 {
		return 1
	}
	return n
}

// readDatasetValue reads a dataset's raw contents into a flat Go slice (or
// a single scalar, for a zero-length shape), returning the value alongside
// a NeXus/NumPy-style dtype name.
func readDatasetValue(ds *hdf5.Dataset, shape []int, name string) (interface{}, string, error) {
	dt, err := ds.Datatype()
	if err != nil {
		return nil, "", err
	}
	defer dt.Close()

	// Route ANY string-class dataset - fixed-length or variable-length -
	// through readFixedStringDataset, and do this check BEFORE looking at
	// GoType(). Datatype.GoType() maps every T_STRING-class datatype to
	// Go's `string` type regardless of whether it's fixed or
	// variable-length (see typeClassToGoType in
	// github.com/gonum/hdf5/blob/master/h5t_types.go), so relying on
	// GoType() here would send strings through the generic reflect-based
	// path below, which shares the same broken low-level string handling
	// as Dataset.Write (see the file-level comment).
	if dt.Class() == hdf5.T_STRING {
		// temp fix to skip unreadable values
		if int(dt.Size()) == int(unsafe.Sizeof(uintptr(0))) {
			return nil, "", errVariableLengthString
		}
		if int(dt.Size()) == int(unsafe.Sizeof(uintptr(0))) {
			return nil, "", newError(
				"dataset %q appears to be a variable-length HDF5 string; "+
					"not supported for reading by this binding yet", name)
		}
		return readFixedStringDataset(ds, dt, shape)
	}

	goType := elementGoType(dt)
	n := numElements(shape)
	if goType == nil {
		return nil, "", newError("unsupported HDF5 datatype class %v", dt.Class())
	}
	if err := checkElemSize(dt, int(goType.Size()), fmt.Sprintf("dataset of Go type %v", goType)); err != nil {
		return nil, "", err
	}

	slicePtr := reflect.New(reflect.SliceOf(goType))
	slicePtr.Elem().Set(reflect.MakeSlice(reflect.SliceOf(goType), n, n))
	if err := ds.Read(slicePtr.Interface()); err != nil {
		return nil, "", err
	}
	slice := slicePtr.Elem().Interface()
	dtype := elemDtype(goType)

	if len(shape) == 0 {
		// Scalar: unwrap the single-element slice.
		return slicePtr.Elem().Index(0).Interface(), dtype, nil
	}
	return slice, dtype, nil
}

// readFixedStringDataset reads a string dataset by reading its raw bytes
// into a buffer sized (element count) x (HDF5 element byte size), then
// slicing that buffer into fixed-width fields and trimming trailing NUL
// padding from each. This correctly round-trips string fields written by
// gonexus itself (see writeFixedStringDataset), which deliberately uses a
// fixed-length HDF5 string datatype to avoid a Dataset.Write bug in the
// pinned gonum/hdf5 binding affecting variable-length ("vlen") strings -
// see the file-level comment above.
//
// KNOWN LIMITATION: if the dataset actually uses HDF5's variable-length
// string encoding - the default h5py/nexusformat uses for text fields, and
// therefore what most pre-existing, third-party NeXus files on disk will
// contain - this will not decode it correctly, because the raw bytes of a
// vlen element are an internal pointer/descriptor, not the characters
// themselves; you'll get garbage or an error, not a crash (this path only
// reads into a plain []byte buffer, so it can't corrupt memory the way the
// write bug could). Properly supporting vlen dataset reads needs a binding
// capability (checking H5Tis_variable_str, which the binding today calls
// only internally inside Attribute.Read) that is not currently exposed for
// datasets. If you need to read text datasets from files written by other
// tools, see the README's "Known Limitations" section for options.
func readFixedStringDataset(ds *hdf5.Dataset, dt *hdf5.Datatype, shape []int) (interface{}, string, error) {
	elemSize := int(dt.Size())
	if elemSize <= 0 {
		elemSize = 1
	}
	n := numElements(shape)
	buf := make([]byte, n*elemSize)
	if err := ds.Read(addressable(buf)); err != nil {
		return nil, "", fmt.Errorf(
			"reading string dataset (note: variable-length HDF5 strings "+
				"are not supported for dataset reads - see README "+
				"Known Limitations): %w", err)
	}
	values := make([]string, n)
	for i := 0; i < n; i++ {
		raw := buf[i*elemSize : (i+1)*elemSize]
		values[i] = strings.TrimRight(string(raw), "\x00")
	}
	if len(shape) == 0 {
		return values[0], "string", nil
	}
	return values, "string", nil
}

// writeDatasetValue creates a new dataset named `name` under `parent`
// holding `value` (a scalar, or a flat slice paired with `shape`).
func writeDatasetValue(parent container, name string, value interface{}, shape []int) (*hdf5.Dataset, error) {
	if value == nil {
		return nil, newError("field has no value to write")
	}

	// Strings always go through writeFixedStringDataset - see the
	// file-level comment on why Dataset.Write cannot safely be given a
	// variable-length string type.
	switch v := value.(type) {
	case string:
		return writeFixedStringDataset(parent, name, []string{v}, nil)
	case []string:
		return writeFixedStringDataset(parent, name, v, shape)
	}

	var sample interface{}
	rv := reflect.ValueOf(value)
	isSlice := rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array
	if isSlice {
		if rv.Len() == 0 {
			return nil, newError("cannot write an empty array field")
		}
		sample = rv.Index(0).Interface()
	} else {
		sample = value
	}

	dtype, err := hdf5.NewDatatypeFromValue(sample)
	if err != nil {
		return nil, fmt.Errorf("determining HDF5 datatype: %w", err)
	}
	defer dtype.Close()

	var space *hdf5.Dataspace
	if len(shape) == 0 {
		space, err = hdf5.CreateDataspace(hdf5.S_SCALAR)
	} else {
		dims := make([]uint, len(shape))
		for i, s := range shape {
			dims[i] = uint(s)
		}
		space, err = hdf5.CreateSimpleDataspace(dims, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("creating dataspace: %w", err)
	}
	defer space.Close()

	ds, err := parent.CreateDataset(name, dtype, space)
	if err != nil {
		return nil, fmt.Errorf("creating dataset: %w", err)
	}

	if err := ds.Write(addressable(value)); err != nil {
		ds.Close()
		return nil, fmt.Errorf("writing dataset contents: %w", err)
	}
	return ds, nil
}

// writeFixedStringDataset creates a dataset holding one or more strings
// using a fixed-length HDF5 string datatype (H5T_C_S1 copied and resized
// to the longest string present, NUL-padded), instead of the pinned
// gonum/hdf5 binding's variable-length T_GO_STRING type. See the
// file-level comment for why: Dataset.WriteSubset mishandles vlen strings
// in a way that crashes the process, but a fixed-length string is simply N
// raw bytes per element, which goes through the same generic byte-buffer
// code path already proven to work for numeric arrays.
func writeFixedStringDataset(parent container, name string, values []string, shape []int) (*hdf5.Dataset, error) {
	n := len(values)
	if n == 0 {
		return nil, newError("cannot write an empty string array field")
	}
	maxLen := 1 // HDF5 rejects a zero-size string datatype; store >=1 byte.
	for _, s := range values {
		if len(s) > maxLen {
			maxLen = len(s)
		}
	}

	dtype, err := hdf5.T_C_S1.Copy()
	if err != nil {
		return nil, fmt.Errorf("copying string datatype: %w", err)
	}
	defer dtype.Close()
	if err := dtype.SetSize(maxLen); err != nil {
		return nil, fmt.Errorf("setting string datatype size: %w", err)
	}

	buf := make([]byte, n*maxLen) // zero-initialized -> NUL-padded strings
	for i, s := range values {
		copy(buf[i*maxLen:(i+1)*maxLen], s)
	}

	var space *hdf5.Dataspace
	if len(shape) == 0 {
		space, err = hdf5.CreateDataspace(hdf5.S_SCALAR)
	} else {
		dims := make([]uint, len(shape))
		for i, s := range shape {
			dims[i] = uint(s)
		}
		space, err = hdf5.CreateSimpleDataspace(dims, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("creating dataspace: %w", err)
	}
	defer space.Close()

	ds, err := parent.CreateDataset(name, dtype, space)
	if err != nil {
		return nil, fmt.Errorf("creating dataset: %w", err)
	}
	if err := ds.Write(addressable(buf)); err != nil {
		ds.Close()
		return nil, fmt.Errorf("writing string dataset contents: %w", err)
	}
	return ds, nil
}

// addressable returns a pointer to a fresh copy of v with the same
// concrete type. gonum/hdf5's Dataset.Write calls reflect.Value.UnsafeAddr()
// internally, which panics unless the value backing the interface{} is
// addressable - a bare slice or scalar passed directly (e.g.
// `ds.Write(myFloat64Slice)`) is not, but `*myFloat64Slice` is. This wraps
// any value (scalar or slice) accordingly; for slices the copy is shallow,
// so the original backing array's contents are what get written.
func addressable(v interface{}) interface{} {
	rv := reflect.ValueOf(v)
	ptr := reflect.New(rv.Type())
	ptr.Elem().Set(rv)
	return ptr.Interface()
}

// writeReady prepares a value for Attribute.Write, which - unlike
// Dataset.Write - handles Go strings correctly on its own (its
// reflect.String case does its own C-string marshaling and works whether
// it's given a pointer or a bare value). So for attributes, strings can be
// passed through unwrapped; everything else still needs addressable() so
// Attribute.Write's fixed-size fallback case (which does call
// UnsafeAddr()) has something addressable to work with.
func writeReady(v interface{}) interface{} {
	switch v.(type) {
	case string, []string:
		return v
	default:
		return addressable(v)
	}
}

// checkElemSize refuses a read instead of letting HDF5 write more bytes
// per element than the Go buffer was sized for. Without this, a mismatch
// between what HDF5 reports (dt.Size()) and what Go assumed corrupts
// unrelated heap memory across the cgo boundary - Go's memory safety
// doesn't apply to bytes written by C. The corruption doesn't crash
// immediately; it surfaces later, unpredictably, whenever the GC scans
// the clobbered region - which is why it looks like a race condition.
func checkElemSize(dt *hdf5.Datatype, elemGoSize int, context string) error {
	if h5Size := int(dt.Size()); elemGoSize != h5Size {
		return newError(
			"refusing to read %s: HDF5 reports %d byte(s)/element but the "+
				"Go buffer assumes %d; reading would overflow the buffer "+
				"and corrupt heap memory", context, h5Size, elemGoSize)
	}
	return nil
}

// elementGoType picks the Go element type from the HDF5 datatype's own
// class and byte size, which are authoritative, rather than trusting
// dt.GoType() alone - that mapping in the pinned binding can pick a
// narrower Go type than the file's actual on-disk width (observed: an
// 8-byte H5T_FLOAT mapped to Go's 4-byte float32), which would silently
// overflow the read buffer. GoType() is still consulted, but only to
// tell int from uint - never for width.
func elementGoType(dt *hdf5.Datatype) reflect.Type {
	size := int(dt.Size())
	switch dt.Class() {
	case hdf5.T_FLOAT:
		switch size {
		case 4:
			return reflect.TypeOf(float32(0))
		case 8:
			return reflect.TypeOf(float64(0))
		}
	case hdf5.T_INTEGER, hdf5.T_ENUM, hdf5.T_BITFIELD:
		// Enums (h5py bool arrays are the common case: H5T_ENUM over a
		// 1-byte integer base) and bitfields are integer-shaped in
		// storage; dt.GoType() collapses all of them to a generic Go
		// `int` in this binding, which is always platform width (8
		// bytes here) regardless of the real on-disk width - so size
		// must still come from dt.Size(), not from the hint's Go type.
		unsigned := false
		if hint := dt.GoType(); hint != nil {
			switch hint.Kind() {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				unsigned = true
			}
		}
		switch {
		case size == 1 && unsigned:
			return reflect.TypeOf(uint8(0))
		case size == 1:
			return reflect.TypeOf(int8(0))
		case size == 2 && unsigned:
			return reflect.TypeOf(uint16(0))
		case size == 2:
			return reflect.TypeOf(int16(0))
		case size == 4 && unsigned:
			return reflect.TypeOf(uint32(0))
		case size == 4:
			return reflect.TypeOf(int32(0))
		case size == 8 && unsigned:
			return reflect.TypeOf(uint64(0))
		case size == 8:
			return reflect.TypeOf(int64(0))
		}
	}
	return dt.GoType()
}
