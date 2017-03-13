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

var _ walk.TreeItem = new(Directory)
var _ walk.TreeModel = new(DirectoryTreeModel)
var _ walk.ReflectTableModel = new(FileInfoModel)
var Mw = new(MyMainWindow)
var treeView *walk.TreeView
var treeModel *DirectoryTreeModel
var tableView *walk.TableView
var tableModel *FileInfoModel
var addrList []string
var settings *walk.IniFileSettings

type tviews struct {
	id      win.HWND
	viewer  *ScrollViewer
	handler *walk.Action
}

type MyMainWindow struct {
	*walk.MainWindow
	toolbar         *walk.CustomWidget
	hSplitter       *walk.Splitter
	viewBase        *walk.Composite
	thumbView       *ScrollViewer
	thumbViews      []tviews
	btn1            *walk.PushButton
	paintWidgetMenu *walk.Menu
	topComposite    *walk.Composite
	lblAddr         *walk.Label
	cmbAddr         *walk.ComboBox
	btnOptions      *walk.PushButton
	menuItemAction  *walk.Menu
	treeMenu        *walk.Menu
	menuView        *walk.Menu
	ViewSlider      *walk.Slider
	prevFilePath    string
	menuKeepLoc     *walk.Action
	menuTest1       *walk.Action
	menuTest2       *walk.Action
	menuTest3       *walk.Action
	menuView0       *walk.Action
	menuView1       *walk.Action
	menuView2       *walk.Action
	menuView3       *walk.Action
	menuView4       *walk.Action
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
			sc := mw.thumbView.scroller
			h := mw.thumbView.itemHeight
			c := 0
			for i := 0; i < sc.MaxValue(); i += h / 4 {
				sc.SetValue(i)
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

			mw.StatusBar().Items().At(2).SetText(fmt.Sprintf("scrolltest done in %6.3f sec. at %6.1f fps", d, fps))
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

	mw.thumbView.itemsModel.SetDirPath(mw.thumbView.itemsModel.dirPath, true)
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
	//treeView.SetVisible(!treeView.Visible())
	mw.menuView1.SetChecked(!mw.menuView1.Checked())
	treeView.Parent().(*walk.Splitter).SetWidgetVisible(treeView, !treeView.Visible())
	treeView.Parent().SendMessage(win.WM_SIZE, 0, 0)
}
func (mw *MyMainWindow) onMenuView2() {

	//mw.hSplitter.SetSplitterPos(20)
	//mw.hSplitter.SetWidgetWidth(mw.viewBase, 800)

	//	switch {
	//	case tableView.Visible():
	//		w := tableView.Width()
	//		tableView.Parent().(*walk.Splitter).SetWidgetWidth(tableView, w/2)
	//		tableView.Parent().(*walk.Splitter).SetWidgetVisible(tableView, false)
	//		if tableView.Parent().(*walk.Splitter).Width()-w > 200 {
	//			mw.hSplitter.SetWidgetWidth(mw.viewBase, mw.viewBase.Width()+w)
	//		}
	//	case !tableView.Visible():
	//		w := tableView.Parent().(*walk.Splitter).Width()
	//		tableView.Parent().(*walk.Splitter).SetWidgetVisible(tableView, true)
	//		tableView.Parent().(*walk.Splitter).SetWidgetWidth(tableView, w/2)
	//	}

	tableView.Parent().(*walk.Splitter).SetWidgetVisible(tableView, !tableView.Visible())

	mw.menuView2.SetChecked(!mw.menuView2.Checked())
	tableView.Parent().SendMessage(win.WM_SIZE, 0, 0)
}
func (mw *MyMainWindow) onMenuView3() {
	// add a thumbviewer object
	if len(mw.thumbViews) == 2 { //allow only 3 total
		return
	}
	tvw, _ := NewScrollViewer(Mw.MainWindow, Mw.viewBase, nil, 0, 0, 0)

	tvw.SetImageProcessorStatusFunc(Mw.imageProcessStatusHandler)
	tvw.SetImageProcessorInfoFunc(Mw.imageProcessInfoHandler)
	tvw.SetDirectoryMonitorInfoFunc(Mw.directoryMonitorInfoHandler)
	tvw.SetProcessStatuswidget(Mw.StatusBar())
	tvw.SetEventMouseDown(Mw.onThumbViewMouseDn)

	tvw.SetItemSize(Mw.thumbView.itemSize.tw, Mw.thumbView.itemSize.th)
	tvw.SetLayoutMode(Mw.thumbView.GetLayoutMode())
	tvw.SetCacheMode(true)
	tvw.Run(tableModel.dirPath, nil, false)

	mw.thumbViews = append(mw.thumbViews, tviews{id: tvw.ID, viewer: tvw, handler: nil})
	mw.menuView4.SetEnabled(true)
}
func (mw *MyMainWindow) onMenuView4() {
	// remove a thumbviewer object

	for i, v := range mw.thumbViews {
		if v.viewer.scroller.Focused() {

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

func (mw *MyMainWindow) OnTreeMouseDown(x, y int, button walk.MouseButton) {
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
	var idx int
	var bounds *image.Rectangle

	idx = mw.thumbView.GetItemAtScreen(x, y)

	bounds = mw.thumbView.getItemRectAtScreen(x, y)
	if bounds == nil {
		return
	}
	if mw.thumbView.GetLayoutMode() == 0 {
		bounds.Min.Y = bounds.Max.Y - 32
	}

	mw.StatusBar().Items().At(2).SetText("  " + mw.thumbView.GetItemName(idx) + "   " + mw.thumbView.GetItemInfo(idx))

	if mw.thumbView.isValidIndex(idx) {
		// popup the ctx menu, depending on the mouse x,y in the
		// image area.
		if button == walk.RightButton {
			pt := image.Point{x, y + mw.thumbView.viewInfo.topPos}
			if pt.In(*bounds) {
				mw.thumbView.suspendPreview = true
				mw.thumbView.SetContextMenu(Mw.menuItemAction)
			} else {
				mw.thumbView.suspendPreview = false
				mw.thumbView.SetContextMenu(nil)
			}
		}
	}
}

func (mw *MyMainWindow) onTableColClick(n int) {
	mw.thumbView.Invalidate()
}
func (mw *MyMainWindow) OnTableSelectedIndexesChanged() {
	//fmt.Printf("SelectedIndexes: %v\n", tableView.SelectedIndexes())
}
func (mw *MyMainWindow) OnTableCurrentIndexChanged() {
	var url string
	if index := tableView.CurrentIndex(); index > -1 {
		name := tableModel.items[index].Name

		dir := tableModel.dirPath
		url = filepath.Join(dir, name)
	}
	Mw.MainWindow.SetTitle(url)
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

func main() {
	var err error

	treeModel, err = NewDirectoryTreeModel()
	if err != nil {
		log.Fatal(err)
	}
	tableModel = NewFileInfoModel()

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

	myFont := *new(Font)
	myFont.PointSize = 10

	if err := (MainWindow{
		AssignTo: &Mw.MainWindow,
		Name:     "mainBrowserWindow",
		Title:    "Walk Image Browser",
		MinSize:  Size{600, 400},
		Size:     Size{1200, 600},
		Layout:   VBox{Margins: Margins{Top: 0, Left: 4, Right: 2, Bottom: 0}, MarginsZero: false},
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
			//CustomWidget{
			//				AssignTo:         &Mw.toolbar,
			//				ClearsBackground: true,
			//				//InvalidatesOnResize: true,
			//				//Paint:               Mw.onDrawPanel,
			//				MaxSize:     Size{2, 48},
			//				OnMouseDown: Mw.OnToolbarClick,

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
				Name:        "mainSplitter",
				HandleWidth: 6,
				AssignTo:    &Mw.hSplitter,

				Children: []Widget{
					HSplitter{
						HandleWidth: 6,
						Name:        "treetableSplitter",
						Children: []Widget{
							TreeView{
								AssignTo:             &treeView,
								Model:                treeModel,
								OnCurrentItemChanged: OnTreeCurrentItemChanged,
								OnMouseDown:          Mw.OnTreeMouseDown,
								Font:                 myFont,
							},
							TableView{
								AssignTo:              &tableView,
								AlternatingRowBGColor: walk.RGB(255, 255, 224),
								CheckBoxes:            true,
								ColumnsOrderable:      true,
								MultiSelection:        true,
								Font:                  myFont,
								Columns: []TableViewColumn{
									TableViewColumn{
										DataMember: "Name",
										Width:      240,
									},
									TableViewColumn{
										DataMember: "Size",
										Format:     "%d",
										Alignment:  AlignFar,
										Width:      64,
									},
									TableViewColumn{
										DataMember: "Modified",
										Format:     "2006-01-02 15:04:05",
										Width:      120,
									},
									TableViewColumn{
										DataMember: "Type",
										Width:      64,
									},
									TableViewColumn{
										DataMember: "Width",
										Alignment:  AlignFar,
										Format:     "%d",
										Width:      40,
									},
									TableViewColumn{
										DataMember: "Height",
										Alignment:  AlignFar,
										Format:     "%d",
										Width:      40,
									},
								},
								Model: tableModel,
								OnCurrentIndexChanged:    Mw.OnTableCurrentIndexChanged,
								OnSelectedIndexesChanged: Mw.OnTableSelectedIndexesChanged,
							},
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

	Mw.Closing().Attach(Mw.onAppClose)

	sbr := Mw.StatusBar()
	sbr.SetVisible(true)

	sbi := walk.NewStatusBarItem()
	sbi.SetWidth(120)
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
	Mw.thumbView, _ = NewScrollViewer(Mw.MainWindow, Mw.viewBase, nil, 0, 0, 0)

	Mw.thumbView.SetImageProcessorStatusFunc(Mw.imageProcessStatusHandler)
	Mw.thumbView.SetImageProcessorInfoFunc(Mw.imageProcessInfoHandler)
	Mw.thumbView.SetDirectoryMonitorInfoFunc(Mw.directoryMonitorInfoHandler)
	Mw.thumbView.SetProcessStatuswidget(Mw.StatusBar())
	Mw.thumbView.SetEventMouseDown(Mw.onThumbViewMouseDn)

	tableModel.viewer = Mw.thumbView

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

	//Treeview context menus
	menu, _ = walk.NewMenu()
	addMenuActions(menu, "&Explore", Mw.onMenuActionExplore, false, false, false)
	addMenuActions(menu, "&Rename", nil, false, false, false)
	addMenuActions(menu, "", nil, true, false, false)
	addMenuActions(menu, "&Delete", nil, false, false, false)
	addMenuActions(menu, "", nil, true, false, false)
	addMenuActions(menu, "&Reload", Mw.onMenuActionReload, false, false, false)

	Mw.treeMenu = menu

	//apply settings
	//	if s, ok := settings.Get("ThumbViewWidth"); ok {
	//		w, _ := strconv.Atoi(s)
	//		if w > 0 {
	//			if w > 800 {
	//				w = 400
	//			}
	//			Mw.hSplitter.SetWidgetWidth(Mw.viewBase, w)
	//		}
	//	}
	if s, ok := settings.Get("Cached"); ok {
		b, _ := strconv.ParseBool(s)
		Mw.thumbView.SetCacheMode(b)
	}
	w, h := 120, 75
	if s, ok := settings.Get("ThumbW"); ok {
		w, _ = strconv.Atoi(s)
		if w < 120 {
			w = 120
		}
	}
	if s, ok := settings.Get("ThumbH"); ok {
		h, _ = strconv.Atoi(s)
		if h < 75 {
			h = 75
		}
	}
	Mw.thumbView.SetItemSize(w, h)

	if s, ok := settings.Get("LayoutMode"); ok {
		idx, _ := strconv.Atoi(s)

		Mw.thumbView.SetLayoutMode(idx)
	}

	if s, ok := settings.Get("LastAddress"); ok {
		LocatePath(s)
	}

	tableView.ColumnClicked().Attach(Mw.onTableColClick)

	//experimental net server
	go StartNet()

	/*-----------------------------
	   START THE WINDOW MAIN LOOP
	------------------------------*/
	Mw.MainWindow.Run()

	//on exit, save settings
	settings.Put("LastAddress", tableModel.dirPath)
	settings.Put("ThumbW", strconv.Itoa(Mw.thumbView.itemSize.tw))
	settings.Put("ThumbH", strconv.Itoa(Mw.thumbView.itemSize.th))
	settings.Put("Cached", strconv.FormatBool(Mw.thumbView.doCache))
	settings.Put("LayoutMode", strconv.Itoa(Mw.thumbView.GetLayoutMode()))
	settings.Put(tableModel.dirPath, strconv.Itoa(Mw.thumbView.viewInfo.topPos))

	if err := settings.Save(); err != nil {
		log.Fatal(err)
	}
}

func (mw *MyMainWindow) imageProcessStatusHandler(i int) {
	if !mw.thumbView.doCache && i == mw.thumbView.NumCols() {
		mw.Synchronize(func() {
			mw.thumbView.Invalidate()
		})
	}
}
func (mw *MyMainWindow) imageProcessInfoHandler(numjob int, d float64) {
	mw.Synchronize(func() {
		mw.MainWindow.SetTitle(tableModel.dirPath + " (" + strconv.Itoa(numjob) + " files) in " + strconv.FormatFloat(d, 'f', 3, 64))

		mw.StatusBar().Items().At(0).SetText("  " + strconv.Itoa(numjob) + " files")
		mw.StatusBar().Items().At(1).SetText(strconv.FormatFloat(d, 'f', 3, 64) + " s")

		AppGetDirSettings(mw.thumbView, tableModel.dirPath)
	})
}
func (mw *MyMainWindow) directoryMonitorInfoHandler(path string) {
	mw.Synchronize(func() {
		tableModel.PublishRowsReset()
		numItems := len(tableModel.items)

		mw.MainWindow.SetTitle(path + " (" + strconv.Itoa(numItems) + " files)")
	})
}

type CustomWidgetView struct {
	*walk.CustomWidget
}

func (ctv *CustomWidgetView) WndProc(hwnd win.HWND, msg uint32, wp, lp uintptr) uintptr {
	//	switch msg {
	//	case win.WM_ERASEBKGND:
	//		log.Println("WM_ERASEBKGND")

	//	case win.WM_MOUSEWHEEL:
	//		log.Println("WM_MOUSEWHEEL", wp, lp)

	//		var cmd uint16
	//		if delta := int16(win.HIWORD(uint32(wp))); delta < 0 {
	//			cmd = win.SB_LINEDOWN
	//		} else {
	//			cmd = win.SB_LINEUP
	//		}

	//		ctv.SetY(Mw.scrollWidget.scroll(win.SB_VERT, cmd))

	//		return 0
	//	}
	return ctv.CustomWidget.WndProc(hwnd, msg, wp, lp)
}
