// Copyright 2011 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"image"
	//"image/draw"
	"log"
	"os"
	"path/filepath"
	//"reflect"
	"fmt"
	"strconv"
	"time"
)

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

//var _ walk.TreeItem = new(Directory)
//var _ walk.TreeModel = new(DirectoryTreeModel)
//var _ walk.ReflectTableModel = new(FileInfoModel)
var Mw = new(MyMainWindow)
var treeView *walk.TreeView
var treeModel *DirectoryTreeModel
var tableView *walk.TableView

//var tableModel *FileInfoModel
var addrList []string
var settings *walk.IniFileSettings

type tviews struct {
	id      win.HWND
	viewer  *ScrollViewer
	handler *walk.Action
}
type albumInfo struct {
	id   int
	name string
	desc string
}

type MyMainWindow struct {
	*walk.MainWindow
	toolbar          *walk.CustomWidget
	hSplitter        *walk.Splitter
	viewBase         *walk.Composite
	thumbView        *ScrollViewer
	albumView        *ScrollViewer
	compAlbum        *walk.Composite
	compFolder       *walk.Composite
	thumbViews       []tviews
	btn1             *walk.PushButton
	paintWidgetMenu  *walk.Menu
	topComposite     *walk.Composite
	lblAddr          *walk.Label
	cmbAddr          *walk.ComboBox
	btnOptions       *walk.PushButton
	menuItemAction   *walk.Menu
	treeMenu         *walk.Menu
	albumMenu        *walk.Menu
	menuView         *walk.Menu
	ViewSlider       *walk.Slider
	prevFilePath     string
	CurrentPath      string
	menuKeepLoc      *walk.Action
	menuTest1        *walk.Action
	menuTest2        *walk.Action
	menuTest3        *walk.Action
	menuView0        *walk.Action
	menuView1        *walk.Action
	menuView2        *walk.Action
	menuView3        *walk.Action
	menuView4        *walk.Action
	actionAlbumItem1 *walk.Action
	actionAlbumItem2 *walk.Action
	actionAlbumItem3 *walk.Action
	actionAlbumSort1 *walk.Action
	actionAlbumSort2 *walk.Action
	actionAlbumSort3 *walk.Action
	albuminfo        *albumInfo
	visibleAlbum     bool
	visibleFolder    bool
}

var testrun1 = false
var stoptest = false

func (mw *MyMainWindow) onTest1() {
	//perform automated scrolling ops
	//testing the rendering speed and
	//efficiency
	if !testrun1 {
		go func() {
			testrun1 = true
			t := time.Now()
			sc := mw.thumbView
			h := mw.thumbView.itemHeight
			c := 0
			for i := 0; i < sc.MaxScrollValue(); i += h / 4 {
				sc.SetScroll(i)

				c++
				if stoptest {
					stoptest = false
					break
				}
				if c%2*h == 0 {
					time.Sleep(time.Millisecond * 1)
				}
			}
			testrun1 = false
			d := time.Since(t).Seconds()
			fps := float64(c) / d
			mw.StatusBar().Items().At(1).SetText(fmt.Sprintf("scrolltest done in %6.3f sec. at %6.1f fps", d, fps))
		}()
	} else {
		stoptest = true
	}
}

func (mw *MyMainWindow) onTest2() {
	//create simple test images

	dlg := new(walk.FileDialog)

	if mw.menuKeepLoc.Checked() {
		dlg.InitialDirPath = mw.prevFilePath
	}
	dlg.Title = "Select a Location to create test image files to"

	if ok, err := dlg.ShowBrowseFolder(Mw.MainWindow); err != nil {
		return
	} else if !ok {
		return
	}
	if num, ok := DrawTestImage(dlg.FilePath); ok {
		walk.MsgBox(mw, "Create test image files", "Created "+strconv.Itoa(num)+" test image files",
			walk.MsgBoxOK|walk.MsgBoxIconInformation)
	}

	mw.prevFilePath = dlg.FilePath
}
func (mw *MyMainWindow) onTest3() {
	//dump in-memory thumbnail data as jpeg files

	dlg := new(walk.FileDialog)

	if mw.menuKeepLoc.Checked() {
		dlg.InitialDirPath = mw.prevFilePath
	}
	dlg.Title = "Select a Location to dump cache image files to"

	if ok, err := dlg.ShowBrowseFolder(Mw.MainWindow); err != nil {
		return
	} else if !ok {
		return
	}
	for _, v := range mw.thumbView.itemsModel.items {
		if f, err := os.Create(filepath.Join(dlg.FilePath, "test-"+v.Name+".jpg")); err == nil {
			f.Write(v.Imagedata)
			f.Close()
		} else {
			return
		}
	}
	walk.MsgBox(mw, "Dump cache image files", "Dump cache image files successful",
		walk.MsgBoxOK|walk.MsgBoxIconInformation)

	mw.prevFilePath = dlg.FilePath
}
func (mw *MyMainWindow) onMenuActionExplore() {
	if treeItemPath != "" {
		NewThumbViewWindow(mw.MainWindow, treeItemPath)
	}
}
func (mw *MyMainWindow) onMenuActionReload() {

	mw.thumbView.Run(mw.thumbView.LastURL, nil, true)
}

