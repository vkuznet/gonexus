package gonexus

// NXlink holds the raw target information for an internal or external HDF5
// link, shared by NXlinkfield and NXlinkgroup. It mirrors the bookkeeping
// Python's NXlink/NXlinkfield/NXlinkgroup classes keep on `_target`,
// `_filename`, `_abspath`, and `_soft`.
type NXlink struct {
	// Target is the absolute path of the linked object, either within
	// the same file (internal/soft link) or within File (external link).
	Target string
	// File is the external file name, or "" for an internal link.
	File string
	// AbsPath is true if File is an absolute path.
	AbsPath bool
	// Soft is true for an internal HDF5 soft link (as opposed to a
	// resolved hard link, which is not represented as a link at all).
	Soft bool
}

// IsExternal reports whether the link points into a different file.
func (l *NXlink) IsExternal() bool { return l.File != "" }

// NXlinkfield is an NXfield that is actually a link (internal soft link or
// external link) to a dataset elsewhere. It embeds *NXfield so it can be
// used anywhere an *NXfield is expected once resolved; NXlink holds the
// unresolved target metadata. Equivalent to Python's NXlinkfield.
type NXlinkfield struct {
	*NXfield
	Link *NXlink
}

// NewLinkField creates an unresolved link to a field, for use when building
// a tree programmatically before saving, e.g. to link a field in one NXdata
// group to a detector's data elsewhere in the file.
func NewLinkField(name, target string) *NXlinkfield {
	return &NXlinkfield{
		NXfield: NewField(nil, name),
		Link:    &NXlink{Target: target},
	}
}

func (f *NXlinkfield) NXClass() string { return "NXlink" }

// NXlinkgroup is an NXgroup that is actually a link to a group elsewhere.
// Equivalent to Python's NXlinkgroup.
type NXlinkgroup struct {
	*NXgroup
	Link *NXlink
}

// NewLinkGroup creates an unresolved link to a group.
func NewLinkGroup(name, target string) *NXlinkgroup {
	return &NXlinkgroup{
		NXgroup: NewGroup("NXlink", name),
		Link:    &NXlink{Target: target},
	}
}

// resolveLink is a hook point for link-following. NXgroup.Get() calls this
// on every path component; today it is the identity function because
// NXlinkfield/NXlinkgroup already embed the underlying *NXfield/*NXgroup,
// but it is kept as a distinct step so a future version can transparently
// redirect to the true target (including across external files) without
// changing call sites.
func resolveLink(obj NXobject) NXobject {
	return obj
}
