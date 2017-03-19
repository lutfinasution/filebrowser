// Copyright 2012 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"image"
	"log"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

type ItmMap map[string]*FileInfo
type painterfunc func(sv *ScrollViewer, canvas *walk.Canvas, updaterect walk.Rectangle, viewrect walk.Rectangle) error

type ScrollViewer struct {
	MainWindow       *walk.MainWindow
	scrollview       *CustomScrollComposite
	canvasView       *walk.CustomWidget
	optionsPanel     *walk.Composite
	ID               win.HWND
	itemSize         ThumbSizes
	itemsCount       int
	itemWidth        int
	itemHeight       int
	SelectedIndex    int
	currentLayout    int
	currentSortIndex int
	currentSortOrder int
	dblClickTime     time.Time
	// basic data structs
	itemsModel *FileInfoModel
	ItemsMap   ItmMap
	viewInfo   ViewInfo
	// concurrent processors
	imageProcessor   *ImageProcessor
	contentMonitor   *ContentMonitor
	directoryMonitor *DirectoryMonitor
	// screen drawers:
	drawersCount  int
	drawerHDC     win.HDC
	drawerBuffer  *drawBuffer
	drawerFunc    func(sv *ScrollViewer, data *FileInfo)
	drawersChan   chan *FileInfo
	drawersWait   sync.WaitGroup
	drawerMutex   sync.Mutex
	drawersActive bool
	// local vars
	ViewerMode         bool // true=as thumbviewer, false=as album list
	suspendPreview     bool
	isResizing         bool
	doCache            bool
	handleChangedItems bool
	lastButtonDown     walk.MouseButton
	selections         []*FileInfo
	selStart           int
	selStop            int
	PreviewRect        *walk.Rectangle
	previewBackground  *walk.Bitmap
	// ui
	lblSize  *walk.Label
	cmbSort  *walk.ComboBox
	cmbMode  *walk.ComboBox
	sldrSize *walk.Slider
	cbCached *walk.CheckBox
	// public event handlers
	evPaint            painterfunc
	evMouseDown        walk.MouseEventHandler
	OnAlbumEditing     func(id int, name string, desc string)
	OnSelectionChanged func()
}

type ViewInfo struct {
	topPos      int //Y
	lastPos     int
	lastMovePos int
	numCols     int
	numRows     int
	viewRows    int
	viewRect    image.Rectangle
	//
	mouseposX  int
	mouseposY  int
	mousemoveX int
	mousemoveY int
	offsetX    int
	offsetY    int
	currentPos *walk.Point
	//
	scrollpos int
	scrolling bool
	showName  bool
	showDate  bool
	showInfo  bool
}

func NewScrollViewer(window *walk.MainWindow, parent walk.Container, viewerMode bool,
	itmCount, itmWidth, itmHeight int) (*ScrollViewer, error) {
	var err error
	var defSize = ThumbSizes{120, 75, 10, 10, 48}

	svr := &ScrollViewer{
		MainWindow:       window,
		itemsCount:       itmCount,
		itemWidth:        defSize.twm(),
		itemHeight:       defSize.thm(),
		itemSize:         defSize,
		SelectedIndex:    -1,
		currentLayout:    0,
		currentSortIndex: 0,
		currentSortOrder: 0,
		ViewerMode:       viewerMode,
		ItemsMap:         make(map[string]*FileInfo),
	}

	var sModel []string

	if !viewerMode {
		sModel = []string{
			" Frameless, variable size",
			" Grid with name, date, and size",
			" Grid with name and date",
			" Grid with name and size",
			" Grid with name only",
			" Grid with no text",
			" Infocard",
			" Album"}
	} else {
		sModel = []string{
			" Frameless, variable size",
			" Grid with name, date, and size",
			" Grid with name and date",
			" Grid with name and size",
			" Grid with name only",
			" Grid with no text",
			" Infocard"}
	}

	svr.itemsModel = new(FileInfoModel)
	svr.itemsModel.viewer = svr

	if viewerMode {
		svr.imageProcessor = new(ImageProcessor)
		svr.contentMonitor = new(ContentMonitor)
		svr.contentMonitor.imageprocessor = svr.imageProcessor

		svr.directoryMonitor = new(DirectoryMonitor)
		svr.directoryMonitor.viewer = svr
		svr.directoryMonitor.imagemon = svr.contentMonitor
	}

	parent.SetSuspended(true)
	defer parent.SetSuspended(false)

	//UI components:
	svr.scrollview, _ = NewCustomScrollComposite(parent, svr)
	svr.canvasView, _ = walk.NewCustomWidget(svr.scrollview, 0, svr.onPaint)

	svr.canvasView.SetPaintMode(walk.PaintNoErase)
	svr.canvasView.SetInvalidatesOnResize(true)
	svr.optionsPanel, err = walk.NewComposite(svr.scrollview)

	var pb1 *walk.ToolButton

	//Declarative style
	ft := Font{Family: parent.Font().Family(), PointSize: 10, Bold: false}
	bldr := NewBuilder(svr.scrollview)

	err = (Composite{
		AssignTo: &svr.optionsPanel,
		Layout:   HBox{Margins: Margins{Top: 2, Left: 1, Right: 1, Bottom: 0}, MarginsZero: false},
		Font:     ft,
		Children: []Widget{
			Composite{
				Layout: Grid{Columns: 8, Margins: Margins{Top: 1, Left: 1, Right: 1, Bottom: 0}, MarginsZero: false},
				Children: []Widget{
					Composite{
						Column: 0,
						Layout: Grid{Columns: 2, SpacingZero: true, MarginsZero: true},
						OnSizeChanged: func() {
							pb1.SetWidth(100)
						},
						Children: []Widget{
							ComboBox{
								AssignTo: &svr.cmbSort,
								Editable: false,
								Model: []string{
									" Sort by Name",
									" Sort by Size",
									" Sort by Date",
									" Sort by Width",
									" Sort by Height",
								},
								OnCurrentIndexChanged: func() {
									svr.setSortMode2(svr.cmbSort.CurrentIndex(), svr.cmbSort.Format() == "", -1)
								},
							},
							ToolButton{
								AssignTo:    &pb1,
								Text:        ".|'",
								ToolTipText: "Toggle sort ascending/descending",
								OnClicked: func() {
									svr.setSortMode2(svr.cmbSort.CurrentIndex(), true, -1)
								},
							},
						},
					},
					Composite{
						Layout: VBox{Margins: Margins{Top: 1, Left: 1, Right: 1, Bottom: 1}, MarginsZero: false},
						Children: []Widget{
							Label{
								Text: "Layout: ",
							},
							VSpacer{
								Size: 10,
							},
						}},
					ComboBox{
						AssignTo: &svr.cmbMode,
						Editable: false,
						Model:    sModel,
						OnCurrentIndexChanged: svr.setLayoutMode,
					},
					HSpacer{},
					Composite{
						Layout: VBox{Margins: Margins{Top: 1, Left: 1, Right: 1, Bottom: 1}, MarginsZero: false},
						Children: []Widget{
							Label{
								AssignTo: &svr.lblSize,
								Text:     "Size: 120 x 75 px",
							},
							VSpacer{
								Size: 10,
							},
						},
					},
					Slider{
						AssignTo:       &svr.sldrSize,
						MaxValue:       300,
						MinValue:       64,
						MinSize:        Size{100, 0},
						MaxSize:        Size{300, 0},
						OnValueChanged: svr.setItemSize,
					},
					HSpacer{Size: 10},
					Composite{
						Layout: VBox{Margins: Margins{Top: 1, Left: 1, Right: 1, Bottom: 0}, MarginsZero: false},
						Children: []Widget{
							CheckBox{
								AssignTo:         &svr.cbCached,
								Text:             "Cached",
								ColumnSpan:       1,
								OnCheckedChanged: svr.setCacheMode,
							},
							VSpacer{
								Size: 2,
							},
						},
					},
				},
			},
		},
	}.Create(bldr))

	svr.canvasView.MouseDown().Attach(svr.OnMouseDown)
	svr.canvasView.MouseMove().Attach(svr.OnMouseMove)
	svr.canvasView.MouseUp().Attach(svr.OnMouseUp)

	svr.scrollview.KeyPress().Attach(svr.OnKeyPress)
	svr.scrollview.SizeChanged().Attach(svr.onSizeChanged)

	parent.SizeChanged().Attach(svr.onSizeParentChanged)

	br, _ := walk.NewSolidColorBrush(walk.RGB(20, 20, 20))
	svr.canvasView.SetBackground(br)

	svr.onSizeParentChanged()

	svr.resizing()

	if !viewerMode {
		svr.SetLayoutMode(7) //album mode
		svr.scrollview.SetBackground(br)
		log.Println("drawInfocardAlbum")
	} else {
		svr.SetLayoutMode(0)
	}

	svr.ID = svr.scrollview.Handle()
	return svr, err
}

