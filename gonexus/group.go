package gonexus

import (
	"fmt"
	"strings"
)

// NXgroup represents a container node in a NeXus tree - the Go equivalent
// of an HDF5 group and of Python's NXgroup class. A group has a NeXus base
// class (e.g. "NXentry", "NXdata", "NXsample", or "NXroot" for the top of a
// file) and holds named children, each of which is an NXfield, NXgroup, or
// link.
type NXgroup struct {
	base
	class   string
	order   []string
	entries map[string]NXobject

	// file is set only on the root group of a tree that was loaded from,
	// or has been saved to, an HDF5 file via NXFile.
	file *NXFile
}

// NewGroup creates an empty NXgroup of the given NeXus class ("NXentry",
// "NXsample", "NXdata", ...) and optional name. If name is omitted, it
// defaults to the class name with the "NX" prefix stripped (e.g. "NXsample"
// -> "sample"), matching Python's default naming convention.
func NewGroup(class string, name ...string) *NXgroup {
	n := ""
	if len(name) > 0 {
		n = name[0]
	} else {
		n = defaultGroupName(class)
	}
	return &NXgroup{
		base:    newBase(n),
		class:   class,
		entries: make(map[string]NXobject),
	}
}

func defaultGroupName(class string) string {
	if strings.HasPrefix(class, "NX") && len(class) > 2 {
		return strings.ToLower(class[2:])
	}
	return strings.ToLower(class)
}

// NXClass returns the group's NeXus base class, e.g. "NXentry".
func (g *NXgroup) NXClass() string { return g.class }

// NXPath returns the group's absolute path in its tree.
func (g *NXgroup) NXPath() string { return nxPath(g) }

// Insert adds or replaces a direct child of the group under the given name,
// and sets the child's parent/name accordingly. This is the Go equivalent
// of Python's `group['name'] = obj`.
func (g *NXgroup) Insert(name string, obj NXobject) *NXgroup {
	if g.entries == nil {
		g.entries = make(map[string]NXobject)
	}
	if _, exists := g.entries[name]; !exists {
		g.order = append(g.order, name)
	}
	obj.setName(name)
	obj.setGroup(g)
	g.entries[name] = obj
	return g
}

// InsertField is a convenience wrapper that builds an NXfield from a raw Go
// value and inserts it, mirroring Python's `group['name'] = 3.14`.
func (g *NXgroup) InsertField(name string, value interface{}) *NXfield {
	f := NewField(value, name)
	g.Insert(name, f)
	return f
}

// Delete removes a direct child from the group by name.
// Equivalent to Python's `del group['name']`.
func (g *NXgroup) Delete(name string) {
	if _, ok := g.entries[name]; !ok {
		return
	}
	delete(g.entries, name)
	for i, k := range g.order {
		if k == name {
			g.order = append(g.order[:i], g.order[i+1:]...)
			break
		}
	}
}

// Child returns the direct child with the given name, or nil.
func (g *NXgroup) Child(name string) NXobject {
	if g.entries == nil {
		return nil
	}
	return g.entries[name]
}

// Get resolves a slash-separated path relative to this group (e.g.
// "instrument/detector/distance"), traversing nested groups and following
// links, and returns the object found there. This is the Go equivalent of
// Python's `group['path/to/item']`.
func (g *NXgroup) Get(path string) (NXobject, error) {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return g, nil
	}
	parts := strings.Split(path, "/")
	var cur NXobject = g
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		cg, ok := resolveGroup(cur)
		if !ok {
			return nil, newError("'%s' is not a group and cannot contain '%s'", cur.NXName(), part)
		}
		child, ok := cg.entries[part]
		if !ok {
			return nil, newError("path component '%s' not found in '%s'", part, cg.NXPath())
		}
		cur = resolveLink(child)
	}
	return cur, nil
}

// GetGroup is like Get but additionally asserts the result is a group.
func (g *NXgroup) GetGroup(path string) (*NXgroup, error) {
	obj, err := g.Get(path)
	if err != nil {
		return nil, err
	}
	sub, ok := resolveGroup(obj)
	if !ok {
		return nil, newError("'%s' is not a group", path)
	}
	return sub, nil
}