func (mw *MyMainWindow) folderShow(bShow bool) {
	if bShow {
		hdr1.SetBackground(brs)
		Mw.compFolder.SetVisible(true)
		if Mw.compAlbum.Visible() {
			minSz := Mw.compAlbum.MinSize()
			minSz.Height -= Mw.compFolder.MinSize().Height
			Mw.compAlbum.SetMinMaxSize(minSz, walk.Size{0, 0})
		}
	} else {
		hdr1.SetBackground(cmp00.Background())
		Mw.compFolder.SetVisible(false)
	}

	Mw.visibleFolder = bShow

	cmp00.SendMessage(win.WM_SIZE, 0, 0)
	cmp00.SizeChanged()
}
func (mw *MyMainWindow) albumEditorShow(bShow bool) {
	if bShow {
		hdr3.SetBackground(brs)
		cmp03.SetVisible(true)
		if Mw.compAlbum.Visible() {
			minSz := Mw.compAlbum.MinSize()
			minSz.Height -= cmp03.MinSize().Height
			Mw.compAlbum.SetMinMaxSize(minSz, walk.Size{0, 0})
		}
	} else {
		hdr3.SetBackground(cmp00.Background())
		cmp03.SetVisible(false)
	}
	cmp00.SendMessage(win.WM_SIZE, 0, 0)
	cmp00.SizeChanged()
}
func (mw *MyMainWindow) albumSortName() {
	mw.albumView.AlbumSortbyName(true)
	mw.actionAlbumSort1.SetChecked(true)
	mw.actionAlbumSort2.SetChecked(false)
	mw.actionAlbumSort3.SetChecked(false)
}
func (mw *MyMainWindow) albumSortDate() {
	mw.albumView.AlbumSortbyDate(true)
	mw.actionAlbumSort2.SetChecked(true)
	mw.actionAlbumSort1.SetChecked(false)
	mw.actionAlbumSort3.SetChecked(false)
}
func (mw *MyMainWindow) albumSortSize() {
	mw.albumView.AlbumSortbySize(true)
	mw.actionAlbumSort3.SetChecked(true)
	mw.actionAlbumSort2.SetChecked(false)
	mw.actionAlbumSort1.SetChecked(false)
}
func (mw *MyMainWindow) albumShow(bShow bool) {
	if bShow {
		if !mw.compAlbum.Visible() {
			hdr2.SetBackground(brs)
			mw.compAlbum.SetVisible(true)
			mw.compAlbum.SetMinMaxSize(walk.Size{0, 180}, walk.Size{0, 0})
		}
		if mw.compAlbum.Children().Len() == 0 {
			mw.albumView, _ = NewScrollViewer(mw.MainWindow, mw.compAlbum, false, 0, 100, 63)

			//tvw.SetEventMouseDown(Mw.onThumbViewMouseDn)
			mw.albumView.SetItemSize(100, 63)
			mw.albumView.OnAlbumEditing = mw.albumStartEdit
			mw.albumView.OnSelectionChanged = mw.albumSelChange
			mw.albumView.SetContextMenu(mw.albumMenu)

			cmp00.Layout().Update(false)
		}
		mw.albumView.RunAlbum()
		mw.StatusBar().Items().At(1).SetText(fmt.Sprintf(" %d albums", mw.albumView.itemsCount))
	} else {
		hdr2.SetBackground(cmp00.Background())
		mw.compAlbum.SetVisible(false)
		mw.compAlbum.SetHeight(1)
	}
	mw.visibleAlbum = bShow

	cmp00.SendMessage(win.WM_SIZE, 0, 0)
	cmp00.SizeChanged()
}
func (mw *MyMainWindow) albumSelChange() {
	// send mw.thumbView as target thumbview to
	// render the album items.

	mw.albumView.AlbumEnumItems(mw.thumbView)
	mw.actionAlbumItem1.SetVisible(false)
	mw.actionAlbumItem2.SetVisible(true)
	mw.actionAlbumItem3.SetVisible(true)
}