func (sv *ScrollViewer) closeDrawers() {
	if sv.drawersActive && sv.drawersChan != nil {
		sv.drawersWait.Add(sv.drawersCount)

		for i := 0; i < sv.drawersCount; i++ {
			sv.drawersChan <- nil
		}
		sv.drawersWait.Wait()
	}
}
func (sv *ScrollViewer) destroy() error {

	defer func() {
		if err := recover(); err != nil { //catch
			log.Println("recover")
			err = nil
		}
	}()

	sv.closeDrawers()
	sv.directoryMonitor.Close()
	sv.imageProcessor.Close(sv)
	sv.scrollview.SetVisible(false)

	p := sv.scrollview.Parent().AsContainerBase().Children()
	i := p.Index(sv.scrollview)

	err := p.RemoveAt(i)
	if err != nil {
		log.Println("error removing item")
		//log.Fatal(err)
		//err = nil
	}

	sv.scrollview.Dispose()
	//log.Println("resume after error removing item")

	if sv.drawerBuffer != nil {
		DeleteDrawBuffer(sv.drawerBuffer)
	}

	if sv.previewBackground != nil {
		sv.previewBackground.Dispose()
	}

	for k, _ := range sv.ItemsMap {
		delete(sv.ItemsMap, k)
	}
	return err
}

func (sv *ScrollViewer) Run(dirPath string, itemsModel *FileInfoModel, watchThisPath bool) {

	if itemsModel == nil {
		sv.itemsModel.BrowsePath(dirPath, true)

		log.Println("internal sv.itemsModel.items")
	} else {
		sv.itemsModel = itemsModel
		log.Println("external sv.itemsModel.items")
	}

	if len(sv.itemsModel.items) == 0 {
		log.Println("ScrollViewer.Run exit, no items in itemsModel")
		return
	}
	log.Println("ScrollViewer.Run1 sv.cmbSort.CurrentIndex(), sv.currentSortIndex, sv.itemsModel.SortedColumn()", sv.cmbSort.CurrentIndex(), sv.currentSortIndex, sv.itemsModel.SortedColumn())

	// resort data if necessary
	// needed for the first run
	if sv.cmbSort.CurrentIndex() != sv.currentSortIndex {
		//if sv.itemsModel.SortedColumn() != sv.currentSortIndex {
		log.Println("ScrollViewer.Run resorting", sv.cmbSort.CurrentIndex(), sv.itemsModel.SortedColumn())
		sv.setSortMode2(sv.cmbSort.CurrentIndex(), true, sv.currentSortOrder)
	}
	log.Println("ScrollViewer.Run2 sv.cmbSort.CurrentIndex(), sv.currentSortIndex, sv.itemsModel.SortedColumn()", sv.cmbSort.CurrentIndex(), sv.currentSortIndex, sv.itemsModel.SortedColumn())

	//--------------------------------------
	//create map containing the file infos
	//--------------------------------------
	for i, vlist := range sv.itemsModel.items {
		fn := filepath.Join(sv.itemsModel.dirPath, vlist.Name)

		if vmap, ok := sv.ItemsMap[fn]; !ok {
			sv.ItemsMap[fn] = vlist
			sv.ItemsMap[fn].index = i
		} else {
			//copy different values from the new list item
			vmap.index = i
			vmap.Changed = (vlist.Size != vmap.Size) || (vlist.Modified != vmap.Modified)
			vmap.Size = vlist.Size
			vmap.Modified = vlist.Modified
			vmap.Width = vlist.Width
			vmap.Height = vlist.Height

			//ACHTUNG! MUCHO IMPORTANTE!
			//assign vmap data back to the list
			//to maintain synch.
			//ie. to point to the same *fileinfo
			//or else...
			sv.itemsModel.items[i] = vmap
		}
	}

	if sv.contentMonitor != nil {
		sv.contentMonitor.removeChangedItems(sv.contentMonitor.doneMap)
	}

	//Updating to reflect the num of items
	sv.SetItemsCount(len(sv.itemsModel.items))

	//initialize cache database
	if CacheDB == nil {
		sv.OpenCacheDB("")
	}
	//--------------------------------
	//run the imageProcessor workers
	//--------------------------------
	sv.imageProcessor.Run(sv, sv.itemsModel.items, []string{dirPath})

	if watchThisPath && sv.itemsCount > 0 {
		sv.directoryMonitor.setFolderWatcher(dirPath)
	}
}

