// Package gonexus is a Go port of the core data model and file I/O of the
// Python nexusformat.nexus package (https://github.com/nexpy/nexusformat).
//
// It maps the hierarchical NeXus data format (a set of conventions built on
// top of HDF5) onto Go values: NXgroup for HDF5 groups, NXfield for HDF5
// datasets, NXattr for HDF5 attributes, and NXlink for internal/external
// HDF5 links. See the package README for a full usage guide.
package gonexus

import "fmt"

// NeXusError is the error type returned by all gonexus operations that fail
// for NeXus/HDF5-specific reasons (bad paths, invalid classes, I/O
// failures, etc). It mirrors Python's `NeXusError` exception class.
type NeXusError struct {
	Msg string
}

func (e *NeXusError) Error() string {
	return e.Msg
}

// newError builds a *NeXusError with a formatted message, in the same spirit
// as `raise NeXusError(f"...")` in the Python source.
func newError(format string, args ...interface{}) *NeXusError {
	return &NeXusError{Msg: fmt.Sprintf(format, args...)}
}

// errors.go
var errVariableLengthString = newError("variable-length HDF5 string not supported for reading")

func isUnsupportedVlenString(err error) bool {
	ne, ok := err.(*NeXusError)
	return ok && ne.Msg == errVariableLengthString.Msg
}
