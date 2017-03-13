// Copyright 2011 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"image"
	//"image/draw"
	"log"
	"strconv"
	//"time"
	"os"
)

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

//var _ walk.TreeItem = new(Directory)
//var _ walk.TreeModel = new(DirectoryTreeModel)
//var _ walk.ReflectTableModel = new(FileInfoModel)
//var Mw = new(MyMainWindow)
//var treeView *walk.TreeView
//var treeModel *DirectoryTreeModel
//var tableView *walk.TableView
//var tableModel *FileInfoModel
//var addrList []string
//var settings *walk.IniFileSettings

type ThumbViewWindow struct {
	*walk.MainWindow
	//toolbar *walk.CustomWidget
	topComposite *walk.Composite
	viewBase     *walk.Composite
	thumbView    *ScrollViewer

	lblAddr        *walk.Label
	cmbAddr        *walk.ComboBox
	btnOptions     *walk.PushButton
	menuItemAction *walk.Menu
	menuKeepLoc    *walk.Action
	ViewSlider     *walk.Slider
	prevFilePath   string
}

func (mw *ThumbViewWindow) onActionReload() {

	mw.thumbView.Run(mw.thumbView.itemsModel.dirPath, nil, true)

	mw.StatusBar().Invalidate()
	mw.MainWindow.SetTitle(mw.thumbView.itemsModel.dirPath + " (" + strconv.Itoa(mw.thumbView.itemsCount) + " files)")
	mw.UpdateAddreebar(mw.thumbView.itemsModel.dirPath)
}
func (mw *ThumbViewWindow) onActionDelete() {

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
func (mw *ThumbViewWindow) onActionRename() {

}
func (mw *ThumbViewWindow) onActionCopyTo() {
	dlg := new(walk.FileDialog)

	if mw.menuKeepLoc.Checked() {
		dlg.InitialDirPath = mw.prevFilePath
	}
	dlg.Title = "Select a Location to copy files to"

	if ok, err := dlg.ShowBrowseFolder(mw.MainWindow); err != nil {
		return
	} else if !ok {
		return
	}
	mw.prevFilePath = dlg.FilePath
}
func (mw *ThumbViewWindow) onActionMoveTo() {
	dlg := new(walk.FileDialog)

	if mw.menuKeepLoc.Checked() {
		dlg.InitialDirPath = mw.prevFilePath
	}
	dlg.Title = "Select a Location to move files to"

	if ok, err := dlg.ShowBrowseFolder(mw.MainWindow); err != nil {
		return
	} else if !ok {
		return
	}
	mw.prevFilePath = dlg.FilePath
}

func (mw *ThumbViewWindow) onActionKeepLoc() {
	mw.menuKeepLoc.SetChecked(!mw.menuKeepLoc.Checked())

	if !mw.menuKeepLoc.Checked() {
		mw.prevFilePath = ""
	}
}

func (mw *ThumbViewWindow) OnToolbarClick(x, y int, button walk.MouseButton) {

}

func (mw *ThumbViewWindow) onDrawPanelMouseDn(x, y int, button walk.MouseButton) {
	w := mw.thumbView.itemSize.twm()
	h := mw.thumbView.itemSize.thm()

	col := int(float32(x) / float32(w))
	row := int(float32(y+mw.thumbView.viewInfo.topPos) / float32(h))

	x1 := col * w
	y1 := row * h

	idx := mw.thumbView.GetItemAtScreen(x, y)
	if mw.thumbView.isValidIndex(idx) {
		//popup the ctx menu, depending on the mouse x,y in the
		//image area.
		if button == walk.RightButton {
			bounds := image.Rect(x1, y1+h-mw.thumbView.itemSize.txth, x1+w, y1+h)
			pt := image.Point{x, y + mw.thumbView.viewInfo.topPos}
			if pt.In(bounds) {
				mw.thumbView.suspendPreview = true
				mw.thumbView.SetContextMenu(mw.menuItemAction)
			} else {
				mw.thumbView.suspendPreview = false
				mw.thumbView.SetContextMenu(nil)
			}
		}
	}
}
func (mw *ThumbViewWindow) onTableColClick(n int) {
	mw.thumbView.Invalidate()
}
func (mw *ThumbViewWindow) UpdateAddreebar(spath string) {
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
		mw.cmbAddr.SetModel(addrList)
	}

	mw.cmbAddr.SetText(spath)
}
func (mw *ThumbViewWindow) onToolbarSizeChanged() {
	if mw.btnOptions != nil {
		mw.btnOptions.SetBounds(walk.Rectangle{mw.topComposite.Bounds().Width - 42, 7, 40, 28})
	}
}
func NewThumbViewWindow(parent *walk.MainWindow, newpath string) int {
	var tvw = new(ThumbViewWindow)

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
		AssignTo: &tvw.MainWindow,
		Name:     "ThumbViewWindow",
		Title:    newpath,
		MinSize:  Size{400, 240},
		Size:     Size{800, 480},
		Layout:   VBox{Margins: Margins{Top: 0, Left: 2, Right: 2, Bottom: 2}, MarginsZero: false},
		Children: []Widget{
			Composite{
				Layout:        Grid{Columns: 3},
				AssignTo:      &tvw.topComposite,
				MinSize:       Size{0, 32},
				MaxSize:       Size{0, 32},
				OnSizeChanged: tvw.onToolbarSizeChanged,
				Font:          myFont,
				Children: []Widget{
					Label{
						AssignTo: &tvw.lblAddr,
						Text:     "Address: ",
					},
					ComboBox{
						AssignTo:   &tvw.cmbAddr,
						Editable:   true,
						ColumnSpan: 1,
					},
					HSpacer{
						Size: 40,
					},
				},
			},
			Composite{
				Name:   "thumbviewComposite",
				Layout: HBox{MarginsZero: true},
				//StretchFactor: 3,
				//RowSpan:  3,
				AssignTo: &tvw.viewBase,
			},
		},
	}.Create()); err != nil {
		return 0
	}

	sbr := tvw.StatusBar()
	sbr.SetVisible(true)

	sbi := walk.NewStatusBarItem()
	sbi.SetWidth(120)
	sbr.Items().Add(sbi)

	sbi = walk.NewStatusBarItem()
	sbi.SetWidth(120)
	sbr.Items().Add(sbi)

	//-----------
	//Thumbviewer
	//-----------
	tvw.thumbView, _ = NewScrollViewer(tvw.MainWindow, tvw.viewBase, nil, 0, 0, 0)

	tvw.thumbView.SetImageProcessorStatusFunc(tvw.imageProcessStatusHandler)
	tvw.thumbView.SetImageProcessorInfoFunc(tvw.imageProcessInfoHandler)
	tvw.thumbView.SetDirectoryMonitorInfoFunc(tvw.directoryMonitorInfoHandler)
	tvw.thumbView.SetProcessStatuswidget(tvw.StatusBar())
	tvw.thumbView.SetEventMouseDown(tvw.onDrawPanelMouseDn)

	//	tvw.lblAddr, _ = walk.NewLabel(tvw.topComposite)
	//	tvw.topComposite.Children().Add(tvw.lblAddr)
	//	tvw.lblAddr.SetText("Address:")

	//	tvw.cmbAddr, _ = walk.NewComboBox(tvw.topComposite)
	//	tvw.topComposite.Children().Add(tvw.cmbAddr)

	tvw.btnOptions, _ = walk.NewPushButton(tvw.topComposite)
	tvw.btnOptions.SetText("   ")
	img, _ := walk.NewImageFromFile("./image/menu.png")
	tvw.btnOptions.SetImage(img)
	tvw.btnOptions.SetImageAboveText(true)
	//tvw.btnOptions.Clicked().Attach(tvw.thumbView.SetOptionMode)

	//context menus
	menu, _ := walk.NewMenu()

	itm := walk.NewAction()
	itm.SetText("&Delete")
	itm.Triggered().Attach(tvw.onActionDelete)
	menu.Actions().Add(itm)

	itm = walk.NewSeparatorAction()
	menu.Actions().Add(itm)

	itm = walk.NewAction()
	itm.SetText("&Rename")
	itm.Triggered().Attach(tvw.onActionRename)
	menu.Actions().Add(itm)

	itm = walk.NewAction()
	itm.SetText("&Copy to...")
	itm.Triggered().Attach(tvw.onActionCopyTo)
	menu.Actions().Add(itm)

	itm = walk.NewAction()
	itm.SetText("&Move to...")
	itm.Triggered().Attach(tvw.onActionMoveTo)
	menu.Actions().Add(itm)

	itm = walk.NewSeparatorAction()
	menu.Actions().Add(itm)

	itm = walk.NewAction()
	itm.SetText("    &Keep last location")
	itm.SetCheckable(true)
	itm.SetChecked(true)
	itm.Triggered().Attach(tvw.onActionKeepLoc)
	menu.Actions().Add(itm)
	tvw.menuKeepLoc = itm

	itm = walk.NewSeparatorAction()
	menu.Actions().Add(itm)

	itm = walk.NewAction()
	itm.SetText("&Reload")
	itm.Triggered().Attach(tvw.onActionReload)
	menu.Actions().Add(itm)

	tvw.menuItemAction = menu

	tvw.thumbView.SetContextMenu(menu)

	//apply settings
	if s, ok := settings.Get("Cached"); ok {
		b, _ := strconv.ParseBool(s)
		tvw.thumbView.SetCacheMode(b)
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
	tvw.thumbView.SetItemSize(w, h)

	//experimental net server
	//go StartNet()

	//create map containing the file infos
	//and launch image cache setup
	tvw.thumbView.Run(newpath, nil, true)

	tvw.StatusBar().Invalidate()
	tvw.MainWindow.SetTitle(newpath + " (" + strconv.Itoa(tvw.thumbView.itemsCount) + " files)")
	tvw.UpdateAddreebar(newpath)

	tvw.onToolbarSizeChanged()
	/*-----------------------------
	   START THE WINDOW MAIN LOOP
	------------------------------*/
	id := tvw.MainWindow.Run()

	//	//on exit, save settings
	//	settings.Put("LastAddress", tableModel.dirPath)
	//	settings.Put("ThumbW", strconv.Itoa(thumbR.tw))
	//	settings.Put("ThumbH", strconv.Itoa(thumbR.th))
	//	settings.Put("Cached", strconv.FormatBool(doCache))

	settings.Put(tvw.thumbView.itemsModel.dirPath, strconv.Itoa(tvw.thumbView.viewInfo.topPos))

	if err := settings.Save(); err != nil {
		log.Fatal(err)
	}
	log.Print("exit form...")
	return id
}