func (sv *ScrollViewer) RunAlbum() {
	//initialize album database
	if AlbumDB == nil {
		OpenAlbumDB("")
	}

	// reset items
	sv.itemsModel.items = []*FileInfo{}
	sv.AlbumDBEnum("")

	if len(sv.itemsModel.items) == 0 {
		sv.SetItemsCount(0)
		sv.Invalidate()
		log.Println("ScrollViewer.Run exit, no items in itemsModel")
		return
	}
	sv.selections = []*FileInfo{}
	lastSelectedID := -1
	if sv.SelectedItem() != nil {
		lastSelectedID = sv.SelectedItem().index
	}
	//--------------------------------------
	//create map containing the file infos
	//--------------------------------------
	for k, _ := range sv.ItemsMap {
		delete(sv.ItemsMap, k)
	}
	for i, v := range sv.itemsModel.items {
		fn := filepath.Join(sv.itemsModel.dirPath, v.Name)

		sv.ItemsMap[fn] = v
		if v.index == lastSelectedID {
			v.checked = true
			sv.SelectedIndex = i
			sv.selections = append(sv.selections, v)
		}
	}

	//Updating to reflect the num of items
	sv.SetItemsCount(len(sv.itemsModel.items))
	sv.Invalidate()
}
func (sv *ScrollViewer) RunAlbumItems(items []*FileInfo) {

	sv.itemsModel.items = nil
	sv.itemsModel.items = items

	log.Println("DB sv.itemsModel.items")

	if len(sv.itemsModel.items) == 0 {
		sv.SetItemsCount(0)
		sv.Invalidate()
		log.Println("ScrollViewer.Run exit, no items in itemsModel")
		return
	}
	// resort data if necessary
	// needed for the first run
	//	if sv.cmbSort.CurrentIndex() != sv.currentSortIndex {
	//		sv.setSortMode2(sv.cmbSort.CurrentIndex(), true, sv.currentSortOrder)

	//		log.Println("ScrollViewer.Run resorting", sv.cmbSort.CurrentIndex(), sv.currentSortIndex)
	//	}

	//--------------------------------------
	//create map containing the file infos
	//--------------------------------------
	//	for i, vlist := range sv.itemsModel.items {
	//		fn := filepath.Join(vlist.Info, vlist.Name)

	//		if vmap, ok := sv.ItemsMap[fn]; !ok {
	//			sv.ItemsMap[fn] = vlist
	//			sv.ItemsMap[fn].index = i
	//		} else {
	//			//copy different values from the new list item
	//			vmap.index = i
	//			vmap.Changed = (vlist.Size != vmap.Size) || (vlist.Modified != vmap.Modified)
	//			vmap.Size = vlist.Size
	//			vmap.Modified = vlist.Modified
	//			vmap.Width = vlist.Width
	//			vmap.Height = vlist.Height

	//			//ACHTUNG! MUCHO IMPORTANTE!
	//			//assign vmap data back to the list
	//			//to maintain synch.
	//			//ie. to point to the same *fileinfo
	//			//or else...
	//			sv.itemsModel.items[i] = vmap
	//		}
	//	}
	for k, _ := range sv.ItemsMap {
		delete(sv.ItemsMap, k)
	}
	var srcPaths []string
	lastpath := ""
	for _, v := range sv.itemsModel.items {
		fn := filepath.Join(v.Info, v.Name)

		sv.ItemsMap[fn] = v

		if lastpath != v.Info {
			srcPaths = append(srcPaths, v.Info)
			lastpath = v.Info
		}
	}

	if sv.contentMonitor != nil {
		sv.contentMonitor.removeChangedItems(sv.contentMonitor.doneMap)
	}

	//Updating to reflect the num of items
	sv.SetItemsCount(len(sv.itemsModel.items))

	//initialize cache database
	if CacheDB == nil {
		sv.OpenCacheDB("")
	}
	//--------------------------------
	//run the imageProcessor workers
	//--------------------------------
	sv.imageProcessor.Run(sv, sv.itemsModel.items, srcPaths)

	//	if watchThisPath && sv.itemsCount > 0 {
	//		sv.directoryMonitor.setFolderWatcher(dirPath)
	//	}
}

func (sv *ScrollViewer) oncanvasViewpaint(canvas *walk.Canvas, updaterect walk.Rectangle) error {
	return nil
}

func (sv *ScrollViewer) OnKeyPress(key walk.Key) {

	switch key {
	case walk.KeyReturn:
		sv.ShowPreviewFull()
	case walk.KeySpace:
		sv.ShowPreview()
	case walk.KeyEscape:
		sv.Repaint()
	case walk.KeyLeft:
		sv.SetItemSelected(sv.SelectedIndex-1, true)

	case walk.KeyRight:
		sv.SetItemSelected(sv.SelectedIndex+1, true)
		sv.Repaint()
	case walk.KeyUp:
		sv.setScrollPosBy(-sv.itemHeight / 4)
	case walk.KeyDown:
		sv.setScrollPosBy(sv.itemHeight / 4)
	case walk.KeyF5:
		if sv.ViewerMode {
			sv.Run(sv.itemsModel.dirPath, sv.itemsModel, true)
		} else {
			sv.RunAlbum()
		}
	}
}
func (sv *ScrollViewer) OnMouseDown(x, y int, button walk.MouseButton) {

	//mouseup does not give this
	sv.lastButtonDown = button

	//perform selection
	idx := sv.GetItemAtScreen(x, y)

	if !sv.isMultipleSelection() {
		sv.SetItemSelected(idx, false)
	} else {
		if button == walk.LeftButton {
			sv.SetItemSelected(idx, false)
		}
	}

	//transfer to a function callback if exists
	if sv.evMouseDown != nil {
		sv.evMouseDown(x, y, button)
	}
	//skip everything if preview is active
	//and mouse x,y is in PreviewRect
	if sv.PreviewRect != nil {
		r := sv.PreviewRect
		bounds := image.Rect(r.X, r.Y, r.X+r.Width, r.Y+r.Height)
		pt := image.Point{x, y}
		if pt.In(bounds) {
			return
		}
	}
	//initialize mouse vars
	//this for mousemove scrolling
	if button == walk.LeftButton {
		sv.viewInfo.mouseposY = y
		sv.viewInfo.scrollpos = sv.viewInfo.topPos
	}

	sv.scrollview.SetFocus()
	sv.Repaint()
}
func (sv *ScrollViewer) OnMouseMove(x, y int, button walk.MouseButton) {
	//perform mouse move
	if button == walk.LeftButton && sv.PreviewRect == nil {
		sv.viewInfo.mousemoveY = sv.viewInfo.mouseposY - y

		val := sv.viewInfo.scrollpos + sv.viewInfo.mousemoveY
		sv.SetScroll(val)
	} else {
		prt := sv.scrollview.Parent().AsContainerBase()
		num := prt.Children().Len()

		hwnd := GetForegroundWindow()
		if hwnd == sv.MainWindow.Handle() && num > 0 && !sv.scrollview.Focused() {
			sv.SetFocus()
		}
	}
}
func (sv *ScrollViewer) OnMouseUp(x, y int, button walk.MouseButton) {
	//do not continue if there is
	//already a preview on screen
	if sv.PreviewRect != nil {
		return
	}

	//reset movement vars
	sv.viewInfo.mousemoveY = 0
	sv.viewInfo.mouseposY = 0
	sv.viewInfo.scrollpos = 0

	if sv.ViewerMode {
		//Display image preview
		if sv.lastButtonDown == walk.RightButton && !sv.suspendPreview {
			sv.PreviewItemAtScreen(x, y)
		}
	}
	//double click to launch preview
	if sv.dblClickTime.IsZero() {
		sv.dblClickTime = time.Now()
	} else {
		d := time.Since(sv.dblClickTime)
		if d.Seconds() < 0.400 {
			sv.OnDoubleClick()
		}
		sv.dblClickTime = time.Now()
	}

}
func (sv *ScrollViewer) OnDoubleClick() {
	if sv.ViewerMode {
		sv.ShowPreviewFull()
	} else {
		if sv.OnAlbumEditing != nil {
			v := sv.SelectedItem()
			if v != nil {
				sv.OnAlbumEditing(v.index, v.Name, v.Info)
			}
		}
	}
}

