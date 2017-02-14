// Copyright 2011 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	//	"fmt"
	"image"
	"image/draw"
	"log"
	//"unsafe"
	//"runtime"
	"math"
	"strconv"
	//"time"
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

type viewRecord struct {
	SelectedName string
	SortMode     bool
}
type mapSelection map[string]viewRecord

type MyMainWindow struct {
	*walk.MainWindow
	toolbar *walk.CustomWidget
	//paintWidget  *walk.CustomWidget
	paintWidget     *CustomWidgetView
	pgBar           *walk.ProgressBar
	vspacebar       *walk.Slider
	scrollWidget    *walk.ScrollView
	btn1            *walk.PushButton
	paintWidgetMenu *walk.Menu
	topComposite    *walk.Composite
	lblAddr         *walk.Label
	lblSize         *walk.Label
	cmbAddr         *walk.ComboBox
	sldrSize        *walk.Slider
	cbCached        *walk.CheckBox
	menuItemAction  *walk.Action
}

func NewDirectory(name string, parent *Directory) *Directory {
	return &Directory{name: name, parent: parent}
}

func (mw *MyMainWindow) SetupPaintAreaSize(numItems int, reset bool) bool {

	if numItems > 0 {
		//Readjusting the paintwidget height & its container's height to reflect the num of items
		nCols := math.Trunc(float64(Mw.paintWidget.Width()) / float64(thumbR.twm()))
		nRows := math.Ceil(float64(numItems) / nCols)

		h := int(nRows) * thumbR.thm()
		if h < Mw.scrollWidget.ClientBounds().Height {
			h = Mw.scrollWidget.ClientBounds().Height
		}
		sz := walk.Size{Mw.scrollWidget.ClientBounds().Width - int(win.GetSystemMetrics(win.SM_CXVSCROLL)), h}
		Mw.paintWidget.SetMinMaxSize(sz, sz)
		Mw.scrollWidget.SendMessage(win.WM_SIZE, 0, 0)
	} else {
		Mw.paintWidget.SetMinMaxSize(walk.Size{4, 4}, walk.Size{5, 5})
		Mw.scrollWidget.SendMessage(win.WM_SIZE, 0, 0)
	}

	if reset {
		Mw.scrollWidget.AsContainerBase().SetY(0)
	}
	log.Println("Mw.paintWidget size", Mw.paintWidget.Size(), Mw.scrollWidget.ClientBounds().Height)
	return true
}

func (mw *MyMainWindow) onSVSizeChanged() {
	if mw.lblAddr != nil {
		//mw.SetupPaintAreaSize(len(tableModel.items), false)
	}
}

func (mw *MyMainWindow) OnBtn1Click() {
	mw.paintWidget.Invalidate()
}

func (mw *MyMainWindow) OnToolbarClick(x, y int, button walk.MouseButton) {
	//mw.paintWidget.Invalidate()

	//mw.paintWidget.AsWidgetBase().SetY(-100)
	//mw.scrollWidget.AsContainerBase().SetY(mw.scrollWidget.AsContainerBase().Y() - 100)
}

func (mw *MyMainWindow) onDrawPanelMouseDn(x, y int, button walk.MouseButton) {
	//log.Println("click: ", x, y)
	w := thumbR.twm()
	h := thumbR.thm()

	col := int(float32(x) / float32(w))
	row := int(float32(y) / float32(h))

	x1 := col * w
	y1 := row * h

	ctl := Mw.paintWidget
	//ctl.SetFocus()
	Mw.scrollWidget.SetFocus()

	idx := col + row*int(float32(ctl.Bounds().Width)/float32(w))
	if (idx >= 0) && (idx < len(tableModel.items)) {
		tableView.SetSelectedIndexes([]int{idx})

		//popup the ctx menu, depending on the mouse x,y in the
		//image area.
		if button == walk.RightButton {
			bounds := image.Rect(x1, y1+h-thumbR.txth, x1+w, y1+h)
			pt := image.Point{x, y}
			if pt.In(bounds) {
				ctl.SetContextMenu(Mw.paintWidgetMenu)
			} else {
				ctl.SetContextMenu(nil)
			}
		} else {
			ctl.Invalidate()
		}
	}
}

