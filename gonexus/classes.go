package gonexus

// This file provides typed convenience constructors for the NeXus base
// classes used in the vast majority of files, mirroring the ~100
// auto-generated NXgroup subclasses in Python's nexusformat (one per class
// defined by the NeXus standard, e.g. NXentry, NXsample, NXinstrument...).
//
// Rather than generating ~100 near-identical Go types, gonexus represents
// every NeXus base class as a plain *NXgroup with its Class() field set
// accordingly (see NewGroup). The constructors below are thin sugar for the
// classes people reach for constantly; for anything not listed here, just
// call gonexus.NewGroup("NXwhatever") - it works identically.
//
//	sample := gonexus.NewGroup("NXsample")          // same as NewNXsample()
//	beam   := gonexus.NewGroup("NXbeam", "in_beam")  // any class, any name

// NewNXroot creates the root group of a NeXus tree. A tree only becomes a
// valid, saveable NeXus file once it has an NXroot at its top, normally
// containing one or more NXentry children.
func NewNXroot(children ...NXobject) *NXgroup {
	g := NewGroup("NXroot", "root")
	for _, c := range children {
		g.Insert(c.NXName(), c)
	}
	return g
}

// NewNXentry creates an NXentry group, the top-level container for a single
// measurement within an NXroot.
func NewNXentry(children ...NXobject) *NXgroup {
	g := NewGroup("NXentry")
	for _, c := range children {
		g.Insert(c.NXName(), c)
	}
	return g
}

// NewNXsubentry creates an NXsubentry group, used for sub-scans within an
// NXentry.
func NewNXsubentry(children ...NXobject) *NXgroup {
	g := NewGroup("NXsubentry")
	for _, c := range children {
		g.Insert(c.NXName(), c)
	}
	return g
}

// NewNXdata creates an NXdata group and sets the "signal" and "axes"
// attributes per the NeXus plotting convention, mirroring the common
// Python idiom `NXdata(signal, axes)`.
//
//	data := gonexus.NewNXdata(gonexus.NewField(z, "z"),
//	    gonexus.NewField(x, "x"), gonexus.NewField(y, "y"))
func NewNXdata(signal *NXfield, axes ...*NXfield) *NXgroup {
	g := NewGroup("NXdata")
	if signal.NXName() == "" {
		signal.setName("signal")
	}
	g.Insert(signal.NXName(), signal)
	g.NXAttrs().Set("signal", signal.NXName())
	if len(axes) > 0 {
		axisNames := make([]string, len(axes))
		for i, a := range axes {
			if a.NXName() == "" {
				a.setName("axis" + string(rune('0'+i)))
			}
			g.Insert(a.NXName(), a)
			axisNames[i] = a.NXName()
		}
		g.NXAttrs().Set("axes", joinNames(axisNames))
	}
	return g
}

func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ":"
		}
		out += n
	}
	return out
}

// NewNXsample creates an NXsample group describing the measured sample.
func NewNXsample(children ...NXobject) *NXgroup { return newSimpleGroup("NXsample", children) }

// NewNXinstrument creates an NXinstrument group.
func NewNXinstrument(children ...NXobject) *NXgroup {
	return newSimpleGroup("NXinstrument", children)
}

// NewNXdetector creates an NXdetector group.
func NewNXdetector(children ...NXobject) *NXgroup { return newSimpleGroup("NXdetector", children) }

// NewNXmonitor creates an NXmonitor group.
func NewNXmonitor(children ...NXobject) *NXgroup { return newSimpleGroup("NXmonitor", children) }

// NewNXsource creates an NXsource group.
func NewNXsource(children ...NXobject) *NXgroup { return newSimpleGroup("NXsource", children) }

// NewNXuser creates an NXuser group.
func NewNXuser(children ...NXobject) *NXgroup { return newSimpleGroup("NXuser", children) }

// NewNXprocess creates an NXprocess group, describing a data-processing
// step applied to the data.
func NewNXprocess(children ...NXobject) *NXgroup { return newSimpleGroup("NXprocess", children) }

// NewNXnote creates an NXnote group, a free-form annotation.
func NewNXnote(children ...NXobject) *NXgroup { return newSimpleGroup("NXnote", children) }

// NewNXcollection creates an NXcollection group, a generic container for
// data that does not fit another base class.
func NewNXcollection(children ...NXobject) *NXgroup {
	return newSimpleGroup("NXcollection", children)
}

// NewNXparameters creates an NXparameters group, typically used inside
// NXprocess to record processing parameters.
func NewNXparameters(children ...NXobject) *NXgroup {
	return newSimpleGroup("NXparameters", children)
}

// NewNXmonochromator creates an NXmonochromator group.
func NewNXmonochromator(children ...NXobject) *NXgroup {
	return newSimpleGroup("NXmonochromator", children)
}

// NewNXcollimator creates an NXcollimator group.
func NewNXcollimator(children ...NXobject) *NXgroup {
	return newSimpleGroup("NXcollimator", children)
}

// NewNXaperture creates an NXaperture group.
func NewNXaperture(children ...NXobject) *NXgroup { return newSimpleGroup("NXaperture", children) }

// NewNXbeam creates an NXbeam group.
func NewNXbeam(children ...NXobject) *NXgroup { return newSimpleGroup("NXbeam", children) }

// NewNXenvironment creates an NXenvironment group, describing sample
// environment equipment (cryostats, furnaces, etc).
func NewNXenvironment(children ...NXobject) *NXgroup {
	return newSimpleGroup("NXenvironment", children)
}

// NewNXlog creates an NXlog group, used for time-series logged values.
func NewNXlog(children ...NXobject) *NXgroup { return newSimpleGroup("NXlog", children) }

// NewNXgeometry creates an NXgeometry group (legacy shape/position
// description; NXtransformations is preferred in modern files).
func NewNXgeometry(children ...NXobject) *NXgroup { return newSimpleGroup("NXgeometry", children) }

// NewNXtransformations creates an NXtransformations group, describing a
// chain of coordinate transformations.
func NewNXtransformations(children ...NXobject) *NXgroup {
	return newSimpleGroup("NXtransformations", children)
}

func newSimpleGroup(class string, children []NXobject) *NXgroup {
	g := NewGroup(class)
	for _, c := range children {
		g.Insert(c.NXName(), c)
	}
	return g
}
