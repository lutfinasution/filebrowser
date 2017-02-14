// fb_dirtree.go project fb_dirtree.go.go
package main

import (
	"log"
	"os"
	"path/filepath"
)

import (
	"github.com/lxn/walk"
)

type Directory struct {
	name     string
	parent   *Directory
	children []*Directory
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

func (d *Directory) ChildCount() int {
	if d.children == nil {
		// It seems this is the first time our child count is checked, so we
		// use the opportunity to populate our direct children.
		if err := d.ResetChildren(); err != nil {
			log.Print(err)
		}
	}

	return len(d.children)
}

func (d *Directory) ChildAt(index int) walk.TreeItem {
	return d.children[index]
}

func (d *Directory) Image() interface{} {
	return d.Path()
}

func (d *Directory) ResetChildren() error {
	d.children = nil

	dirPath := d.Path()

	if err := filepath.Walk(d.Path(), func(path string, info os.FileInfo, err error) error {
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

func OnTreeCurrentItemChanged() {
	dir := treeView.CurrentItem().(*Directory)
	if err := tableModel.SetDirPath(dir.Path()); err != nil {
		walk.MsgBox(
			Mw.MainWindow,
			"Error",
			err.Error(),
			walk.MsgBoxOK|walk.MsgBoxIconError)
	}
}

func locateDir(itms []*Directory, sdir string) *Directory {
	var res *Directory
	res = nil

	for _, itm2 := range itms {
		if itm2.name == sdir {
			res = itm2
			break
		}
	}
	return res
}

func LocatePath(fpath string) bool {
	var sfile string
	var paths, paths1 []string

	//split path into parts
	sdrive := filepath.VolumeName(fpath) + "\\"
	for {
		fpath, sfile = filepath.Split(fpath)
		fpath, _ = filepath.Abs(fpath)

		if sfile != "" {
			paths = append(paths, sfile)
		} else {
			break
		}
	}
	paths = append(paths, sdrive)

	//flip to a sorted array
	for i := len(paths) - 1; i >= 0; i-- {
		paths1 = append(paths1, paths[i])
	}

	tm := treeModel
	var itmnext *Directory

	for i := 0; i < len(paths1); i++ {
		if i == 0 {
			itmnext = locateDir(tm.roots, paths1[i])
		} else {
			if itmnext != nil {
				itmnext = locateDir(itmnext.children, paths1[i])
			}
		}
		if itmnext != nil {
			treeView.SetCurrentItem(itmnext)
			treeView.SetExpanded(itmnext, true)
		}
	}
	return true
}
