// fb_dirtree.go project fb_dirtree.go.go
package main

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

import (
	"github.com/lxn/walk"
)

type Directory struct {
	name      string
	parent    *Directory
	children  DirSlice //[]*Directory
	donereset bool
}
type DirSlice []*Directory

func (d DirSlice) Len() int {
	return len(d)
}
func (d DirSlice) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}
func (d DirSlice) Less(i, j int) bool {
	strings.ToLower(d[i].name)

	//return d[i].name < d[j].name
	return strings.ToLower(d[i].name) < strings.ToLower(d[j].name)
}

type DirectoryTreeModel struct {
	walk.TreeModelBase
	roots []*Directory
}

func (d *Directory) Text() string {
	return d.name
}

func (d *Directory) Parent() walk.TreeItem {
	if d.parent == nil {
		// We can't simply return d.parent in this case, because the interface
		// value then would not be nil.
		return nil
	}

	return d.parent
}

func (d *Directory) ChildAt(index int) walk.TreeItem {
	return d.children[index]
}
func (d *Directory) ChildCount() int {

	if !d.donereset && d.children == nil {
		// It seems this is the first time our child count is checked, so we
		// use the opportunity to populate our direct children.
		if err := d.ResetChildren(); err != nil {
			log.Print(err)
		}
	}

	return len(d.children)
}

func (d *Directory) Image() interface{} {
	return d.Path()
}

func (d *Directory) ResetChildren() error {
	d.children = nil
	dirPath := d.Path()
	d.donereset = true

	if err := filepath.Walk(d.Path(),
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if info == nil {
					return filepath.SkipDir
				}
			}

			name := info.Name()

			if !info.IsDir() || path == dirPath || shouldExclude(name) {
				return nil
			}

			d.children = append(d.children, NewDirectory(name, d))

			return filepath.SkipDir
		}); err != nil {
		return err
	}

	dc := d.children
	sort.Sort(dc)
	d.children = dc

	return nil
}

func (d *Directory) Path() string {
	elems := []string{d.name}

	dir, _ := d.Parent().(*Directory)

	for dir != nil {
		elems = append([]string{dir.name}, elems...)
		dir, _ = dir.Parent().(*Directory)
	}

	return filepath.Join(elems...)
}

func NewDirectory(name string, parent *Directory) *Directory {
	return &Directory{name: name, parent: parent}
}

func NewDirectoryTreeModel() (*DirectoryTreeModel, error) {
	model := new(DirectoryTreeModel)

	drives, err := walk.DriveNames()
	if err != nil {
		return nil, err
	}

	for _, drive := range drives {
		switch drive {
		case "A:\\", "B:\\":
			continue
		}

		model.roots = append(model.roots, NewDirectory(drive, nil))
	}

	return model, nil
}

func (*DirectoryTreeModel) LazyPopulation() bool {
	// We don't want to eagerly populate our tree view with the whole file system.
	return true
}

func (m *DirectoryTreeModel) RootCount() int {
	return len(m.roots)
}

func (m *DirectoryTreeModel) RootAt(index int) walk.TreeItem {
	return m.roots[index]
}

// expands the treeview n locate the treenode
// corresponding to fpath. Once found, make it
// the currently selected item.
func LocatePath(fpath string) bool {

	locateSubDir := func(itms []*Directory, sdir string) (res *Directory) {
		res = nil

		for _, itm := range itms {
			if strings.Contains(sdir, itm.Path()) {
				res = itm
				break
			}
		}
		return res
	}

	tm := treeModel
loop:
	for _, v := range tm.roots {
		if strings.Contains(fpath, v.Path()) {
			treeView.SetExpanded(v, true)
			itmsnext := v.children
			for {
				itmDir := locateSubDir(itmsnext, fpath)
				if itmDir != nil {
					treeView.SetExpanded(itmDir, true)
					itmsnext = itmDir.children

					if itmDir.Path() == fpath {
						treeView.SetCurrentItem(itmDir)
						return true
					}

				} else {
					break loop
				}
			}
			break
		}
	}

	return false
}