func (sv *ScrollViewer) onSizeParentChanged() {
	//manage scrollviewer objects placement
	//distribute scrollviewer objects vertically
	//Usefull when more than one scrollview are
	//sharing a single parent.
	if sv.scrollview.Parent() == nil {
		return
	}
	if sv.scrollview.Layout() != nil {
		return
	}

	p := sv.scrollview.Parent().AsContainerBase()
	b := p.ClientBounds()
	n := p.Children().Len()
	h := b.Height / n

	if p.Layout() != nil {
		m := p.Layout().Margins()
		b.X += m.HNear
		b.Width -= m.HFar
	}
	for i := 0; i < n; i++ {
		ch := p.Children().At(i)
		ch.SetBounds(walk.Rectangle{b.X, b.Y + i*(h+2), b.Width - b.X, h})
	}
}
func (sv *ScrollViewer) onSizeChanged() {
	sv.resizing()
}
func (sv *ScrollViewer) resizing() {
	if sv.scrollview.Layout() != nil {
		return
	}
	rs := int(win.GetSystemMetrics(win.SM_CXVSCROLL))

	if sv.ViewerMode {
		sv.optionsPanel.SetBounds(walk.Rectangle{0, 0, sv.Width() - rs - 1, 30})

		sv.canvasView.SetBounds(walk.Rectangle{0, 30, sv.Width() - rs - 1, sv.Height() - 30})
	} else {
		sv.canvasView.SetBounds(walk.Rectangle{0, 0, sv.Width() - rs - 1, sv.Height()})
	}
	sv.recalc()
}

//var resCount int

//func doResizing(sv *ScrollViewer) {
//	resCount++
//	if !sv.isResizing {
//		sv.isResizing = true
//		go func() {
//			t := time.NewTicker(time.Millisecond * 100)
//		loop:
//			for {
//				if resCount > 0 {
//					resCount--
//					time.Sleep(time.Millisecond * 10)
//				} else {
//					select {
//					case <-t.C:
//						if resCount <= 0 {
//							break loop
//						}
//					}
//				}
//			}
//			sv.isResizing = false
//			t.Stop()
//			resCount = 0
//			sv.scrollview.Synchronize(func() {
//				if sv.SelectedIndex == -1 {
//					sv.Invalidate()
//				} else {
//					sv.setSelectionVisible()
//				}
//			})
//		}()
//	}
//}

func (sv *ScrollViewer) Bounds() walk.Rectangle {
	return sv.scrollview.Bounds()
}
func (sv *ScrollViewer) SetBounds(b walk.Rectangle) {
	sv.scrollview.SetBounds(b)
}
func (sv *ScrollViewer) SetEnabled(b bool) {
	sv.canvasView.SetEnabled(b)
}
func (sv *ScrollViewer) ViewBounds() walk.Rectangle {
	return sv.canvasView.ClientBounds()
}

func (sv *ScrollViewer) setLayoutMode() {
	//0"Frameless, variable size"
	//1"Grid with name, date, and size",
	//2"Grid with name and date",
	//3"Grid with name and size",
	//4"Grid with name only",
	//5"Grid with no text",
	//6"Infocard",
	//7"Infocard Album",
	sv.currentLayout = sv.cmbMode.CurrentIndex()

	switch sv.cmbMode.CurrentIndex() {
	case 0:
		sv.itemSize.mx = 0
		sv.itemSize.my = 0
		sv.itemSize.txth = 0
		sv.evPaint = drawContinuos
		sv.viewInfo.showName = false
		sv.viewInfo.showDate = false
		sv.viewInfo.showInfo = false
	case 1:
		sv.itemSize.mx = 10
		sv.itemSize.my = 10
		sv.evPaint = drawGrid
		sv.viewInfo.showName = true
		sv.viewInfo.showDate = true
		sv.viewInfo.showInfo = true
		sv.itemSize.txth = 3 * 16
	case 2:
		sv.itemSize.mx = 10
		sv.itemSize.my = 10
		sv.evPaint = drawGrid
		sv.viewInfo.showName = true
		sv.viewInfo.showDate = true
		sv.viewInfo.showInfo = false
		sv.itemSize.txth = 2 * 17
	case 3:
		sv.itemSize.mx = 10
		sv.itemSize.my = 10
		sv.evPaint = drawGrid
		sv.viewInfo.showName = true
		sv.viewInfo.showDate = false
		sv.viewInfo.showInfo = true
		sv.itemSize.txth = 2 * 17
	case 4:
		sv.itemSize.mx = 10
		sv.itemSize.my = 10
		sv.evPaint = drawGrid
		sv.viewInfo.showName = true
		sv.viewInfo.showDate = false
		sv.viewInfo.showInfo = false
		sv.itemSize.txth = 1 * 20
	case 5:
		sv.itemSize.mx = 10
		sv.itemSize.my = 10
		sv.evPaint = drawGrid
		sv.viewInfo.showName = false
		sv.viewInfo.showDate = false
		sv.viewInfo.showInfo = false
		sv.itemSize.txth = 0
	case 6:
		sv.itemSize.mx = 6
		sv.itemSize.my = 6
		sv.evPaint = drawInfocard
		sv.viewInfo.showName = true
		sv.viewInfo.showDate = true
		sv.viewInfo.showInfo = true
		sv.itemSize.txth = 0
	case 7:
		sv.itemSize.mx = 6
		sv.itemSize.my = 6
		sv.evPaint = drawInfocardAlbum
		sv.viewInfo.showName = true
		sv.viewInfo.showDate = true
		sv.viewInfo.showInfo = true
		sv.itemSize.txth = 0
	}
	sv.itemWidth = sv.itemSize.twm()
	sv.itemHeight = sv.itemSize.thm()
	sv.recalc()

	sv.Invalidate()
	defer sv.SetFocus()
}
func (sv *ScrollViewer) SetLayoutMode(idx int) {
	switch idx {
	case 0:
		sv.evPaint = drawContinuos
		sv.cmbMode.SetCurrentIndex(idx)
	case 1, 2, 3, 4, 5:
		sv.evPaint = drawGrid
		sv.cmbMode.SetCurrentIndex(idx)
	case 6:
		sv.evPaint = drawInfocard
		sv.cmbMode.SetCurrentIndex(idx)
	case 7:
		sv.evPaint = drawInfocardAlbum
		sv.cmbMode.SetCurrentIndex(idx)
	}
}
func (sv *ScrollViewer) GetLayoutMode() int {
	return sv.currentLayout
}
func (sv *ScrollViewer) GetSortMode() int {
	return sv.currentSortIndex
}
func (sv *ScrollViewer) GetSortOrder() int {
	return sv.currentSortOrder
}
func (sv *ScrollViewer) SetSortMode(idx, order int) bool {
	sv.cmbSort.SetFormat("-")
	sv.cmbSort.SetCurrentIndex(idx) // this will call setSortMode2
	sv.currentSortOrder = order

	return true
}