func (mw *MyMainWindow) albumEdit() {

	mw.albumView.AlbumEdit()
}
func (mw *MyMainWindow) albumStartEdit(id int, name string, desc string) {
	// called from the album view callback OnAlbumEditing
	// could be triggered by calling AlbumEdit() or internal dblclick
	mw.albumEditorShow(true)

	mw.albumView.SetEnabled(false)
	mw.albuminfo = &albumInfo{id: id, name: name, desc: desc}

	albumData1.SetText(name)
	albumData2.SetText(desc)
}
func (mw *MyMainWindow) albumCancel() {
	mw.albuminfo = nil
	mw.albumView.SetEnabled(true)
}
func (mw *MyMainWindow) albumSaveEdit() bool {

	if mw.albumView != nil {
		info := FileInfo{index: -1, Name: albumData1.Text(), URL: albumData2.Text()}
		if mw.albuminfo != nil {
			info.index = mw.albuminfo.id
		}

		res, _ := mw.albumView.AlbumDBUpdateAlbum(&info)
		if res > 0 {
			albumData1.SetText("")
			albumData2.SetText("")

			mw.albumView.RunAlbum()

			if mw.albuminfo != nil {
				mw.albuminfo = nil
			}
			mw.albumView.SetEnabled(true)
			return true
		}
	}
	return false
}
func (mw *MyMainWindow) AlbumAddItems() {
	//Display album frame

	mw.albumShow(true)

	if mw.albumView.SelectedIndex != -1 {
		mw.albumView.AlbumAddItems(mw.thumbView)
		mw.albumView.RunAlbum()
	} else {
		walk.MsgBox(mw, "Add to Album", "Please select an album first",
			walk.MsgBoxOK|walk.MsgBoxIconInformation)
	}
}

// Sets the album cover image for the currently selected
// album.
func (mw *MyMainWindow) AlbumSetCover() {
	if mw.albumView != nil {
		//use source thumbview's selected item's data
		val := mw.thumbView.SelectedItem()
		org := mw.albumView.SelectedItem()

		info := FileInfo{index: org.index, Name: org.Name, URL: org.URL, Imagedata: val.Imagedata}

		res, _ := mw.albumView.AlbumDBUpdateAlbum(&info)
		if res > 0 {
			mw.albumView.RunAlbum()
		}
	}
}

// Delete items from album
func (mw *MyMainWindow) AlbumDeleteItems() {

	mw.albumShow(true)

	if mw.albumView.SelectedIndex != -1 {

		if win.IDYES == walk.MsgBox(mw, "Remove from Album", "Remove items from album?",
			walk.MsgBoxYesNo|walk.MsgBoxIconQuestion|walk.MsgBoxDefButton2) {

			mw.albumView.AlbumDelItems(mw.thumbView)
			mw.albumView.RunAlbum()
		}
	} else {
		walk.MsgBox(mw, "Remove from Album", "Please select an album first",
			walk.MsgBoxOK|walk.MsgBoxIconInformation)
	}

}