var dummyimg draw.Image

func (mw *MyMainWindow) onDrawPanelPaint(canvas *walk.Canvas, updateBounds walk.Rectangle) error {

	RedrawScreen(canvas, updateBounds, mw.scrollWidget.ClientBounds())
	return nil
}

func (mw *MyMainWindow) onTableColClick(n int) {
	mw.paintWidget.Invalidate()
}

func (mw *MyMainWindow) onToolbarSizeChanged() {
	if mw.lblAddr != nil {
		//mw.sldrSize.SetY(12)
	}
}

func (mw *MyMainWindow) OnToolbarCheckCache() {
	doCache = mw.cbCached.Checked()
}

func (mw *MyMainWindow) OnToolbarCacheSize() {
	thumbR.tw = mw.sldrSize.Value()
	thumbR.th = int(float64(thumbR.tw) * float64(10) / float64(16))
	mw.paintWidget.Invalidate()

	mw.lblSize.SetText("Thumbsize: " + strconv.Itoa(thumbR.tw) + "x" + strconv.Itoa(thumbR.th))
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

func main() {
	//runtime.GOMAXPROCS(runtime.NumCPU() * 1)
	//log.Println("cpu: ", runtime.NumCPU())
	var err error
	treeModel, err = NewDirectoryTreeModel()
	if err != nil {
		log.Fatal(err)
	}
	tableModel = NewFileInfoModel()

	//initialize cache database
	CreateCacheDB("")
	defer CloseCacheDB()

	myFont := *new(Font)
	//myFont.Family = "Arial"
	myFont.PointSize = 9

	// These specify the app data sub directory for the settings file.
	app := walk.App()
	app.SetOrganizationName("MLN")
	app.SetProductName("GoImageBrowser")

	// Settings file name.
	settings := walk.NewIniFileSettings("settings.ini")
	if err := settings.Load(); err != nil {
		log.Fatal(err)
	}
	//apply settings to window
	app.SetSettings(settings)

	cwv := new(CustomWidgetView)

	if err := (MainWindow{
		AssignTo: &Mw.MainWindow,
		Name:     "mainBrowserWindow",
		Title:    "Walk Image Browser",
		MinSize:  Size{600, 400},
		Size:     Size{1024, 550},
		Layout:   VBox{Margins: Margins{Top: 0}, MarginsZero: true},
		Children: []Widget{
			//CustomWidget{
			//				AssignTo:         &Mw.toolbar,
			//				ClearsBackground: true,
			//				//InvalidatesOnResize: true,
			//				//Paint:               Mw.onDrawPanel,
			//				MaxSize:     Size{2, 48},
			//				OnMouseDown: Mw.OnToolbarClick,
			Composite{
				Layout:        HBox{MarginsZero: false},
				AssignTo:      &Mw.topComposite,
				MinSize:       Size{100, 32},
				OnSizeChanged: Mw.onToolbarSizeChanged,
				Children:      []Widget{},
			},
			HSplitter{
				Children: []Widget{
					TreeView{
						AssignTo:             &treeView,
						Model:                treeModel,
						OnCurrentItemChanged: OnTreeCurrentItemChanged,
						Font:                 myFont,
					},
					TableView{
						AssignTo:              &tableView,
						StretchFactor:         1,
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
						OnCurrentIndexChanged:    OnTableCurrentIndexChanged,
						OnSelectedIndexesChanged: OnTableSelectedIndexesChanged,
					},
					ScrollView{
						AssignTo:      &Mw.scrollWidget,
						Layout:        VBox{Margins: Margins{Top: 0}, MarginsZero: true},
						StretchFactor: 3,
						OnSizeChanged: Mw.onSVSizeChanged,
						Children: []Widget{
							CustomWidget{
								AssignTo:            &cwv.CustomWidget, //&Mw.paintWidget,
								ClearsBackground:    true,
								InvalidatesOnResize: true,
								Paint:               Mw.onDrawPanelPaint,
								OnMouseDown:         Mw.onDrawPanelMouseDn,
								//OnMouseUp: Mw.onDrawPanelMouseDn,

								ContextMenuItems: []MenuItem{
									Action{
										Text:     "&Delete",
										AssignTo: &Mw.menuItemAction,
										//Image:       "../img/open.png",
										OnTriggered: Mw.OnBtn1Click,
									},
									Separator{},
									Action{
										Text:        "&Rename",
										AssignTo:    &Mw.menuItemAction,
										OnTriggered: Mw.OnBtn1Click,
									},
									Action{
										Text:        "&Copy to...",
										AssignTo:    &Mw.menuItemAction,
										OnTriggered: Mw.OnBtn1Click,
									},
									Action{
										Text:        "&Move to...",
										AssignTo:    &Mw.menuItemAction,
										OnTriggered: Mw.OnBtn1Click,
									},
								},
							},
						},
					},
				},
			},
			ProgressBar{
				AssignTo: &Mw.pgBar,
				Value:    0,
			},
		},
	}.Create()); err != nil {
		log.Fatal(err)
	}
	if err := walk.InitWrapperWindow(cwv); err != nil {
		log.Fatal(err)
	}

	Mw.lblAddr, _ = walk.NewLabel(Mw.topComposite)
	Mw.topComposite.Children().Add(Mw.lblAddr)
	Mw.lblAddr.SetText("Address:")

	Mw.cmbAddr, _ = walk.NewComboBox(Mw.topComposite)
	Mw.topComposite.Children().Add(Mw.cmbAddr)

	sp, _ := walk.NewHSpacerFixed(Mw.topComposite, 50)
	Mw.topComposite.Children().Add(sp)

	Mw.lblSize, _ = walk.NewLabel(Mw.topComposite)
	Mw.lblSize.SetText("Thumbsize:")
	Mw.topComposite.Children().Add(Mw.lblSize)

	Mw.sldrSize, _ = walk.NewSlider(Mw.topComposite)
	Mw.sldrSize.SetMinMaxSize(walk.Size{160, 26}, walk.Size{160, 26})
	Mw.sldrSize.SetRange(120, 360)
	Mw.sldrSize.ValueChanged().Attach(Mw.OnToolbarCacheSize)
	Mw.topComposite.Children().Add(Mw.sldrSize)

	sp2, _ := walk.NewHSpacerFixed(Mw.topComposite, 10)
	Mw.topComposite.Children().Add(sp2)

	Mw.cbCached, _ = walk.NewCheckBox(Mw.topComposite)
	Mw.cbCached.SetText("Cached:")
	Mw.cbCached.CheckedChanged().Attach(Mw.OnToolbarCheckCache)
	Mw.topComposite.Children().Add(Mw.cbCached)

	Mw.paintWidget = cwv
	Mw.paintWidgetMenu = cwv.ContextMenu()
	cwv.SetContextMenu(nil)

	//apply settings
	if s, ok := settings.Get("Cached"); ok {
		b, _ := strconv.ParseBool(s)
		Mw.cbCached.SetChecked(b)
		doCache = b
	}
	if s, ok := settings.Get("ThumbW"); ok {
		thumbR.tw, _ = strconv.Atoi(s)
	}
	if s, ok := settings.Get("ThumbH"); ok {
		thumbR.th, _ = strconv.Atoi(s)
	}
	if s, ok := settings.Get("LastAddress"); ok {
		LocatePath(s)
	}

	Mw.sldrSize.SetValue(thumbR.tw)
	tableView.ColumnClicked().Attach(Mw.onTableColClick)
	//experimental net server
	go StartNet()

	/*-----------------------------
	   START THE WINDOW MAIN LOOP
	------------------------------*/
	Mw.MainWindow.Run()

	//on exit, save settings
	settings.Put("LastAddress", tableModel.dirPath)
	settings.Put("ThumbW", strconv.Itoa(thumbR.tw))
	settings.Put("ThumbH", strconv.Itoa(thumbR.th))
	settings.Put("Cached", strconv.FormatBool(doCache))

	if err := settings.Save(); err != nil {
		log.Fatal(err)
	}
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