// GetField is like Get but additionally asserts the result is a field.
func (g *NXgroup) GetField(path string) (*NXfield, error) {
	obj, err := g.Get(path)
	if err != nil {
		return nil, err
	}
	switch f := obj.(type) {
	case *NXfield:
		return f, nil
	case *NXlinkfield:
		return f.NXfield, nil
	default:
		return nil, newError("'%s' is not a field", path)
	}
}

// Entries returns the group's direct children in insertion order, as an
// ordered slice of (name, object) pairs.
func (g *NXgroup) Entries() []Entry {
	out := make([]Entry, 0, len(g.order))
	for _, k := range g.order {
		out = append(out, Entry{Name: k, Object: g.entries[k]})
	}
	return out
}

// Entry pairs a child's name with the child itself; returned by Entries().
type Entry struct {
	Name   string
	Object NXobject
}

// Keys returns the group's direct child names in insertion order.
func (g *NXgroup) Keys() []string {
	out := make([]string, len(g.order))
	copy(out, g.order)
	return out
}

// Component returns every descendant group whose NXClass equals class,
// searched recursively, in the order encountered. This is the Go
// equivalent of Python's `group.component('NXclass')` / `group.NXclass`
// property.
func (g *NXgroup) Component(class string) []*NXgroup {
	var out []*NXgroup
	for _, k := range g.order {
		child := g.entries[k]
		resolved := resolveLink(child)
		if sub, ok := resolved.(*NXgroup); ok {
			if sub.class == class {
				out = append(out, sub)
			}
			out = append(out, sub.Component(class)...)
		} else if lg, ok := resolved.(*NXlinkgroup); ok {
			if lg.class == class {
				out = append(out, lg.NXgroup)
			}
			out = append(out, lg.NXgroup.Component(class)...)
		}
	}
	return out
}

// Signal returns the group's designated NXdata signal field, following the
// "signal" attribute convention, or nil if not applicable/found.
// Equivalent to Python's NXdata.nxsignal property.
func (g *NXgroup) Signal() *NXfield {
	name := g.attrs.GetString("signal")
	if name == "" {
		return nil
	}
	f, err := g.GetField(name)
	if err != nil {
		return nil
	}
	return f
}

// Save writes this group's tree to filename as a new HDF5/NeXus file.
// Only meaningful when called on an NXroot group (as returned by
// NewNXroot or Load); equivalent to Python's `root.save(filename)`.
func (g *NXgroup) Save(filename string, mode ...string) error {
	return Save(filename, g, mode...)
}

// Dir returns a one-line-per-child summary of the group's immediate
// contents, matching Python's `group.dir()` (non-recursive).
func (g *NXgroup) Dir() string {
	lines := []string{fmt.Sprintf("%s:%s", g.name, g.class)}
	for _, k := range g.order {
		child := g.entries[k]
		switch c := child.(type) {
		case *NXfield:
			lines = append(lines, "  "+c.treeLines("", 0)[0])
		default:
			lines = append(lines, fmt.Sprintf("  %s:%s", child.NXName(), child.NXClass()))
		}
	}
	return strings.Join(lines, "\n")
}

// Tree returns a full recursive rendering of the group and all its
// descendants, formatted the same way as Python's `group.tree` property,
// e.g.:
//
//	root:NXroot
//	  entry:NXentry
//	    data:NXdata
//	      data = float64(10x10)
//	        @units = counts
func (g *NXgroup) Tree() string {
	return strings.Join(g.treeLines("", 0), "\n")
}

func (g *NXgroup) treeLines(prefix string, depth int) []string {
	indent := indentString(depth)
	lines := []string{fmt.Sprintf("%s%s:%s", indent, g.name, g.class)}
	attrIndent := indentString(depth + 1)
	for _, k := range g.attrs.Keys() {
		a, _ := g.attrs.Get(k)
		lines = append(lines, fmt.Sprintf("%s@%s = %v", attrIndent, k, a.Value))
	}
	for _, k := range g.order {
		lines = append(lines, g.entries[k].treeLines(prefix, depth+1)...)
	}
	return lines
}

// resolveGroup returns obj as an *NXgroup if it is one (directly or via an
// NXlinkgroup), following at most one level of link indirection.
func resolveGroup(obj NXobject) (*NXgroup, bool) {
	switch o := obj.(type) {
	case *NXgroup:
		return o, true
	case *NXlinkgroup:
		return o.NXgroup, true
	default:
		return nil, false
	}
}