func (sv *ScrollViewer) Less(i, j int) bool {
	d := sv.itemsModel.items

	if sv.itemsModel.SortOrder() == walk.SortAscending {
		switch sv.itemsModel.SortedColumn() {
		case 0:
			return d[i].Name < d[j].Name
		case 1:
			return d[i].Size < d[j].Size
		case 2:
			return d[i].Modified.Unix() < d[j].Modified.Unix()
		case 3:
			return d[i].Width < d[j].Width
		case 4:
			return d[i].Height < d[j].Height
		}
	} else {
		switch sv.itemsModel.SortedColumn() {
		case 0:
			return d[i].Name > d[j].Name
		case 1:
			return d[i].Size > d[j].Size
		case 2:
			return d[i].Modified.Unix() > d[j].Modified.Unix()
		case 3:
			return d[i].Width > d[j].Width
		case 4:
			return d[i].Height > d[j].Height
		}
	}
	return false
}
func (sv *ScrollViewer) doSort() {

	sort.SliceStable(sv.itemsModel.items, sv.Less)

}
func (sv *ScrollViewer) setSortMode2(idx int, doAction bool, sortOrder int) {
	log.Println("setSortMode2 idx:", idx, "doaction:", doAction, "sortOrder:", sortOrder)
	//
	flipsort := func(index int, order int) {
		ev := *sv.itemsModel.SortChanged()
		s := fmt.Sprint(ev)
		if s == "{[]}" {
			sv.itemsModel.SortChanged().Attach(sv.doSort)
		}

		if order == -1 {
			if sv.itemsModel.SortedColumn() == index {
				if sv.itemsModel.SortOrder() == walk.SortAscending {
					sv.itemsModel.Sort(index, walk.SortDescending)
				} else {
					sv.itemsModel.Sort(index, walk.SortAscending)
				}
			} else {
				sv.itemsModel.Sort(index, walk.SortAscending)
			}
		} else {
			sv.itemsModel.Sort(index, walk.SortOrder(order))
		}
	}
	if doAction {
		switch idx {
		case 0:
			flipsort(0, sortOrder)
		case 1:
			flipsort(1, sortOrder)
		case 2:
			flipsort(2, sortOrder)
		case 3:
			flipsort(3, sortOrder)
		case 4:
			flipsort(4, sortOrder)
		}

		sv.Invalidate()
		sv.currentSortIndex = sv.cmbSort.CurrentIndex()
		sv.currentSortOrder = int(sv.itemsModel.SortOrder())
	}
	sv.cmbSort.SetFormat("")
}
func (sv *ScrollViewer) GetItemName(idx int) (res string) {

	if sv.isValidIndex(idx) {
		res = sv.itemsModel.items[idx].Name
	}
	return res
}
func (sv *ScrollViewer) GetItemInfo(idx int) (res string) {

	if sv.isValidIndex(idx) {
		v := sv.itemsModel.items[idx]
		info1 := fmt.Sprintf("%d x %d px, %d KB", v.Width, v.Height, v.Size/1024)
		info2 := v.Modified.Format("Jan 2, 2006 3:04pm")
		res = info1 + "   " + info2
	}
	return res
}
func (sv *ScrollViewer) GetItemAtScreen(x int, y int) (idx int) {

	switch sv.GetLayoutMode() {
	case 0:
		idx = sv.getItemAtScreenNB2(x, y)
	case 1, 2, 3, 4, 5:
		col := x / sv.itemWidth
		idx = -1

		if col < sv.NumCols() {
			row := int(float32(y+sv.viewInfo.topPos) / float32(sv.itemHeight))
			idx = col + row*sv.NumCols()
			if idx >= sv.itemsCount {
				idx = -1
			}
		}
	case 6, 7:
		w := sv.itemWidth + 150
		col := x / w
		idx = -1
		cols := sv.canvasView.Width() / w

		if col < cols {
			row := (y + sv.viewInfo.topPos) / sv.itemHeight
			idx = col + row*cols
			if idx >= sv.itemsCount {
				idx = -1
			}
		}
	}
	return idx
}

