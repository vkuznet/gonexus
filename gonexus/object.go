package gonexus

import "strings"

// NXobject is implemented by every node in a NeXus tree: *NXfield,
// *NXgroup, and the link variants *NXlinkfield / *NXlinkgroup. It mirrors
// the attributes shared by Python's NXobject base class: nxname, nxclass,
// nxgroup (parent), and attrs.
type NXobject interface {
	// NXName returns the object's name within its parent group.
	NXName() string
	// NXClass returns "NXfield" for fields, the NeXus base class
	// ("NXentry", "NXdata", ...) for groups, or "NXroot" for the root.
	NXClass() string
	// NXGroup returns the parent group, or nil if this is the root or
	// otherwise unattached.
	NXGroup() *NXgroup
	// NXAttrs returns the object's attribute set.
	NXAttrs() *AttrSet
	// NXPath returns the absolute slash-separated path of this object
	// within its tree, e.g. "/entry/data/signal".
	NXPath() string

	setName(string)
	setGroup(*NXgroup)
	treeLines(prefix string, depth int) []string
}

// base holds the fields shared by NXfield and NXgroup implementations.
type base struct {
	name  string
	group *NXgroup
	attrs *AttrSet
}

func newBase(name string) base {
	return base{name: name, attrs: NewAttrSet()}
}

func (b *base) NXName() string        { return b.name }
func (b *base) NXGroup() *NXgroup     { return b.group }
func (b *base) NXAttrs() *AttrSet     { return b.attrs }
func (b *base) setName(name string)   { b.name = name }
func (b *base) setGroup(g *NXgroup)   { b.group = g }

// NXPath walks up the parent chain accumulating names, matching Python's
// NXobject.nxpath property.
func nxPath(o NXobject) string {
	var parts []string
	cur := o
	for cur != nil {
		name := cur.NXName()
		g := cur.NXGroup()
		if g == nil {
			// Root: only prepend its name if it is not the implicit "root".
			if name != "" && name != "root" {
				parts = append([]string{name}, parts...)
			}
			break
		}
		parts = append([]string{name}, parts...)
		cur = g
	}
	return "/" + strings.Join(parts, "/")
}
