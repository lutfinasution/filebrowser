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
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

type ItmMap map[string]*FileInfo
type painterfunc func(sv *ScrollViewer, canvas *walk.Canvas, updaterect walk.Rectangle, viewrect walk.Rectangle) error

type ScrollViewer struct {
	MainWindow     *walk.MainWindow
	scrollview     *walk.Composite
	canvasView     *walk.CustomWidget
	optionsPanel   *walk.Composite
	scroller       *CustomSlider
	scrollcmp      *walk.Composite
	scrollControl  ScrollController
	scrlUp, scrlDn *walk.PushButton
	ID             win.HWND
	itemSize       ThumbSizes
	itemsCount     int
	itemWidth      int
	itemHeight     int
	SelectedIndex  int
	currentLayout  int
	dblClickTime   time.Time
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
	drawerFunc    func(sv *ScrollViewer, path string, data *FileInfo)
	drawersChan   chan *FileInfo
	drawersWait   sync.WaitGroup
	drawerMutex   sync.Mutex
	drawersActive bool

	// local vars
	suspendPreview     bool
	isResizing         bool
	doCache            bool
	handleChangedItems bool
	lastButtonDown     walk.MouseButton
	PreviewRect        *walk.Rectangle
	previewBackground  *walk.Bitmap
	// ui
	lblSize  *walk.Label
	cmbMode  *walk.ComboBox
	sldrSize *walk.Slider
	cbCached *walk.CheckBox
	// public event handlers
	evPaint     painterfunc
	evMouseDown walk.MouseEventHandler
}

type drawBuffer struct {
	size     walk.Size
	drawDib  win.HBITMAP
	drawPtr  unsafe.Pointer
	drawHDC  win.HDC
	hdcOld   win.HDC
	destHDC  win.HDC
	zoom     float64
	viewinfo ViewInfo
}

func (db *drawBuffer) canPan() bool {
	return (db.zoom != 0) && (db.zoomSize().Width > db.viewinfo.viewRect.Dx() || db.zoomSize().Height > db.viewinfo.viewRect.Dy())
}
func (db *drawBuffer) canPanX() bool {
	return (db.zoom != 0) && (db.zoomSize().Width > db.viewinfo.viewRect.Dx())
}
func (db *drawBuffer) canPanY() bool {
	return (db.zoom != 0) && (db.zoomSize().Height > db.viewinfo.viewRect.Dy())
}
func (db *drawBuffer) zoomSize() walk.Size {

	ws, hs := db.size.Width, db.size.Height

	if db.zoom != 0 {
		return walk.Size{int(db.zoom * float64(ws)), int(db.zoom * float64(hs))}
	} else {
		return db.fitSize()
	}
}
func (db *drawBuffer) zoomSizeAt(fzoom float64) walk.Size {

	ws, hs := db.size.Width, db.size.Height

	return walk.Size{int(fzoom * float64(ws)), int(fzoom * float64(hs))}
}
func (db *drawBuffer) fitSize() walk.Size {

	wd, hd := getOptimalThumbSize(db.viewinfo.viewRect.Dx(), db.viewinfo.viewRect.Dy(),
		db.size.Width, db.size.Height)

	return walk.Size{wd, hd}
}