func (sv *ScrollViewer) getItemRectAtScreen(xs, ys int) *image.Rectangle {
	switch sv.GetLayoutMode() {
	case 1, 2, 3, 4, 5:
		w := sv.itemSize.twm()
		h := sv.itemSize.thm()

		col := int(float32(xs) / float32(w))
		row := int(float32(ys+sv.viewInfo.topPos) / float32(h))
		x1 := col * w
		y1 := row * h
		r := image.Rect(x1, y1+h-sv.itemSize.txth, x1+w, y1+h)
		return &r
	case 6, 7:
		w := sv.itemSize.twm() + 150
		h := sv.itemSize.thm()

		col := int(float32(xs) / float32(w))
		row := int(float32(ys+sv.viewInfo.topPos) / float32(h))
		x1 := col * w
		y1 := row * h
		r := image.Rect(x1, y1+h-sv.itemSize.txth, x1+w, y1+h)
		return &r
	case 0:
		return sv.getItemRectAtScreenNB2(xs, ys)
	}
	return nil
}
func (sv *ScrollViewer) getItemAtScreenNB2(xs, ys int) int {
	x := 0
	y := 0
	wmax := sv.ViewWidth()
	tw := sv.itemSize.tw
	th := sv.itemSize.th
	h := th

	for i, val := range sv.itemsModel.items {

		wd, _ := getOptimalThumbSize(tw, th, val.Width, val.Height)

		if (x + wd) >= wmax {
			if wmax-x > 6*wd/10 {
				wd = wmax - x
			} else {
				x = 0
				y += h
			}
		}

		rItm := image.Rect(x, y, x+wd, y+h)
		pt := image.Point{xs, ys + sv.viewInfo.topPos}

		if pt.In(rItm) {
			return i
		}

		x += wd
	}
	return -1
}
func (sv *ScrollViewer) getItemRectAtScreenNB2(xs, ys int) *image.Rectangle {

	wmax := sv.ViewWidth()
	tw := sv.itemSize.tw
	th := sv.itemSize.th
	h := th
	x := 0
	y := 0
	for _, val := range sv.itemsModel.items {

		wd, _ := getOptimalThumbSize(tw, th, val.Width, val.Height)

		if (x + wd) >= wmax {
			if wmax-x > 6*wd/10 {
				wd = wmax - x
			} else {
				x = 0
				y += h
			}
		}

		rItm := image.Rect(x, y, x+wd, y+h)
		pt := image.Point{xs, ys + sv.viewInfo.topPos}

		if pt.In(rItm) {
			return &rItm
		}
		x += wd
	}
	return nil
}
func (sv *ScrollViewer) getTotalHeightNB2() int {

	x := 0
	y := 0
	wmax := sv.ViewWidth()
	tw := sv.itemSize.tw
	th := sv.itemSize.th
	h := th
	for _, val := range sv.itemsModel.items {

		wd, _ := getOptimalThumbSize(tw, th, val.Width, val.Height)
		if (x + wd) > wmax {
			if wmax-x > 6*wd/10 {
				wd = wmax - x
			} else {
				x = 0
				y += h
			}
		}
		x += wd
	}
	return y + h
}

func (sv *ScrollViewer) isValidIndex(idx int) bool {
	return idx >= 0 && idx < sv.itemsCount
}
func (sv *ScrollViewer) isMultipleSelection() bool {
	return len(sv.selections) > 1
}
func (sv *ScrollViewer) PreviewItemAtScreen(x int, y int) bool {
	/*---------------------------------
	actual drawing code us in fb_image.go
	-----------------------------------*/
	idx := sv.GetItemAtScreen(x, y)

	if sv.isValidIndex(idx) {
		sv.PreviewRect = DrawPreview(sv, idx)
	}
	return true
}
func (sv *ScrollViewer) ShowPreview() bool {
	/*---------------------------------
	actual drawing code us in fb_image.go
	-----------------------------------*/
	if sv.isValidIndex(sv.SelectedIndex) {
		sv.PreviewRect = DrawPreview(sv, sv.SelectedIndex)
		return true
	}
	return false
}
func (sv *ScrollViewer) ShowPreviewFull() bool {

	if sv.SelectedNameFull() != "" {
		NewImageViewWindow(sv.MainWindow, sv.SelectedNameFull(), sv.itemsModel, sv.synchWithPreview)
	}

	return true
}
func (sv *ScrollViewer) synchWithPreview(idx int) {

	if sv.isValidIndex(idx) {
		sv.SetItemSelected(idx, true)

	}
}
func (sv *ScrollViewer) recalc() int {
	hMax := sv.ViewHeight()
	switch sv.GetLayoutMode() {
	case 0:
		hMax = sv.getTotalHeightNB2()
	case 1, 2, 3, 4, 5:
		hMax = sv.NumRows() * sv.itemHeight
	case 6, 7:
		cols := math.Trunc(float64(sv.canvasView.Width()) / float64(sv.itemWidth+150))
		hMax = sv.itemHeight * int(math.Ceil(float64(sv.itemsCount)/cols))
	}

	sv.scrollview.updateScrollbar(hMax-sv.ViewHeight(), 2*sv.itemHeight, sv.itemHeight, 10)

	sv.viewInfo.topPos = sv.scrollview.Value()
	sv.viewInfo.numCols = sv.NumCols()
	sv.viewInfo.viewRows = sv.NumRowsVisible()

	r := image.Rect(0, sv.viewInfo.topPos, sv.ViewWidth(), sv.viewInfo.topPos+sv.ViewHeight())
	sv.viewInfo.viewRect = r

	//log.Println("recalcSize ItemCount,ItemWidth,ItemHeight", sv.items, sv.itemWidth, sv.itemHeight)
	//	log.Println("recalcSize h,NumRows,NumCols", h, sv.NumRows(), sv.NumCols())
	//	log.Println("recalcSize", sv.scrollview.AsContainerBase().Bounds(), sv.canvasView.Bounds())
	return hMax
}

func (sv *ScrollViewer) SetProcessStatuswidget(sw *walk.StatusBar) {
	sv.imageProcessor.setstatuswidget(sw)
	sv.contentMonitor.setstatuswidget(sw)
}
func (sv *ScrollViewer) SetImageProcessorStatusFunc(ipf func(i int)) {
	sv.imageProcessor.statusfunc = ipf
}
func (sv *ScrollViewer) SetImageProcessorInfoFunc(ipf func(numjob int, d float64)) {
	sv.imageProcessor.infofunc = ipf
}
func (sv *ScrollViewer) SetDirectoryMonitorInfoFunc(ipf func(dirpath string)) {
	sv.directoryMonitor.infofunc = ipf
}

