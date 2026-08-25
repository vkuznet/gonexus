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

import (
	"fmt"
	"reflect"

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
	return attr.Write(value, dtype)
}

// ---------------------------------------------------------------------
// Datasets
// ---------------------------------------------------------------------

func datasetShape(ds *hdf5.Dataset) ([]int, error) {
	space := ds.Space()
	defer space.Close()
	if !space.IsSimple() {
		return nil, nil // scalar
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
func readDatasetValue(ds *hdf5.Dataset, shape []int) (interface{}, string, error) {
	dt, err := ds.Datatype()
	if err != nil {
		return nil, "", err
	}
	defer dt.Close()
	goType := dt.GoType()
	n := numElements(shape)

	if goType == nil {
		// Fall back to string, the common case for NeXus text fields.
		return readStringDataset(ds, shape)
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

func readStringDataset(ds *hdf5.Dataset, shape []int) (interface{}, string, error) {
	n := numElements(shape)
	buf := make([]string, n)
	if err := ds.Read(&buf); err != nil {
		return nil, "", err
	}
	if len(shape) == 0 {
		return buf[0], "string", nil
	}
	return buf, "string", nil
}

// writeDatasetValue creates a new dataset named `name` under `parent`
// holding `value` (a scalar, or a flat slice paired with `shape`).
func writeDatasetValue(parent container, name string, value interface{}, shape []int) (*hdf5.Dataset, error) {
	if value == nil {
		return nil, newError("field has no value to write")
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

	if err := ds.Write(value); err != nil {
		ds.Close()
		return nil, fmt.Errorf("writing dataset contents: %w", err)
	}
	return ds, nil
}