//-----------------------------------------
//informational function callback handlers
//-----------------------------------------
//called during the image process run
func (mw *ThumbViewWindow) imageProcessStatusHandler(i int) {
	if !mw.thumbView.doCache && i == mw.thumbView.NumCols() {
		mw.Synchronize(func() {
			mw.thumbView.Invalidate()
		})
	}
}

//called upon completion of image process run
func (mw *ThumbViewWindow) imageProcessInfoHandler(numjob int, d float64) {
	mw.Synchronize(func() {
		mw.MainWindow.SetTitle(mw.thumbView.itemsModel.dirPath + " (" + strconv.Itoa(numjob) + " files) in " + strconv.FormatFloat(d, 'f', 3, 64))

		mw.StatusBar().Items().At(0).SetText("  " + strconv.Itoa(numjob) + " files")
		mw.StatusBar().Items().At(1).SetText(strconv.FormatFloat(d, 'f', 3, 64) + " s")

		AppGetDirSettings(mw.thumbView, mw.thumbView.itemsModel.dirPath)
	})
}

//called upon completion of image process run
func (mw *ThumbViewWindow) directoryMonitorInfoHandler(path string) {
	mw.Synchronize(func() {
		mw.MainWindow.SetTitle(path + " (" + strconv.Itoa(mw.thumbView.itemsCount) + " files)")
	})
}