func (sv *ScrollViewer) SetEventMouseDown(evt walk.MouseEventHandler) {
	sv.evMouseDown = evt
}
func (sv *ScrollViewer) SetEventPaint(eventproc walk.PaintFunc) {
	//sv.evPaint = eventproc
}
func (sv *ScrollViewer) SetEventSizeChanged(eventproc walk.EventHandler) {
	sv.scrollview.SizeChanged().Attach(eventproc)
}
func (sv *ScrollViewer) SetFocus() {
	sv.scrollview.SetFocus()
}
func (sv *ScrollViewer) SelectedItem() *FileInfo {
	if sv.SelectedIndex >= 0 {
		return sv.itemsModel.items[sv.SelectedIndex]
	} else {
		return nil
	}
}
func (sv *ScrollViewer) SelectedName() string {
	if sv.SelectedIndex >= 0 {
		return sv.itemsModel.items[sv.SelectedIndex].Name
	} else {
		return ""
	}
}
func (sv *ScrollViewer) SelectedNameFull() string {
	if sv.SelectedIndex >= 0 {
		return sv.itemsModel.getFullPath(sv.SelectedIndex)
	} else {
		return ""
	}
}
func (sv *ScrollViewer) SetContextMenu(menu *walk.Menu) {
	sv.canvasView.SetContextMenu(menu)
}
func (sv *ScrollViewer) SetItemsCount(count int) {
	if sv.itemsCount != count {
		sv.itemsCount = count
		sv.recalc()
	}

	if count == 0 {
		sv.SetScroll(0)
		sv.Invalidate()

		//close folder watcher
		//sv.directoryMonitor.Close()
		if sv.directoryMonitor != nil {
			sv.directoryMonitor.setFolderWatcher("")
		}
	}
}

func (sv *ScrollViewer) setCacheMode() {

	sv.doCache = sv.cbCached.Checked()
}
func (sv *ScrollViewer) SetCacheMode(active bool) {

	sv.cbCached.SetChecked(active)
}
func (sv *ScrollViewer) setItemSize() {
	//called by slider control

	tw := sv.sldrSize.Value()
	th := int(math.Ceil(float64(tw) * float64(10) / float64(16)))

	sv.lblSize.SetText("Size: " + strconv.Itoa(tw) + "x" + strconv.Itoa(th))

	sv.SetItemSize(tw, th)
	//sv.ShowOptions(false)
	sv.SetFocus()
}
func (sv *ScrollViewer) SetItemSize(w, h int) {
	if sv.itemSize.tw != w || sv.itemSize.th != h {

		sv.itemSize.tw = w
		sv.itemSize.th = h

		sv.itemWidth = sv.itemSize.twm()
		sv.itemHeight = sv.itemSize.thm()

		if sv.contentMonitor != nil {
			sv.contentMonitor.removeChangedItems(sv.contentMonitor.doneMap)
		}
	}
	sv.recalc()
	sv.Invalidate()

	if sv.sldrSize.Value() != w {
		sv.sldrSize.SetValue(w)
	}
}
func (sv *ScrollViewer) exitPreviewMode() bool {
	if sv.PreviewRect != nil {
		r := sv.PreviewRect

		if sv.previewBackground != nil {
			cvs, _ := sv.canvasView.CreateCanvas()
			defer cvs.Dispose()

			cvs.DrawImage(sv.previewBackground, walk.Point{r.X, r.Y})
			sv.previewBackground.Dispose()
			sv.previewBackground = nil
		}
		sv.PreviewRect = nil
		return true
	}
	return false
}

func (sv *ScrollViewer) SetScrollPos(val int) {
	//	sv.scrollview.SetValue(val)
	//	sv.Invalidate()

	sv.SetScroll(val)
}
func (sv *ScrollViewer) setScrollPosBy(val int) int {
	sv.scrollview.Synchronize(func() {
		sv.SetScroll(sv.scrollview.Value() + val)
	})

	return sv.scrollview.Value()
}
func (sv *ScrollViewer) SetScroll(val int) {
	//	if val < 0 {
	//		val = 0
	//	}

	if val == sv.viewInfo.topPos {
		return
	}

	var pos int
	if sv.scrollview.Value() != val {
		pos = sv.scrollview.SetValue(val)

		if val != pos {
			val = pos
		}
	}
	// calculate the source rect
	rSrc := image.Rect(0, val, sv.ViewWidth(), val+sv.ViewHeight())

	iscrollSize := abs(val - sv.viewInfo.topPos)

	if iscrollSize > 2*sv.itemHeight {
		//		log.Println("SetScroll: jump scroll", iscrollSize, val, sv.viewInfo.topPos)
		iscrollSize = sv.NumRowsVisible() * sv.itemHeight
	}

	cvs, _ := sv.canvasView.CreateCanvas()
	defer cvs.Dispose()

	//detect scroll direction
	//setup update rect, reflecting the exposed scroll area

	y := 0
	// ScrollDown:
	if val > sv.viewInfo.topPos {
		y = rSrc.Max.Y - iscrollSize
	} else if val < sv.viewInfo.topPos {
		y = rSrc.Min.Y
	}
	sv.viewInfo.topPos = val
	sv.viewInfo.numCols = sv.NumCols()
	sv.viewInfo.viewRows = sv.NumRowsVisible()
	sv.viewInfo.viewRect = rSrc

	// calculate the target rect
	rDst := walk.Rectangle{0, y, sv.ViewWidth(), iscrollSize}

	//set scrolling flag for the onpaint
	sv.viewInfo.scrolling = true

	//trigger onpaint to handle the rest of the drawing
	sv.onPaint(cvs, rDst)

	//switch back the flag
	sv.viewInfo.scrolling = false
}
func (sv *ScrollViewer) selectionChanged(newindex int, oldindex int) {
	if sv.OnSelectionChanged != nil {
		sv.OnSelectionChanged()
	}
}
func (sv *ScrollViewer) SetItemSelected(index int, redraw bool) {

	checkItem := func(item *FileInfo) (int, bool) {
		// check if item is already in the list
		for i, v := range sv.selections {
			if v == item {
				return i, true
			}
		}
		return -1, false
	}
	clearSelections := func() {
		for _, v := range sv.itemsModel.items {
			v.checked = false
		}
	}

	oldindex := sv.SelectedIndex
	sv.SelectedIndex = index

	if !sv.isValidIndex(index) {
		clearSelections()
		sv.Repaint()
		return
	}

	if !walk.ControlDown() && !walk.ShiftDown() {

		//detect selection change
		if index != oldindex {
			sv.selectionChanged(index, sv.SelectedIndex)
		}

		// normal selection, no keys active.
		// reset everything
		clearSelections()
		sv.selections = []*FileInfo{}
		//sv.itemsModel.items[index].checked = true
		sv.selections = append(sv.selections, sv.itemsModel.items[index])
	}

	if walk.ShiftDown() {
		if sv.selStart != -1 {
			var lo, hi int
			if sv.selStart < index {
				lo, hi = sv.selStart, index
			} else {
				lo, hi = index, sv.selStart
			}

			// append sel slice for items in range lo-hi
			for i, v := range sv.itemsModel.items {
				if i >= lo && i <= hi {
					//only append those not already in selections
					if _, ok := checkItem(v); !ok {
						sv.selections = append(sv.selections, v)
					}
				}
			}
		}
	} else if walk.ControlDown() {
		vv := sv.itemsModel.items[index]

		//if item already in selections, remove it
		//else add to selections
		if ii, ok := checkItem(vv); ok {
			vv.checked = false
			sv.selections = append(sv.selections[:ii], sv.selections[ii+1:]...)
		} else {
			sv.selections = append(sv.selections, vv)
		}
	}

	sv.selStart = index

	// apply the computed selections
	for _, v := range sv.selections {
		v.checked = true
	}
	if redraw {
		sv.Repaint()
	}
}