func NewDrawBuffer(width, height int) *drawBuffer {
	db := new(drawBuffer)

	db.drawDib, db.drawPtr = createDrawDibsection(width, height)

	if db.drawDib != 0 {
		db.size = walk.Size{width, height}
		db.drawHDC = win.CreateCompatibleDC(0)
		db.hdcOld = win.HDC(win.SelectObject(db.drawHDC, win.HGDIOBJ(db.drawDib)))
	}

	return db
}
func DeleteDrawBuffer(db *drawBuffer) (res bool) {
	win.SelectObject(db.hdcOld, win.HGDIOBJ(db.drawDib))

	res = win.DeleteDC(db.drawHDC)
	res = res && win.DeleteObject(win.HGDIOBJ(db.drawDib))

	db = nil
	return res
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

//var viewInfo ViewInfo

type workitem struct {
	path string
	done bool
}
type WorkMap map[string]*workitem
type ImageProcessor struct {
	workmap       WorkMap
	doCancelation bool
	donewait      sync.WaitGroup
	workerWaiter  sync.WaitGroup
	imageWorkChan []chan string
	workStatus    *ProgresDrawer
	workCounter   uint64
	statuswidget  *walk.StatusBar
	statusfunc    func(i int)
	infofunc      func(numjob int, d float64)
}

func (ip *ImageProcessor) setstatuswidget(widget *walk.StatusBar) {
	ip.statuswidget = widget
}
func (ip *ImageProcessor) Close(sv *ScrollViewer) bool {

	ip.donewait.Add(1)
	ip.workCounter = 0
	gorCount := runtime.NumCPU()

	go func() {
		log.Println("Terminating all ImageProcessor goroutines")

		//setup waiters
		ip.workerWaiter.Add(gorCount)

		for i := 0; i < gorCount; i++ {
			key := ""

			//send exit data through
			//worker channel
			ip.imageWorkChan[0] <- key
		}
		//wait for all workers to finish
		ip.workerWaiter.Wait()
		ip.donewait.Done()

		log.Println("ImageProcessor goroutines all terminated")

	}()

	return true
}
func (ip *ImageProcessor) Run(sv *ScrollViewer, jobList []*FileInfo, dirpath string) bool {
	runtime.GC()
	sv.handleChangedItems = false

	if ip.workmap == nil {
		ip.workmap = make(WorkMap)
	}

	if len(ip.workmap) > 0 {
		ip.doCancelation = true
		ip.donewait.Wait()
		ip.doCancelation = false
	}

	numJob := len(jobList)

	//add work items to workmap
	for _, v := range jobList {
		ip.workmap[filepath.Join(dirpath, v.Name)] = &workitem{path: dirpath}
	}

	//determine the num of worker goroutines
	gorCount := runtime.NumCPU()
	ip.donewait.Add(1)
	ip.workCounter = 0

	if ip.imageWorkChan == nil {
		ip.imageWorkChan = make([]chan string, gorCount)
		for i := range ip.imageWorkChan {
			ip.imageWorkChan[i] = make(chan string)
		}
		//--------------------------
		//run the worker goroutines
		//--------------------------
		for j := 0; j < gorCount; j++ {
			//go ip.doRenderTasker(sv, j+1, ip.imageWorkChan[j])    //with individual channel
			go ip.doRenderTasker(sv, j+1, ip.imageWorkChan[0]) //with just one shared channel
		}
	}

	go func(itms WorkMap) {
		t := time.Now()

		getNextItem := func() (res string) {
			for key, _ := range itms {
				res = key
				delete(itms, key)
				break
			}
			return res
		}

		//Load data from cache
		n := sv.CacheDBEnum(dirpath)

		log.Println("CacheDBEnum found", n, "in", dirpath)

		if ip.statuswidget != nil {
			ip.workStatus = NewProgresDrawer(ip.statuswidget.AsWidgetBase(), 240, numJob)
		}

		//setup waiters
		ip.workerWaiter.Add(len(itms))

		//run the distributor
		i := 0

	loop:
		for {
			if !ip.doCancelation {
				n := 0
				key := getNextItem()
				if key != "" {
					//send data through worker channel
					ip.imageWorkChan[n] <- key

					if ip.statusfunc != nil {
						ip.statusfunc(i)
					}
					i++
				} else {
					break loop
				}
			}

			if i%4 == 0 {
				if ip.workStatus != nil {
					ip.workStatus.DrawProgress(i)
				}
			}
		}

		if ip.infofunc != nil {
			d := time.Since(t).Seconds()
			ip.infofunc(numJob, d)

			sv.recalc()
		}
		sv.canvasView.Synchronize(func() {
			sv.canvasView.Invalidate()
		})

		//wait for all workers to finish
		ip.workerWaiter.Wait()

		//log.Println("ImageProcessor Run finished", itms, sv.ItemsMap)

		if ip.workStatus != nil {
			ip.workStatus.Clear()
		}

		//update db for items in this path only
		cntupd, _ := sv.CacheDBUpdateMapItems(sv.ItemsMap, dirpath)

		sv.handleChangedItems = true

		if !ip.doCancelation {
			wc := atomic.LoadUint64(&ip.workCounter)
			log.Println("Cache items processed: ", wc)
			log.Println("Cache items updated: ", cntupd)
		}
		ip.donewait.Done()
	}(ip.workmap)

	return true
}

func (ip *ImageProcessor) doRenderTasker(sv *ScrollViewer, id int, fnames chan string) bool {
	//icount := 0
loop:
	for v := range fnames {
		if v != "" {
			if processImageData(sv, v, true, nil) != nil {
				atomic.AddUint64(&ip.workCounter, 1)
			}

			//decrement the wait counter
			ip.workerWaiter.Done()
		} else {
			log.Println("doRenderTasker exiting..this-should-not-have-happened.")
			break loop
		}
	}
	return true
}

type ScrollController struct {
	isRunning  bool
	isDone     bool
	doneWaiter sync.WaitGroup
}

func (sc *ScrollController) Init() {
	sc.doneWaiter.Add(1)
}
func (sc *ScrollController) endScroll() {
	if sc.isRunning {
		sc.isRunning = false
		sc.doneWaiter.Wait()
	}
}
func (sc *ScrollController) doScroll(scrollfunc func(val int) int, scrollBy int) {

	scrollfunc(scrollBy)

	if !sc.isRunning {
		sc.Init()

		go func() {
			sc.isRunning = true
			i := 0
			delay := 100
			for {
				if sc.isRunning {
					if i > 5 { //delay the loop for 500msec
						scrollfunc(scrollBy)

						delay = 70 - (i/15)*10
						if delay < 30 {
							delay = 30
						}
					}
					time.Sleep(time.Millisecond * time.Duration(delay))
				} else {
					break
				}
				i++
			}
			sc.doneWaiter.Done()
		}()
	}
}

func NewScrollViewer(window *walk.MainWindow, parent walk.Container, paintfunc walk.PaintFunc, itmCount, itmWidth, itmHeight int) (*ScrollViewer, error) {
	var err error
	var defSize = ThumbSizes{120, 75, 10, 10, 48}

	svr := &ScrollViewer{
		MainWindow:    window,
		itemsCount:    itmCount,
		itemWidth:     defSize.twm(),
		itemHeight:    defSize.thm(),
		itemSize:      defSize,
		SelectedIndex: -1,
		currentLayout: 0,
		ItemsMap:      make(map[string]*FileInfo),
	}

	svr.itemsModel = new(FileInfoModel)
	svr.itemsModel.viewer = svr

	svr.imageProcessor = new(ImageProcessor)
	svr.contentMonitor = new(ContentMonitor)
	svr.contentMonitor.imageprocessor = svr.imageProcessor

	svr.directoryMonitor = new(DirectoryMonitor)
	svr.directoryMonitor.viewer = svr
	svr.directoryMonitor.imagemon = svr.contentMonitor

	parent.SetSuspended(true)
	//UI components:
	svr.scrollview, _ = walk.NewComposite(parent)
	svr.canvasView, _ = walk.NewCustomWidget(svr.scrollview, 0, svr.onPaint)
	svr.canvasView.SetPaintMode(walk.PaintNoErase)

	svr.scrollcmp, _ = walk.NewComposite(svr.scrollview)

	//----------------------
	//CustomSlider
	//----------------------
	ctv := new(CustomSlider)
	ctv.host = svr
	ctv.Slider, err = walk.NewSliderWithOrientation(svr.scrollcmp, walk.Vertical)
	svr.scroller = ctv
	if err := walk.InitWrapperWindow(ctv); err != nil {
		log.Fatal(err)
	}
	svr.scroller.KeyDown().Attach(svr.OnKeyPress)

	svr.scrlUp, _ = walk.NewPushButton(svr.scrollview)
	svr.scrlDn, _ = walk.NewPushButton(svr.scrollview)

	svr.scrlUp.MouseDown().Attach(func(x, y int, button walk.MouseButton) { svr.OnButtonDown(svr.scrlUp) })
	svr.scrlDn.MouseDown().Attach(func(x, y int, button walk.MouseButton) { svr.OnButtonDown(svr.scrlDn) })
	svr.scrlUp.MouseUp().Attach(svr.OnButtonUp)
	svr.scrlDn.MouseUp().Attach(svr.OnButtonUp)

	img, err := walk.NewImageFromFile("./image/aup.png")
	svr.scrlUp.SetImage(img)
	img, err = walk.NewImageFromFile("./image/adown.png")
	svr.scrlDn.SetImage(img)

	svr.scrlUp.SetText(".")
	svr.scrlDn.SetText(".")
	svr.scrlUp.SetImageAboveText(true)
	svr.scrlDn.SetImageAboveText(true)

	svr.optionsPanel, err = walk.NewComposite(svr.scrollview)

	var pb1, pb2, pb3, pb4, pb5 *walk.ToolButton

	//Declarative style
	ft := Font{Family: parent.Font().Family(), PointSize: 10, Bold: false}
	ft2 := Font{Family: parent.Font().Family(), PointSize: 9, Bold: true}
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
						Layout: Grid{Columns: 5, SpacingZero: true, MarginsZero: true},
						OnSizeChanged: func() {
							pb1.SetWidth(90)
							pb2.SetWidth(90)
							pb3.SetWidth(90)
							pb4.SetWidth(90)
							pb5.SetWidth(90)
						},
						Children: []Widget{
							ToolButton{
								AssignTo:    &pb1,
								Text:        "N",
								ToolTipText: "Sort by name",
								OnClicked: func() {
									svr.onSorterAction(pb1)
								},
							},
							ToolButton{
								AssignTo:    &pb2,
								Text:        "S",
								ToolTipText: "Sort by size",
								OnClicked: func() {
									svr.onSorterAction(pb2)
								},
							},
							ToolButton{
								AssignTo:    &pb3,
								Text:        "D",
								ToolTipText: "Sort by date",
								OnClicked: func() {
									svr.onSorterAction(pb3)
								},
							},
							ToolButton{
								AssignTo:    &pb4,
								Text:        "W",
								ToolTipText: "Sort by width",
								OnClicked: func() {
									svr.onSorterAction(pb4)
								},
							},
							ToolButton{
								AssignTo:    &pb5,
								Text:        "H",
								ToolTipText: "Sort by height",
								OnClicked: func() {
									svr.onSorterAction(pb5)
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
						Font:     ft2,
						Model: []string{
							"Frameless, variable size",
							"Grid with name, date, and size",
							"Grid with name and date",
							"Grid with name and size",
							"Grid with name only",
							"Grid with no text",
						},
						OnCurrentIndexChanged: svr.setLayoutMode,
					},
					HSpacer{
						ColumnSpan: 1,
					},
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
						MinValue:       120,
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
	svr.canvasView.KeyPress().Attach(svr.OnKeyPress)
	svr.scrollview.SizeChanged().Attach(svr.onSizeChanged)
	svr.scroller.ValueChanged().Attach(svr.OnScrollerValueChanged)

	parent.SizeChanged().Attach(svr.onSizeParentChanged)

	br, _ := walk.NewSolidColorBrush(walk.RGB(20, 20, 20))
	svr.canvasView.SetBackground(br)

	br, _ = walk.NewSolidColorBrush(walk.RGB(44, 44, 44))
	svr.scrollcmp.SetBackground(br)

	svr.onSizeParentChanged()
	svr.resizing()
	svr.SetLayoutMode(0)

	parent.SetSuspended(false)

	svr.ID = svr.scrollview.Handle()
	return svr, err
}

func (sv *ScrollViewer) onSorterAction(tb *walk.ToolButton) {

	flipsort := func(index int) {
		if sv.itemsModel.SortedColumn() == index {
			if sv.itemsModel.SortOrder() == walk.SortAscending {
				sv.itemsModel.Sort(index, walk.SortDescending)
			} else {
				sv.itemsModel.Sort(index, walk.SortAscending)
			}
		} else {
			sv.itemsModel.Sort(index, walk.SortAscending)
		}
	}
	switch {
	case tb.Text() == "N":
		flipsort(0)
	case tb.Text() == "S":
		flipsort(1)
	case tb.Text() == "D":
		flipsort(2)
	case tb.Text() == "W":
		flipsort(4)
	case tb.Text() == "H":
		flipsort(5)
	}
	sv.Invalidate()
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

	sv.contentMonitor.removeChangedItems(sv.contentMonitor.doneMap)

	//Updating to reflect the num of items
	sv.SetItemsCount(len(sv.itemsModel.items))

	//initialize cache database
	if CacheDB == nil {
		sv.OpenCacheDB("")
	}
	//--------------------------------
	//run the imageProcessor workers
	//--------------------------------
	sv.imageProcessor.Run(sv, sv.itemsModel.items, dirPath)

	if watchThisPath && sv.itemsCount > 0 {
		sv.directoryMonitor.setFolderWatcher(dirPath)
	}
}
func (sv *ScrollViewer) OnButtonDown(button *walk.PushButton) {
	if button == sv.scrlUp {
		sv.scrollControl.doScroll(sv.setScrollPosBy, -sv.itemHeight/2)
	} else if button == sv.scrlDn {
		sv.scrollControl.doScroll(sv.setScrollPosBy, sv.itemHeight/2)
	}
}
func (sv *ScrollViewer) OnButtonUp(x, y int, button walk.MouseButton) {
	sv.scrollControl.endScroll()
}
func (sv *ScrollViewer) oncanvasViewpaint(canvas *walk.Canvas, updaterect walk.Rectangle) error {
	return nil
}

func (sv *ScrollViewer) OnKeyPress(key walk.Key) {

	switch key {
	case walk.KeyReturn:
		sv.ShowPreviewFull()
	case walk.KeyLeft:
		sv.SetItemSelected(sv.SelectedIndex - 1)
	case walk.KeyRight:
	case walk.KeyUp:
		sv.setScrollPosBy(-1)
	case walk.KeyDown:
		sv.setScrollPosBy(1)
	}

}
func (sv *ScrollViewer) OnMouseDown(x, y int, button walk.MouseButton) {

	//mouseup does not give this
	sv.lastButtonDown = button

	//perform selection
	idx := sv.GetItemAtScreen(x, y)
	sv.SetItemSelected(idx)

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

	sv.scroller.SetFocus()
	sv.Repaint()
}
func (sv *ScrollViewer) OnMouseMove(x, y int, button walk.MouseButton) {
	//perform mouse move
	if button == walk.LeftButton && sv.PreviewRect == nil {
		sv.viewInfo.mousemoveY = sv.viewInfo.mouseposY - y

		if sv.scroller.Value() != (sv.viewInfo.scrollpos + sv.viewInfo.mousemoveY) {
			sv.scroller.SetValue(sv.viewInfo.scrollpos + sv.viewInfo.mousemoveY)
		}
	} else {
		prt := sv.scrollview.Parent().AsContainerBase()
		num := prt.Children().Len()

		hwnd := GetForegroundWindow()
		if hwnd == sv.MainWindow.Handle() && num > 0 && !sv.scroller.Focused() {
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

	//Display image preview
	if sv.lastButtonDown == walk.RightButton && !sv.suspendPreview {
		sv.PreviewItemAtScreen(x, y)
	}

	//double click to launch preview
	if sv.dblClickTime.IsZero() {
		sv.dblClickTime = time.Now()
	} else {
		d := time.Since(sv.dblClickTime)
		if d.Seconds() < 0.500 {
			//log.Println("doubleclick", d.Seconds())
			sv.ShowPreviewFull()
		}
		sv.dblClickTime = time.Now()
	}
}

func (sv *ScrollViewer) onSizeParentChanged() {
	//manage scrollviewer objects placement
	//distribute scrollviewer objects vertically
	if sv.scrollview.Parent() == nil {
		return
	}
	p := sv.scrollview.Parent().AsContainerBase()
	n := p.Children().Len()
	h := p.Height() / n

	for i := 0; i < n; i++ {
		ch := p.Children().At(i)
		ch.SetBounds(walk.Rectangle{0, i * (h + 2), p.Width(), h})
	}
}
func (sv *ScrollViewer) onSizeChanged() {
	sv.resizing()
	defer doResizing(sv)
}
func (sv *ScrollViewer) resizing() {
	sv.optionsPanel.SetBounds(walk.Rectangle{0, 0, sv.Width() - 28, 30})
	sv.canvasView.SetBounds(walk.Rectangle{0, 30, sv.Width() - 28, sv.Height() - 30})

	sv.scrollcmp.SetBounds(walk.Rectangle{sv.Width() - 28, 30, 28, sv.Height() - 56})
	r := sv.scrollcmp.ClientBounds()
	r.X -= 6
	r.Y -= 4
	r.Width += 8
	r.Height += 8
	sv.scroller.SetBounds(r)

	sv.scrlUp.SetBounds(walk.Rectangle{sv.Width() - 29, 3, 30, 28})
	sv.scrlDn.SetBounds(walk.Rectangle{sv.Width() - 29, sv.Height() - 27, 30, 28})

	sv.recalc()
}

var resCount int

func doResizing(sv *ScrollViewer) {
	resCount++
	if !sv.isResizing {
		sv.isResizing = true
		go func() {
			t := time.NewTicker(time.Millisecond * 100)
		loop:
			for {
				if resCount > 0 {
					resCount--
					time.Sleep(time.Millisecond * 10)
				} else {
					select {
					case <-t.C:
						if resCount <= 0 {
							break loop
						}
					}
				}
			}
			sv.isResizing = false
			t.Stop()
			resCount = 0
			sv.scrollview.Synchronize(func() {
				if sv.SelectedIndex == -1 {
					sv.Invalidate()
				} else {
					sv.setSelectionVisible()
				}
			})
		}()
	}
}

func (sv *ScrollViewer) Bounds() walk.Rectangle {
	return sv.scrollview.Bounds()
}
func (sv *ScrollViewer) ViewBounds() walk.Rectangle {
	return sv.canvasView.Bounds()
}

func (sv *ScrollViewer) setLayoutMode() {
	//0"Frameless, variable size"
	//1"Grid with name, date, and size",
	//2"Grid with name and date",
	//3"Grid with name and size",
	//4"Grid with name only",
	//5"Grid with no text",

	sv.currentLayout = sv.cmbMode.CurrentIndex()

	if sv.cmbMode.CurrentIndex() != 0 {
		sv.itemSize.mx = 10
		sv.itemSize.my = 10
	}

	switch sv.cmbMode.CurrentIndex() {
	case 0:
		sv.itemSize.mx = 0
		sv.itemSize.my = 0
		sv.itemSize.txth = 0
		sv.evPaint = RedrawScreenNB2
		sv.viewInfo.showName = false
		sv.viewInfo.showDate = false
		sv.viewInfo.showInfo = false
	case 1:
		sv.evPaint = RedrawScreenNB
		sv.viewInfo.showName = true
		sv.viewInfo.showDate = true
		sv.viewInfo.showInfo = true
		sv.itemSize.txth = 3 * 16
	case 2:
		sv.evPaint = RedrawScreenNB
		sv.viewInfo.showName = true
		sv.viewInfo.showDate = true
		sv.viewInfo.showInfo = false
		sv.itemSize.txth = 2 * 17
	case 3:
		sv.evPaint = RedrawScreenNB
		sv.viewInfo.showName = true
		sv.viewInfo.showDate = false
		sv.viewInfo.showInfo = true
		sv.itemSize.txth = 2 * 17
	case 4:
		sv.evPaint = RedrawScreenNB
		sv.viewInfo.showName = true
		sv.viewInfo.showDate = false
		sv.viewInfo.showInfo = false
		sv.itemSize.txth = 1 * 20
	case 5:
		sv.evPaint = RedrawScreenNB
		sv.viewInfo.showName = false
		sv.viewInfo.showDate = false
		sv.viewInfo.showInfo = false
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
		sv.evPaint = RedrawScreenNB2
		sv.cmbMode.SetCurrentIndex(idx)
	case 1, 2, 3, 4, 5:
		sv.evPaint = RedrawScreenNB
		sv.cmbMode.SetCurrentIndex(idx)
	}
}
func (sv *ScrollViewer) GetLayoutMode() int {
	return sv.currentLayout
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
		info1 := fmt.Sprintf("%d x %d, %d KB", v.Width, v.Height, v.Size/1024)
		info2 := v.Modified.Format("Jan 2, 2006 3:04pm")
		res = info1 + "   " + info2
	}
	return res
}
func (sv *ScrollViewer) GetItemAtScreen(x int, y int) (idx int) {

	if sv.GetLayoutMode() == 0 {
		idx = sv.getItemAtScreenNB2(x, y)
	} else {
		col := x / sv.itemWidth
		idx = -1

		if col < sv.NumCols() {
			row := int(float32(y+sv.viewInfo.topPos) / float32(sv.itemHeight))
			idx = col + row*sv.NumCols()
			if idx >= sv.itemsCount {
				idx = -1
			}
		}
	}
	return idx
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
func (sv *ScrollViewer) getItemRectAtScreen(xs, ys int) *image.Rectangle {
	if sv.GetLayoutMode() != 0 {
		w := sv.itemSize.twm()
		h := sv.itemSize.thm()

		col := int(float32(xs) / float32(w))
		row := int(float32(ys+sv.viewInfo.topPos) / float32(h))
		x1 := col * w
		y1 := row * h
		r := image.Rect(x1, y1+h-sv.itemSize.txth, x1+w, y1+h)
		return &r
	} else {
		return sv.getItemRectAtScreenNB2(xs, ys)
	}
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
	return y + h + 2
}

func (sv *ScrollViewer) isValidIndex(idx int) bool {
	return idx >= 0 && idx < sv.itemsCount
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

	if sv.SelectedIndex != -1 {
		if sv.SelectedNameFull() != "" {
			NewImageViewWindow(sv.MainWindow, sv.SelectedNameFull(), sv.itemsModel)
		}
	}

	return true
}

func (sv *ScrollViewer) recalc() int {
	hMax := sv.ViewHeight()
	switch sv.GetLayoutMode() {
	case 0:
		hMax = sv.getTotalHeightNB2()
	case 1, 2, 3, 4, 5:
		hMax = sv.NumRows() * sv.itemHeight
	}

	sv.scroller.SetRange(0, hMax-sv.ViewHeight())
	sv.scroller.SendMessage(win.WM_USER+21, 0, uintptr(sv.itemHeight*(sv.NumRowsVisible()-1))) //TBM_SETPAGESIZE
	sv.scroller.SendMessage(win.WM_USER+23, 0, uintptr(sv.itemHeight))                         //TBM_SETLINESIZE

	wstyle := win.GetWindowLong(sv.scroller.Handle(), win.GWL_STYLE)
	wstyle = wstyle | 0x0008 //0x0040
	win.SetWindowLong(sv.scroller.Handle(), win.GWL_STYLE, wstyle)

	sv.viewInfo.topPos = sv.scroller.Value()
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
	sv.scroller.SetFocus()
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
		sv.ResetScrollPos()
		sv.Invalidate()

		//close folder watcher
		//sv.directoryMonitor.Close()
		sv.directoryMonitor.setFolderWatcher("")
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

		sv.contentMonitor.removeChangedItems(sv.contentMonitor.doneMap)
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
func (sv *ScrollViewer) ResetScrollPos() {
	sv.scroller.SetValue(0)
	sv.canvasView.Invalidate()
}
func (sv *ScrollViewer) setScrollPosBy(val int) int {
	sv.scroller.Synchronize(func() {
		sv.scroller.SetValue(sv.scroller.Value() + val)
	})
	return sv.scroller.Value()
}
func (sv *ScrollViewer) SetScrollPos(val int) {
	sv.scroller.SetValue(val)
	sv.Invalidate()
	//sv.Repaint()
}
func (sv *ScrollViewer) SetRowScroll(val int) {

	//	if val == viewInfo.topPos {
	//		return
	//	}

	r := image.Rect(0, val, sv.ViewWidth(), val+sv.ViewHeight())

	iscrollSize := int(math.Abs(float64(val - sv.viewInfo.topPos)))

	if iscrollSize > 2*sv.itemHeight {
		iscrollSize = 2 * sv.itemHeight
	}

	bScrollDown := (val > sv.viewInfo.topPos)

	sv.viewInfo.topPos = val
	sv.viewInfo.numCols = sv.NumCols()
	sv.viewInfo.viewRows = sv.NumRowsVisible()
	sv.viewInfo.viewRect = r

	cvs, _ := sv.canvasView.CreateCanvas()
	defer cvs.Dispose()

	//detect scroll direction
	//setup update rect, reflecting the exposed scroll area
	posY := 0
	if bScrollDown {
		posY = r.Max.Y - iscrollSize
	} else {
		posY = r.Min.Y
	}
	rupdate := walk.Rectangle{0, posY, sv.ViewWidth(), iscrollSize}

	//set scrolling flag for the onpaint
	sv.viewInfo.scrolling = true

	//trigger onpaint to handle the rest of the drawing
	sv.onPaint(cvs, rupdate)

	//switch back the flag
	sv.viewInfo.scrolling = false
}

func (sv *ScrollViewer) SetItemSelected(index int) {

	sv.SelectedIndex = index
	if sv.isValidIndex(index) {
		if !walk.ControlDown() {
			for _, v := range sv.itemsModel.items {
				v.checked = false
			}
		}
		sv.itemsModel.items[index].checked = true
	} else {
		for _, v := range sv.itemsModel.items {
			v.checked = false
		}
	}
}

func (sv *ScrollViewer) SetViewWidth(newWidth int) {
	//sv.scrollview.Parent().SetWidth(newWidth)
	//sv.scrollview.Parent().AsContainerBase().Parent().Layout().Update(true)

	//log.Println("SetViewWidth: ", sv.scrollview.Width())
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
func (sv *ScrollViewer) OnScrollerValueChanged() {
	sv.SetRowScroll(sv.scroller.Value())
}
func (sv *ScrollViewer) onPaint(canvas *walk.Canvas, updaterect walk.Rectangle) error {
	//Shift screen update rect
	//to virtual view rect
	if !sv.viewInfo.scrolling {
		updaterect.Y += sv.viewInfo.topPos
	}

	sv.exitPreviewMode()

	if sv.evPaint == nil {
		sv.evPaint = RedrawScreenNB
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

		if sv.scroller.Value() != top {
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

type CustomSlider struct {
	*walk.Slider
	host *ScrollViewer
}

func (sl *CustomSlider) WndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_HSCROLL, win.WM_VSCROLL:
		//switch win.LOWORD(uint32(wParam)) {
		//case win.TB_THUMBPOSITION, win.TB_ENDTRACK:
		//sl.valueChangedPublisher.Publish()
		//}
		sl.host.OnScrollerValueChanged()
		return 0
	case win.WM_MOUSEWHEEL:
		if delta := int16(win.HIWORD(uint32(wParam))); delta < 0 {
			//DOWN
			sl.Slider.SetValue(sl.Slider.Value() + sl.host.itemHeight)
		} else {
			//UP
			sl.Slider.SetValue(sl.Slider.Value() - sl.host.itemHeight)
		}

		return 0
	}
	return sl.WidgetBase.WndProc(hwnd, msg, wParam, lParam)
}

type ContentMonitor struct {
	//viewer         *ScrollViewer
	imageprocessor *ImageProcessor
	changeMap      ItmMap
	doneMap        ItmMap
	activated      bool
	infofunc       func()
	itmMutex       sync.Mutex
	runMutex       sync.Mutex
	statuswidget   *walk.StatusBar
}

func (im *ContentMonitor) setstatuswidget(widget *walk.StatusBar) {
	im.statuswidget = widget
}
func (im *ContentMonitor) removeChangedItem(mkey string) {
	if im.changeMap != nil {
		if _, ok := im.changeMap[mkey]; ok {
			delete(im.changeMap, mkey)
		}
	}
}
func (im *ContentMonitor) removeChangedItems(cmp ItmMap) {
	if cmp != nil {
		for k, _ := range cmp {
			delete(cmp, k)
		}
	}
}
func (im *ContentMonitor) submitChangedItem(mkey string, cItm *FileInfo) {
	if im.changeMap == nil {
		im.changeMap = make(ItmMap)
	}
	if im.doneMap == nil {
		im.doneMap = make(ItmMap)
	}

	//new item must not already be in the doneMap & changeMap
	im.itmMutex.Lock()
	if _, ok := im.doneMap[mkey]; !ok {
		if _, ok := im.changeMap[mkey]; !ok {
			im.changeMap[mkey] = cItm
			im.changeMap[mkey].Changed = true

			log.Println("submitChangedItem: ", mkey)
		}

	}
	im.itmMutex.Unlock()
}

func (im *ContentMonitor) processChangedItem(sv *ScrollViewer, repaint bool) {
	if im.changeMap == nil {
		return
	}
	if len(im.changeMap) == 0 {
		return
	}
	if !im.activated {
		//copy changeMap to a string slice
		//important to stability
		var worklist []string
		for key, _ := range im.changeMap {
			worklist = append(worklist, key)
		}

		go func(workitmlist []string) {
			ires := 0
			//im.runMutex.Lock()
			im.activated = true
			log.Println("processChangedItem ---------------------------------")

			jobStatus := NewProgresDrawer(im.statuswidget.AsWidgetBase(), 100, len(workitmlist))

			if im.imageprocessor.imageWorkChan != nil {
				im.imageprocessor.workerWaiter.Add(len(workitmlist))

				for i, key := range workitmlist {
					//send data to workers by writing
					//to the common channel

					//key shouldn't already be in doneMap
					if _, ok := im.doneMap[key]; !ok {

						im.imageprocessor.imageWorkChan[0] <- key

						im.itmMutex.Lock()
						im.doneMap[key] = &FileInfo{Name: key}
						im.itmMutex.Unlock()

						v := im.doneMap[key]
						v.dbsynched = false

						if jobStatus != nil {
							jobStatus.DrawProgress(i)
						}
						ires = i
					}
				}
				im.imageprocessor.workerWaiter.Wait()
			}
			if len(workitmlist) > 0 {
				n, _ := sv.CacheDBUpdateMapItems(im.doneMap, "")

				im.itmMutex.Lock()
				im.removeChangedItems(im.changeMap)
				im.itmMutex.Unlock()

				if repaint && ires > 0 {
					if im.infofunc != nil {
						im.infofunc()
					}
					sv.canvasView.Synchronize(func() {
						sv.canvasView.Invalidate()
					})
				}

				log.Println("processChangedItem/processImageData: ", ires+1)
				log.Println("processChangedItem/CacheDBUpdateMapItems: ", n)
				jobStatus.Clear()
			}

			im.activated = false
			//im.runMutex.Unlock()
		}(worklist)
	}
}