func (mw *MyMainWindow) onMenuActionPreview() {
	//Display full image preview
	mw.thumbView.ShowPreviewFull()
}
func (mw *MyMainWindow) onMenuActionPreview2() {
	//do not continue if there is
	//already a preview on screen
	if mw.thumbView.PreviewRect != nil {
		return
	}
	//Display image preview
	mw.thumbView.ShowPreview()
}
func (mw *MyMainWindow) onMenuActionDelete() {
	fdelete := mw.thumbView.SelectedName()

	if fdelete != "" {
		if walk.MsgBox(mw, "Delete File", "Delete file "+fdelete,
			walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == win.IDYES {
			if err := os.Remove(mw.thumbView.SelectedNameFull()); err == nil {
				mw.thumbView.SelectedIndex = -1
			}
		}
	}
}

func (mw *MyMainWindow) onMenuActionRename() {

}

func (mw *MyMainWindow) onMenuActionCopyTo() {

	dlg := new(walk.FileDialog)

	if mw.menuKeepLoc.Checked() {
		dlg.InitialDirPath = mw.prevFilePath
	}
	dlg.Title = "Select a Location to copy files to"

	if ok, err := dlg.ShowBrowseFolder(Mw.MainWindow); err != nil {
		return
	} else if !ok {
		return
	}
	mw.prevFilePath = dlg.FilePath
}
func (mw *MyMainWindow) onMenuActionMoveTo() {
	dlg := new(walk.FileDialog)

	if mw.menuKeepLoc.Checked() {
		dlg.InitialDirPath = mw.prevFilePath
	}
	dlg.Title = "Select a Location to move files to"

	if ok, err := dlg.ShowBrowseFolder(Mw.MainWindow); err != nil {
		return
	} else if !ok {
		return
	}
	mw.prevFilePath = dlg.FilePath
}

func (mw *MyMainWindow) onMenuActionKeepLoc() {
	mw.menuKeepLoc.SetChecked(!mw.menuKeepLoc.Checked())

	if !mw.menuKeepLoc.Checked() {
		mw.prevFilePath = ""
	}
}
func (mw *MyMainWindow) onMenuView0() {
	mw.thumbView.ShowPreviewFull()
}
func (mw *MyMainWindow) onMenuView1() {

}
func (mw *MyMainWindow) onMenuView2() {

}
func (mw *MyMainWindow) onMenuView3() {
	// add a thumbviewer object
	if len(mw.thumbViews) == 2 { //allow only 3 total
		return
	}
	tvw, _ := NewScrollViewer(Mw.MainWindow, Mw.viewBase, true, 0, 0, 0)

	tvw.SetImageProcessorInfoFunc(Mw.imageProcessInfoHandler)
	tvw.SetDirectoryMonitorInfoFunc(Mw.directoryMonitorInfoHandler)
	tvw.SetProcessStatuswidget(Mw.StatusBar())
	tvw.SetEventMouseDown(Mw.onThumbViewMouseDn)

	tvw.SetItemSize(Mw.thumbView.itemSize.tw, Mw.thumbView.itemSize.th)
	tvw.SetLayoutMode(Mw.thumbView.GetLayoutMode())
	tvw.SetCacheMode(true)
	tvw.Run(mw.CurrentPath, nil, false)

	mw.thumbViews = append(mw.thumbViews, tviews{id: tvw.ID, viewer: tvw, handler: nil})
	mw.menuView4.SetEnabled(true)
}
func (mw *MyMainWindow) onMenuView4() {
	// remove a thumbviewer object

	for i, v := range mw.thumbViews {
		if v.viewer.scrollview.Focused() {

			err := v.viewer.destroy()
			if err != nil {
				log.Println("error removing item")
				//log.Fatal(err)
			}
			mw.viewBase.SendMessage(win.WM_SIZE, 0, 0)

			// resize the mw.thumbViews slice, removing element i
			mw.thumbViews = append(mw.thumbViews[:i], mw.thumbViews[i+1:]...)
			break
		}
	}
	mw.menuView4.SetEnabled(len(mw.thumbViews) > 0)
}

var treeItemPath string

func (mw *MyMainWindow) onTreeCurrentItemChanged() {

	dir := treeView.CurrentItem().(*Directory)
	mw.CurrentPath = dir.Path()

	//	go func(spath string) {
	//		tm := time.NewTimer(time.Millisecond * 50)

	//		select {
	//		case <-tm.C:
	//			tm.Stop()
	//			Mw.MainWindow.Synchronize(func() {

	//				if err := mw.thumbView.Run(spath, nil, true); err != nil {
	//					walk.MsgBox(
	//						mw.MainWindow,
	//						"Error",
	//						err.Error(),
	//						walk.MsgBoxOK|walk.MsgBoxIconError)
	//				}
	//			})
	//		}
	//	}(mw.CurrentPath)

	doTreeChange := func() {
		if err := mw.thumbView.Run(mw.CurrentPath, nil, true); err != nil {
			walk.MsgBox(
				mw.MainWindow,
				"Error",
				err.Error(),
				walk.MsgBoxOK|walk.MsgBoxIconError)
		}
	}
	defer doTreeChange()

	mw.actionAlbumItem1.SetVisible(true)
	mw.actionAlbumItem2.SetVisible(false)
	mw.actionAlbumItem3.SetVisible(false)
}

func (mw *MyMainWindow) onTreeMouseDown(x, y int, button walk.MouseButton) {
	if button == walk.RightButton {
		if item := treeView.ItemAt(x, y); item != nil {
			treeItemPath = item.(*Directory).Path()
			treeView.SetContextMenu(mw.treeMenu)
		} else {
			treeView.SetContextMenu(nil)
			treeItemPath = ""
		}
		mnu := mw.treeMenu.Actions().At(0)
		mnu.SetText("&Explore " + treeItemPath)
	}
}
func (mw *MyMainWindow) onThumbViewMouseDn(x, y int, button walk.MouseButton) {

	var menuBounds *image.Rectangle

	// get the valid menu area for the item in pos x,y
	if menuBounds = mw.thumbView.GetItemMenuRectAtScreen(x, y); menuBounds == nil {
		mw.StatusBar().Items().At(3).SetText(" 0 selected")
		return
	}

	if r := mw.thumbView.PreviewRect; r != nil {
		rr := image.Rect(r.X, r.Y, r.X+r.Width, r.Y+r.Height)
		menuBounds = &rr
	}

	idx := mw.thumbView.SelectedIndex
	if mw.thumbView.isValidIndex(idx) {
		// popup the ctx menu, depending on the
		// mouse x,y in the image area.
		if button == walk.RightButton {
			pt := image.Point{x, y}
			if pt.In(*menuBounds) {
				mw.thumbView.SetContextMenu(Mw.menuItemAction)
			} else {
				mw.thumbView.SetContextMenu(nil)
			}
		}

		mw.StatusBar().Items().At(3).SetText(fmt.Sprintf("  %d selected   ", len(mw.thumbView.selections)))
		mw.StatusBar().Items().At(4).SetText(mw.thumbView.GetItemName(idx) + "  " + mw.thumbView.GetItemInfo(idx))
	}
}

func (mw *MyMainWindow) onToolbarSizeChanged() {
	if mw.btnOptions != nil {
		mw.btnOptions.SetBounds(walk.Rectangle{mw.topComposite.Bounds().Width - 42, 7, 40, 28})
	}
}
func (mw *MyMainWindow) UpdateAddreebar(spath string) {
	f := false
	for i, adr := range addrList {
		if adr == spath {
			f = true
			mw.cmbAddr.SetCurrentIndex(i)
			break
		}
	}

	if !f {
		addrList = append(addrList, spath)
		Mw.cmbAddr.SetModel(addrList)
	}

	Mw.cmbAddr.SetText(spath)
}

func addMenuActions(wmenu *walk.Menu, text string, onTriggered walk.EventHandler,
	isSeparator, canCheck, isChecked bool) *walk.Action {
	var itm *walk.Action
	if !isSeparator {
		itm = walk.NewAction()
		itm.SetText(text)
		itm.SetCheckable(canCheck)
		itm.SetChecked(isChecked)
		itm.Triggered().Attach(onTriggered)
	} else {
		itm = walk.NewSeparatorAction()
	}
	wmenu.Actions().Add(itm)
	return itm
}

func (mw *MyMainWindow) onAppClose(canceled *bool, reason walk.CloseReason) {
	*canceled = false
	//mw.MainWindow.Close()
}

var cmp00, cmp03 *walk.Composite
var hdr1, hdr2, hdr3 *walk.Composite
var brs *walk.SolidColorBrush
var albumData1, albumData2 *walk.TextEdit

func main() {
	var err error

	treeModel, err = NewDirectoryTreeModel()
	if err != nil {
		log.Fatal(err)
	}

	// These specify the app data sub directory for the settings file.
	app := walk.App()
	app.SetOrganizationName("MLN")
	app.SetProductName("GoImageBrowser")

	// Settings file name.
	settings = walk.NewIniFileSettings("settings.ini")
	if err := settings.Load(); err != nil {
		log.Fatal(err)
	}

	//apply settings to window
	app.SetSettings(settings)

	var lbl1 *walk.Label

	myFont := *new(Font)
	myFont.PointSize = 10
	myFont2 := *new(Font)
	myFont2.PointSize = 10
	myFont2.Bold = true

	brs, _ = walk.NewSolidColorBrush(walk.RGB(195, 200, 205))

	if err := (MainWindow{
		AssignTo: &Mw.MainWindow,
		Name:     "mainBrowserWindow",
		Title:    "Walk Image Browser",
		Layout:   VBox{Margins: Margins{Top: 0, Left: 4, Right: 2, Bottom: 0}, MarginsZero: false},
		MinSize:  Size{600, 400},
		Size:     Size{1200, 600},
		MenuItems: []MenuItem{
			Menu{
				Text: "&File",
				Items: []MenuItem{
					Action{
						Text:        "&Reload",
						OnTriggered: Mw.onMenuActionReload,
					},
					Menu{
						//AssignTo: &recentMenu,
						Text: "Recent",
					},
					Separator{},
					Action{
						Text:        "E&xit",
						OnTriggered: func() { Mw.Close() },
					},
				},
			},
			Menu{
				Text: "&Tools",
				Items: []MenuItem{
					Action{
						AssignTo:    &Mw.menuTest1,
						Text:        "Scroll test",
						OnTriggered: Mw.onTest1,
					},
					Action{
						AssignTo:    &Mw.menuTest2,
						Text:        "Create test image files...",
						OnTriggered: Mw.onTest2,
					},
					Action{
						AssignTo:    &Mw.menuTest3,
						Text:        "Dump in memory cache to disk...",
						OnTriggered: Mw.onTest3,
					},
				},
			},
			Menu{
				Text:     "&View",
				AssignTo: &Mw.menuView,
				Items: []MenuItem{
					Action{
						AssignTo:    &Mw.menuView0,
						Text:        "View image in new window",
						Checkable:   true,
						OnTriggered: Mw.onMenuView0,
					},
					Action{
						AssignTo:    &Mw.menuView1,
						Text:        "Show Folder tree",
						Checkable:   true,
						OnTriggered: Mw.onMenuView1,
					},
					Action{
						AssignTo:    &Mw.menuView2,
						Text:        "Show File list",
						Checkable:   true,
						OnTriggered: Mw.onMenuView2,
					},
					Separator{},
					Action{
						AssignTo:    &Mw.menuView3,
						Text:        "Add more Viewers",
						OnTriggered: Mw.onMenuView3,
					},
					Action{
						AssignTo:    &Mw.menuView4,
						Text:        "Remove Viewer",
						OnTriggered: Mw.onMenuView4,
						Enabled:     false,
					},
				},
			},
			Menu{
				Text: "&Help",
				Items: []MenuItem{
					Action{
						Text: "About",
						//OnTriggered: mw.showAboutBoxAction_Triggered,
					},
				},
			},
		},
		Children: []Widget{
			Composite{
				Layout:        Grid{Columns: 3},
				AssignTo:      &Mw.topComposite,
				OnSizeChanged: Mw.onToolbarSizeChanged,
				Font:          myFont,
				Children: []Widget{
					Label{
						AssignTo: &Mw.lblAddr,
						Text:     "Address:",
					},
					ComboBox{
						AssignTo:   &Mw.cmbAddr,
						Editable:   true,
						ColumnSpan: 1,
					},
					HSpacer{
						Size: 40,
					},
				},
			},
			HSplitter{
				AssignTo:    &Mw.hSplitter,
				Name:        "mainSplitter",
				HandleWidth: 6,
				Children: []Widget{
					Composite{
						AssignTo: &cmp00,
						Layout:   VBox{Margins: Margins{0, 1, 4, 1}},
						Name:     "leftbar",
						Font:     myFont,
						OnSizeChanged: func() {

							if cmp03.Visible() {
								cmp03.SetMinMaxSize(walk.Size{0, 120}, walk.Size{0, 120})
							}

							if Mw.compAlbum.Visible() {
								b := cmp00.ClientBounds()
								Mw.compAlbum.SetMinMaxSize(walk.Size{0, b.Height - Mw.compAlbum.Bounds().Top() - 7},
									walk.Size{0, 0})
							}
							if Mw.StatusBar().Items() != nil {
								if Mw.StatusBar().Items().Len() > 1 {
									Mw.StatusBar().Items().At(1).SetWidth(cmp00.Width() - 100)
								}
							}
						},
						Children: []Widget{
							Composite{
								AssignTo: &hdr1,
								Name:     "leftbar-header1",
								Layout:   HBox{Margins: Margins{4, 1, 4, 1}},
								MinSize:  Size{0, 24},
								MaxSize:  Size{0, 24},
								Children: []Widget{
									ToolButton{Text: "-",
										OnMouseUp: func(x, y int, mb walk.MouseButton) {
											Mw.folderShow(!Mw.compFolder.Visible())
										}},
									Label{
										AssignTo: &lbl1,
										Text:     "Folders",
									},
									HSpacer{},
								},
							},
							Composite{
								AssignTo: &Mw.compFolder,
								Name:     "treebasecomp",
								Layout:   HBox{Margins: Margins{16, 0, 0, 1}},
								MinSize:  Size{0, 200},
								MaxSize:  Size{0, 360},
								Children: []Widget{
									TreeView{
										Name:     "treecomp",
										AssignTo: &treeView,
										Model:    treeModel,

										OnCurrentItemChanged: Mw.onTreeCurrentItemChanged,
										OnMouseDown:          Mw.onTreeMouseDown,
									},
								},
							},

							Composite{
								AssignTo: &hdr3,
								Name:     "leftbar-header3",
								Layout:   HBox{Margins: Margins{4, 0, 4, 0}},
								MinSize:  Size{0, 24},
								MaxSize:  Size{0, 24},
								Children: []Widget{
									ToolButton{Text: "+",
										OnMouseUp: func(x, y int, mb walk.MouseButton) {
											Mw.albumEditorShow(!cmp03.Visible())
										}},
									Label{
										AssignTo: &lbl1,
										Text:     "Create Albums",
									},
									HSpacer{},
								},
							},
							Composite{
								AssignTo: &cmp03,
								Name:     "editorbasecomp",
								Layout:   Grid{Columns: 1, Margins: Margins{1, 0, 1, 0}, SpacingZero: true},
								Children: []Widget{
									Composite{
										Layout: Grid{Columns: 2, Margins: Margins{Bottom: 0}},
										Children: []Widget{
											Label{
												Text: "Album name:",
											},
											TextEdit{AssignTo: &albumData1, Font: myFont2},
											Label{
												Text:       "Description:",
												Column:     0,
												ColumnSpan: 1,
											},
											TextEdit{AssignTo: &albumData2, Font: myFont2,
												MinSize: Size{0, 50},
											},
										}},
									Composite{
										Layout: Grid{Columns: 3, Margins: Margins{4, 0, 8, 4}},
										Children: []Widget{
											HSpacer{},
											PushButton{
												Text: "Cancel",
												OnClicked: func() {
													Mw.albumCancel()
													Mw.albumEditorShow(false)
												},
											},
											PushButton{
												Text: "Save",
												OnClicked: func() {
													Mw.albumSaveEdit()
													Mw.albumEditorShow(false)
												},
											},
										},
									},
								},
							},
							Composite{
								AssignTo: &hdr2,
								Name:     "leftbar-header2",
								Layout:   HBox{Margins: Margins{4, 0, 4, 0}},
								MinSize:  Size{0, 24},
								MaxSize:  Size{0, 24},
								Children: []Widget{
									ToolButton{Text: "+",
										OnMouseUp: func(x, y int, mb walk.MouseButton) {
											Mw.albumShow(!Mw.compAlbum.Visible())
										}},
									Label{
										Text: "Albums",
									},
									HSpacer{},
								},
							},
							Composite{
								AssignTo: &Mw.compAlbum,
								Name:     "albumbasecomp",
								Layout:   HBox{Margins: Margins{16, 0, 0, 0}},
							},
							VSpacer{},
						},
					},
					Composite{
						Name:          "scrollviewComposite",
						Layout:        VBox{MarginsZero: true},
						StretchFactor: 2,
						AssignTo:      &Mw.viewBase,
					},
				},
			},
		},
	}.Create()); err != nil {
		log.Fatal(err)
	}

	Mw.hSplitter.SetFixed(cmp00, true)

	hdr1.SetBackground(brs)

	fn, _ := walk.NewFont(Mw.Font().Family(), 9, Mw.Font().Style())
	sbr := Mw.StatusBar()
	sbr.SetFont(fn)
	sbr.SetVisible(true)

	sbi := walk.NewStatusBarItem()
	sbi.SetWidth(100)
	sbr.Items().Add(sbi)

	sbi = walk.NewStatusBarItem()
	sbi.SetWidth(230)
	sbr.Items().Add(sbi)

	sbi = walk.NewStatusBarItem()
	sbi.SetWidth(80)
	sbr.Items().Add(sbi)

	sbi = walk.NewStatusBarItem()
	sbi.SetWidth(100)
	sbr.Items().Add(sbi)

	sbi = walk.NewStatusBarItem()
	sbi.SetWidth(600)
	sbr.Items().Add(sbi)

	Mw.menuView1.SetChecked(true)
	Mw.menuView2.SetChecked(true)

	//-----------
	//Thumbviewer
	//-----------
	Mw.thumbView, _ = NewScrollViewer(Mw.MainWindow, Mw.viewBase, true, 0, 0, 0)

	Mw.thumbView.SetImageProcessorInfoFunc(Mw.imageProcessInfoHandler)
	Mw.thumbView.SetDirectoryMonitorInfoFunc(Mw.directoryMonitorInfoHandler)
	Mw.thumbView.SetProcessStatuswidget(Mw.StatusBar())
	Mw.thumbView.SetEventMouseDown(Mw.onThumbViewMouseDn)

	//initialize cache database
	defer Mw.thumbView.CloseCacheDB()

	Mw.btnOptions, _ = walk.NewPushButton(Mw.topComposite)
	Mw.btnOptions.SetText("   ")
	img, err := walk.NewImageFromFile("./image/menu.png")
	Mw.btnOptions.SetImage(img)
	Mw.btnOptions.SetImageAboveText(true)
	//Mw.btnOptions.Clicked().Attach(Mw.thumbView.SetOptionMode)
	Mw.onToolbarSizeChanged()

	//context menus
	menu, _ := walk.NewMenu()
	Mw.actionAlbumItem1 = addMenuActions(menu, "&Add to Album", Mw.AlbumAddItems, false, false, false)
	Mw.actionAlbumItem2 = addMenuActions(menu, "&Remove from Album", Mw.AlbumDeleteItems, false, false, false)
	Mw.actionAlbumItem3 = addMenuActions(menu, "&Set as Album cover image", Mw.AlbumSetCover, false, false, false)

	addMenuActions(menu, "", nil, true, false, false)
	addMenuActions(menu, "&Preview", Mw.onMenuActionPreview, false, false, false)
	addMenuActions(menu, "&Quickview", Mw.onMenuActionPreview2, false, false, false)
	addMenuActions(menu, "", nil, true, false, false)
	addMenuActions(menu, "&Delete", Mw.onMenuActionDelete, false, false, false)
	addMenuActions(menu, "&Rename", Mw.onMenuActionRename, false, false, false)
	addMenuActions(menu, "&Copy to...", Mw.onMenuActionCopyTo, false, false, false)
	addMenuActions(menu, "&Move to...", Mw.onMenuActionMoveTo, false, false, false)
	addMenuActions(menu, "", nil, true, false, false)
	Mw.menuKeepLoc = addMenuActions(menu, "&Keep last location", Mw.onMenuActionKeepLoc, false, false, false)

	Mw.menuItemAction = menu
	Mw.thumbView.SetContextMenu(menu)

	//Album context menus
	menu, _ = walk.NewMenu()
	Mw.actionAlbumSort1 = addMenuActions(menu, "Sort by name", Mw.albumSortName, false, true, false)
	Mw.actionAlbumSort2 = addMenuActions(menu, "Sort by date", Mw.albumSortDate, false, true, false)
	Mw.actionAlbumSort3 = addMenuActions(menu, "Sort by size", Mw.albumSortSize, false, true, false)
	addMenuActions(menu, "", nil, true, false, false)
	addMenuActions(menu, "&Edit Album", Mw.albumEdit, false, false, false)
	addMenuActions(menu, "", nil, true, false, false)
	addMenuActions(menu, "&Delete Album", nil, false, false, false)
	Mw.albumMenu = menu

	//Treeview context menus
	menu, _ = walk.NewMenu()
	addMenuActions(menu, "&Explore", Mw.onMenuActionExplore, false, false, false)
	addMenuActions(menu, "&Rename", nil, false, false, false)
	addMenuActions(menu, "", nil, true, false, false)
	addMenuActions(menu, "&Delete", nil, false, false, false)
	addMenuActions(menu, "", nil, true, false, false)
	addMenuActions(menu, "&Reload", Mw.onMenuActionReload, false, false, false)

	Mw.treeMenu = menu

	cmp03.SetVisible(false)
	Mw.compAlbum.SetVisible(false)

	if s, ok := settings.Get("LeftBar-Folders"); ok {
		b, _ := strconv.ParseBool(s)
		Mw.folderShow(b)
	}
	if s, ok := settings.Get("LeftBar-Albums"); ok {
		b, _ := strconv.ParseBool(s)

		if b {
			Mw.albumShow(true)
		}
	}

	if s, ok := settings.Get("Cached"); ok {
		b, _ := strconv.ParseBool(s)
		Mw.thumbView.SetCacheMode(b)
	}
	w, h := 120, 75
	if s, ok := settings.Get("ThumbW"); ok {
		w, _ = strconv.Atoi(s)
		if w < 64 {
			w = 64
		}
	}
	if s, ok := settings.Get("ThumbH"); ok {
		h, _ = strconv.Atoi(s)
		if h < 40 {
			h = 40
		}
	}
	Mw.thumbView.SetItemSize(w, h)

	if s, ok := settings.Get("LayoutMode"); ok {
		idx, _ := strconv.Atoi(s)

		Mw.thumbView.SetLayoutMode(idx)
	}

	if s, ok := settings.Get("SortMode"); ok {
		idx, _ := strconv.Atoi(s)
		ord := 0
		if s, ok = settings.Get("SortOrder"); ok {
			ord, _ = strconv.Atoi(s)
		}
		Mw.thumbView.SetSortMode(idx, ord)
	}

	if s, ok := settings.Get("LastAddress"); ok && s != "" {

		go func(spath string) {
			tm := time.NewTimer(time.Millisecond * 100)

			select {
			case <-tm.C:
				tm.Stop()
				Mw.MainWindow.Synchronize(func() {
					LocatePath(spath)
				})
			}
		}(s)
	}

	//experimental net server
	go StartNet()

	/*-----------------------------
	   START THE WINDOW MAIN LOOP
	------------------------------*/
	Mw.MainWindow.Run()

	//on exit, save settings
	settings.Put("LeftBar-Folders", strconv.FormatBool(Mw.visibleFolder))
	settings.Put("LeftBar-Albums", strconv.FormatBool(Mw.visibleAlbum))

	settings.Put("LastAddress", Mw.thumbView.itemsModel.dirPath)
	settings.Put("ThumbW", strconv.Itoa(Mw.thumbView.itemSize.tw))
	settings.Put("ThumbH", strconv.Itoa(Mw.thumbView.itemSize.th))
	settings.Put("Cached", strconv.FormatBool(Mw.thumbView.doCache))
	settings.Put("LayoutMode", strconv.Itoa(Mw.thumbView.GetLayoutMode()))
	settings.Put("SortMode", strconv.Itoa(Mw.thumbView.GetSortMode()))
	settings.Put("SortOrder", strconv.Itoa(Mw.thumbView.GetSortOrder()))
	settings.Put(Mw.thumbView.itemsModel.dirPath, strconv.Itoa(Mw.thumbView.viewInfo.topPos))

	if err := settings.Save(); err != nil {
		log.Fatal(err)
	}
}

func (mw *MyMainWindow) imageProcessInfoHandler(numjob int, d float64) {
	mw.Synchronize(func() {
		mw.StatusBar().Items().At(0).SetText(fmt.Sprintf("%6.3f s", d))
		mw.StatusBar().Items().At(2).SetText(fmt.Sprintf("  %d files", numjob))
		AppGetDirSettings(mw.thumbView, mw.thumbView.itemsModel.dirPath)
	})
}
func (mw *MyMainWindow) directoryMonitorInfoHandler(path string) {
	mw.Synchronize(func() {
		numItems := len(mw.thumbView.itemsModel.items)

		mw.MainWindow.SetTitle(path + " (" + strconv.Itoa(numItems) + " files)")
	})
}