func (sv *ScrollViewer) SetViewWidth(newWidth int) {

}
func (sv *ScrollViewer) Invalidate() {
	sv.canvasView.Invalidate()
}
func (sv *ScrollViewer) Show() {
	sv.scrollview.Parent().SetVisible(true)
}
func (sv *ScrollViewer) Width() int {
	return sv.scrollview.Width()
}
func (sv *ScrollViewer) Height() int {
	return sv.scrollview.Height()
}
func (sv *ScrollViewer) ViewWidth() int {
	return sv.canvasView.Width()
}
func (sv *ScrollViewer) ViewHeight() int {
	return sv.canvasView.Height()
}
func (sv *ScrollViewer) NumRows() int {
	return int(math.Ceil(float64(sv.itemsCount) / float64(sv.NumCols())))
}
func (sv *ScrollViewer) NumRowsVisible() int {
	return int(math.Ceil(float64(sv.ViewHeight()) / float64(sv.itemHeight)))
}
func (sv *ScrollViewer) NumRowsFit() int {
	return int(math.Floor(float64(sv.ViewHeight()) / float64(sv.itemHeight)))
}
func (sv *ScrollViewer) NumCols() int {
	return int(math.Trunc(float64(sv.canvasView.Width()) / float64(sv.itemWidth)))
}

func (sv *ScrollViewer) onPaint(canvas *walk.Canvas, updaterect walk.Rectangle) error {
	//Shift screen update rect
	//to virtual view rect
	if !sv.viewInfo.scrolling {
		updaterect.Y += sv.viewInfo.topPos
	}

	sv.exitPreviewMode()

	if sv.evPaint == nil {
		sv.evPaint = drawGrid
	}
	sv.evPaint(sv, canvas, updaterect, sv.ViewBounds())

	return nil
}
func (sv *ScrollViewer) Repaint() {
	cvs, _ := sv.canvasView.CreateCanvas()
	defer cvs.Dispose()

	sv.onPaint(cvs, sv.canvasView.Bounds())
}

func (sv *ScrollViewer) setSelectionVisible() {
	if sv.NumCols() == 0 {
		return
	}
	idx := sv.SelectedIndex
	if sv.isValidIndex(idx) {
		row := idx / sv.NumCols()
		top := row * sv.itemHeight

		if sv.scrollview.Value() != top {
			toprow := top - (sv.ViewHeight()-sv.itemHeight)/2

			if toprow < 0 {
				toprow = 0
			}
			sv.SetScrollPos(toprow)
		}
	}
}

func (sv *ScrollViewer) onTestImagePaint(canvas *walk.Canvas, updaterect walk.Rectangle) error {
	var ft *walk.Font
	ft, _ = walk.NewFont("arial", 20, walk.FontBold)

	p, _ := walk.NewCosmeticPen(walk.PenSolid, walk.RGB(0, 0, 0))
	defer p.Dispose()
	defer ft.Dispose()

	w := sv.itemWidth
	h := sv.itemHeight
	num := sv.itemsCount
	numcols := sv.NumCols()

	for i := 0; i < num; i++ {
		x := (i % numcols) * w
		y := int(i/numcols) * h

		r := walk.Rectangle{x, y, w, h}

		canvas.DrawRectangle(p, r)
		canvas.DrawText(fmt.Sprintf("A%d", i), ft, walk.RGB(0, 0, 0), r, walk.TextCenter|walk.TextVCenter|walk.TextSingleLine)
	}
	return nil
}
func (sv *ScrollViewer) contentMonitorInfoHandler() {

	sv.canvasView.Synchronize(func() {
		sv.canvasView.Invalidate()
	})
}
func (sv *ScrollViewer) directoryMonitorInfoHandler(path string) {

	sv.canvasView.Synchronize(func() {
		sv.itemsModel.PublishRowsReset()
		sv.SetItemsCount(len(sv.itemsModel.items))
		sv.canvasView.Invalidate()

		//Relay this event to subscribers
		if sv.directoryMonitor.infofunc != nil {
			sv.directoryMonitor.infofunc(path)
		}
	})
}
func (sv *ScrollViewer) MaxScrollValue() int {
	return sv.scrollview.MaxValue()
}

//------------------------------------
// ALBUM Management routines
//------------------------------------
func (sv *ScrollViewer) AlbumAddItems(svSource *ScrollViewer) bool {

	if len(svSource.selections) > 0 {
		if sv.SelectedItem() != nil {
			albumID := sv.SelectedItem().index

			//update the info field with the source dirpath of the item
			for _, v := range svSource.selections {
				if v.Info == "" {
					v.Info = svSource.itemsModel.dirPath
				}
			}

			res, err := sv.AlbumDBUpdateItems(albumID, svSource.selections)
			if err != nil {
				log.Println(err.Error())
			}
			log.Println("AddAlbumItems", res)
			return res > 0
		}
	}
	return false
}
func (sv *ScrollViewer) AlbumDelItems(svSource *ScrollViewer) bool {

	if len(svSource.selections) > 0 {
		if sv.SelectedItem() != nil {

			res, err := sv.AlbumDBDeleteItems(svSource.selections)
			if err != nil {
				log.Println(err.Error())
			}
			log.Println("AlbumDelItems", res)

			for j := 0; j < len(svSource.selections); j++ {
				for i, v := range svSource.itemsModel.items {
					if v.index == -1 {
						key := sv.itemsModel.getFullItemPath(v)
						delete(sv.ItemsMap, key)
						svSource.itemsModel.items = append(svSource.itemsModel.items[:i], svSource.itemsModel.items[i+1:]...)
						break
					}
				}
			}
			svSource.SetItemsCount(len(svSource.itemsModel.items))
			svSource.Invalidate()

			return res > 0
		}
	}
	return false
}
func (sv *ScrollViewer) AlbumEnumItems(svTarget *ScrollViewer) bool {

	if svTarget != nil {
		if sv.SelectedItem() != nil {
			albumID := sv.SelectedItem().index
			fi := sv.AlbumDBEnumItems(albumID)

			if len(fi) > 0 {
				svTarget.RunAlbumItems(fi)
				return true
			}
		}
	}
	return false
}
