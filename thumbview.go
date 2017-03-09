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
	//"unsafe"
)

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

type ItmMap map[string]*FileInfo
type painterfunc func(sv *ScrollViewer, canvas *walk.Canvas, updaterect walk.Rectangle, viewrect walk.Rectangle) error

type ScrollViewer struct {
	ID            win.HWND
	MainWindow    *walk.MainWindow
	scrollview    *walk.Composite
	canvasView    *walk.CustomWidget
	optionsPanel  *walk.Composite
	scroller      *CustomSlider
	scrollControl ScrollController
	pb1, pb2      *walk.PushButton
	itemSize      ThumbSizes
	itemSizeTmp   *ThumbSizes
	itemsCount    int
	itemWidth     int
	itemHeight    int
	SelectedIndex int
	currentLayout int
	evPaint       painterfunc
	evMouseDown   walk.MouseEventHandler
	//
	imageProcessor   *ImageProcessor
	contentMonitor   *ContentMonitor
	directoryMonitor *DirectoryMonitor
	//
	itemsModel *FileInfoModel
	ItemsMap   ItmMap
	viewInfo   ViewInfo
	//screen drawers:
	drawersActive bool
	drawerBitmap  *walk.Bitmap
	drawerCanvas  *walk.Canvas
	drawerHDC     win.HDC
	drawerMutex   sync.Mutex
	drawersCount  int
	drawersChan   chan *FileInfo
	drawersWait   sync.WaitGroup
	drawerFunc    func(sv *ScrollViewer, path string, data *FileInfo)
	//
	SuspendPreview     bool
	isResizing         bool
	doCache            bool
	handleChangedItems bool
	lastButtonDown     walk.MouseButton
	PreviewRect        *walk.Rectangle
	previewBackground  *walk.Bitmap
	bmpCntr            *walk.Bitmap
	cvsCntr            *walk.Canvas
	lblSize            *walk.Label
	cmbMode            *walk.ComboBox
	sldrSize           *walk.Slider
	cbCached           *walk.CheckBox
}

type ViewInfo struct {
	topPos       int //Y
	lastPos      int
	lastMovePos  int
	numCols      int
	numRows      int
	viewRows     int
	viewRect     image.Rectangle
	parentWidth  int
	mousepos     int
	scrollpos    int
	mousemove    int
	scrolling    bool
	initSLBuffer bool
	showName     bool
	showDate     bool
	showInfo     bool
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
		//for key, _ := range itms {
		for {
			if !ip.doCancelation {
				//for n := 0; n < gorCount; n++ {
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
				//}
			}

			if i%4 == 0 {
				if ip.workStatus != nil {
					ip.workStatus.DrawProgress(i)
				}
			}
			//i++
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
func (sc *ScrollController) EndScroll() {
	if sc.isRunning {
		sc.isRunning = false
		sc.doneWaiter.Wait()
	}
}
func (sc *ScrollController) DoScroll(scrollfunc func(val int) int, scrollBy int) {

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
	//svr.canvasView.SetPaintMode(walk.PaintBuffered)

	//----------------------
	//CustomSlider
	//----------------------
	ctv := new(CustomSlider)
	ctv.host = svr
	ctv.Slider, err = walk.NewSliderWithOrientation(svr.scrollview, walk.Vertical)
	svr.scroller = ctv
	if err := walk.InitWrapperWindow(ctv); err != nil {
		log.Fatal(err)
	}

	svr.pb1, _ = walk.NewPushButton(svr.scrollview)
	svr.pb2, _ = walk.NewPushButton(svr.scrollview)

	svr.pb1.MouseDown().Attach(svr.OnButtonDown)
	svr.pb2.MouseDown().Attach(svr.OnButtonDown2)
	svr.pb1.MouseUp().Attach(svr.OnButtonUp)
	svr.pb2.MouseUp().Attach(svr.OnButtonUp)

	img, err := walk.NewImageFromFile("./image/aup.png")
	svr.pb1.SetImage(img)
	img, err = walk.NewImageFromFile("./image/adown.png")
	svr.pb2.SetImage(img)

	svr.pb1.SetText(".")
	svr.pb2.SetText(".")
	svr.pb1.SetImageAboveText(true)
	svr.pb2.SetImageAboveText(true)

	svr.optionsPanel, err = walk.NewComposite(svr.scrollview)

	//ft3, _ := walk.NewFont(svr.scrollview.Font().Family(), 10, 0)
	//svr.optionsPanel.SetFont(ft3)

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
				Layout: Grid{Columns: 7, Margins: Margins{Top: 1, Left: 1, Right: 1, Bottom: 0}, MarginsZero: false},
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
								AssignTo: &pb1,
								Text:     "N",
								OnClicked: func() {
									svr.onSorterAction(pb1)
								},
							},
							ToolButton{
								AssignTo: &pb2,
								Text:     "S",
								OnClicked: func() {
									svr.onSorterAction(pb2)
								},
							},
							ToolButton{
								AssignTo: &pb3,
								Text:     "D",
								OnClicked: func() {
									svr.onSorterAction(pb3)
								},
							},
							ToolButton{
								AssignTo: &pb4,
								Text:     "W",
								OnClicked: func() {
									svr.onSorterAction(pb4)
								},
							},
							ToolButton{
								AssignTo: &pb5,
								Text:     "H",
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
						Model: []string{"Grid with name, date, and size",
							"Grid with name and date",
							"Grid with name and size",
							"Grid with name only",
							"Grid with no text",
							"Frameless, variable size"},
						OnCurrentIndexChanged: svr.setLayoutMode,
					},
					HSpacer{
						Size: 10,
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
						MaxSize:        Size{300, 0},
						OnValueChanged: svr.setItemSize,
					},
					HSpacer{},
					CheckBox{
						AssignTo:         &svr.cbCached,
						Text:             "Cached",
						ColumnSpan:       1,
						OnCheckedChanged: svr.setCacheMode,
					},
				},
			},
		},
	}.Create(bldr))

	svr.canvasView.MouseDown().Attach(svr.OnMouseDown)
	svr.canvasView.MouseMove().Attach(svr.OnMouseMove)
	svr.canvasView.MouseUp().Attach(svr.OnMouseUp)
	svr.scrollview.SizeChanged().Attach(svr.onSizeChanged)
	svr.scroller.ValueChanged().Attach(svr.OnScrollerValueChanged)

	parent.SizeChanged().Attach(svr.onSizeParentChanged)

	br, _ := walk.NewSolidColorBrush(walk.RGB(20, 20, 20))
	svr.canvasView.SetBackground(br)
	svr.scrollview.Parent().SetBackground(br)

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
func (sv *ScrollViewer) destroy() {
	sv.closeDrawers()
	sv.directoryMonitor.Close()
	sv.imageProcessor.Close(sv)
	sv.scrollview.SetVisible(false)

	p := sv.scrollview.Parent().AsContainerBase().Children()
	i := p.Index(sv.scrollview)
	p.RemoveAt(i)

	sv.scrollview.Dispose()

	if sv.cvsCntr != nil {
		sv.cvsCntr.Dispose()
	}
	if sv.bmpCntr != nil {
		sv.bmpCntr.Dispose()
	}
	if sv.previewBackground != nil {
		sv.previewBackground.Dispose()
	}

	for k, _ := range sv.ItemsMap {
		delete(sv.ItemsMap, k)
	}
}

func (sv *ScrollViewer) Run(dirPath string, itemsModel *FileInfoModel, watchThisPath bool) {
	if itemsModel == nil {
		sv.itemsModel.BrowsePath(dirPath, true)

		log.Println("internal sv.itemsModel.items")
	} else {
		sv.itemsModel = itemsModel
		log.Println("external sv.itemsModel.items")
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
			//vmap.Type = vlist.Type
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

	sv.viewInfo.initSLBuffer = false

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

func (sv *ScrollViewer) OnButtonDown(x, y int, button walk.MouseButton) {
	sv.scrollControl.DoScroll(sv.SetScrollPosBy, -sv.itemHeight/2)
}
func (sv *ScrollViewer) OnButtonDown2(x, y int, button walk.MouseButton) {
	sv.scrollControl.DoScroll(sv.SetScrollPosBy, sv.itemHeight/2)
}
func (sv *ScrollViewer) OnButtonUp(x, y int, button walk.MouseButton) {
	sv.scrollControl.EndScroll()
}
func (sv *ScrollViewer) oncanvasViewpaint(canvas *walk.Canvas, updaterect walk.Rectangle) error {
	return nil
}
func (sv *ScrollViewer) OnMouseMove(x, y int, button walk.MouseButton) {
	//perform mouse move
	if button == walk.LeftButton && sv.PreviewRect == nil {
		sv.viewInfo.mousemove = sv.viewInfo.mousepos - y

		if sv.scroller.Value() != (sv.viewInfo.scrollpos + sv.viewInfo.mousemove) {
			sv.scroller.SetValue(sv.viewInfo.scrollpos + sv.viewInfo.mousemove)
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

func (sv *ScrollViewer) OnMouseDown(x, y int, button walk.MouseButton) {

	//mouseup does not give this
	sv.lastButtonDown = button
	//perform selection
	idx := 0
	if sv.GetLayoutMode() == 5 {
		idx = sv.GetItemAtScreenNB2(x, y)
	} else {
		idx = sv.GetItemAtScreen(x, y)
	}
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
		sv.viewInfo.mousepos = y
		sv.viewInfo.scrollpos = sv.viewInfo.topPos
	}

	sv.scroller.SetFocus()
	sv.Repaint()
}
func (sv *ScrollViewer) OnMouseUp(x, y int, button walk.MouseButton) {
	//do not continue if there is
	//already a preview on screen
	if sv.PreviewRect != nil {
		return
	}

	//reset movement vars
	sv.viewInfo.mousemove = 0
	sv.viewInfo.mousepos = 0
	sv.viewInfo.scrollpos = 0

	//Display image preview
	if sv.lastButtonDown == walk.RightButton && !sv.SuspendPreview {
		sv.PreviewItemAtScreen(x, y)
	}
}

func (sv *ScrollViewer) onSizeParentChanged() {
	//manage scrollviewer object placement
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
	sv.scrollview.SetSuspended(true)
	sv.canvasView.SetSuspended(true)
	sv.optionsPanel.SetBounds(walk.Rectangle{0, 0, sv.Width() - 28, 30})
	sv.canvasView.SetBounds(walk.Rectangle{0, 30, sv.Width() - 28, sv.Height() - 30})

	sv.scroller.SetBounds(walk.Rectangle{sv.Width() - 34, 28, 36, sv.Height() - 56})
	sv.pb1.SetBounds(walk.Rectangle{sv.Width() - 28, 0, 28, 28})
	sv.pb2.SetBounds(walk.Rectangle{sv.Width() - 28, sv.Height() - 28, 28, 28})

	sv.recalc()
	sv.scrollview.SetSuspended(false)
	sv.canvasView.SetSuspended(false)
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
	//"Grid with name, date, and size",
	//"Grid with name and date",
	//"Grid with name and size",
	//"Grid with name only",
	//"Grid with no text",
	//"Frameless, variable size"
	sv.currentLayout = sv.cmbMode.CurrentIndex()

	if sv.cmbMode.CurrentIndex() == 5 {
		//set current itemSize to new val
		sv.itemSize.mx = 0
		sv.itemSize.my = 0
		sv.itemSize.txth = 0
	} else {
		sv.itemSize.mx = 10
		sv.itemSize.my = 10
	}

	switch sv.cmbMode.CurrentIndex() {
	case 0:
		sv.evPaint = RedrawScreenNB
		sv.viewInfo.showName = true
		sv.viewInfo.showDate = true
		sv.viewInfo.showInfo = true
		sv.itemSize.txth = 3 * 16
	case 1:
		sv.evPaint = RedrawScreenNB
		sv.viewInfo.showName = true
		sv.viewInfo.showDate = true
		sv.viewInfo.showInfo = false
		sv.itemSize.txth = 2 * 17
	case 2:
		sv.evPaint = RedrawScreenNB
		sv.viewInfo.showName = true
		sv.viewInfo.showDate = false
		sv.viewInfo.showInfo = true
		sv.itemSize.txth = 2 * 17
	case 3:
		sv.evPaint = RedrawScreenNB
		sv.viewInfo.showName = true
		sv.viewInfo.showDate = false
		sv.viewInfo.showInfo = false
		sv.itemSize.txth = 1 * 20
	case 4:
		sv.evPaint = RedrawScreenNB
		sv.viewInfo.showName = false
		sv.viewInfo.showDate = false
		sv.viewInfo.showInfo = false
		sv.itemSize.txth = 0
	case 5:
		sv.evPaint = RedrawScreenNB2
		sv.viewInfo.showName = false
		sv.viewInfo.showDate = false
		sv.viewInfo.showInfo = false
	}
	sv.itemWidth = sv.itemSize.twm()
	sv.itemHeight = sv.itemSize.thm()
	sv.recalc()

	sv.Invalidate()
	defer sv.SetFocus()
}
func (sv *ScrollViewer) SetLayoutMode(idx int) {
	switch idx {
	case 0, 1, 2, 3, 4:
		sv.evPaint = RedrawScreenNB
		sv.cmbMode.SetCurrentIndex(idx)
	//case 1:
	//	sv.evPaint = RedrawScreenSLB
	//	sv.cmbMode.SetCurrentIndex(idx)
	case 5:
		sv.evPaint = RedrawScreenNB2
		sv.cmbMode.SetCurrentIndex(idx)
	}
}
func (sv *ScrollViewer) GetLayoutMode() int {
	return sv.currentLayout
}

func (sv *ScrollViewer) GetItemAtScreen(x int, y int) int {
	col := x / sv.itemWidth
	idx := -1

	if col < sv.NumCols() {
		row := int(float32(y+sv.viewInfo.topPos) / float32(sv.itemHeight))
		idx = col + row*sv.NumCols()
		if idx >= sv.itemsCount {
			idx = -1
		}
	}
	return idx
}

var rTestGetItemAtScreenNB2 *walk.Rectangle

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
func (sv *ScrollViewer) GetItemAtScreenNB2(xs, ys int) int {
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
				//wd, hd = getOptimalThumbSize(wmax-x, h, val.Width, val.Height)
				wd = wmax - x
			} else {
				x = 0
				y += h
			}
		}

		rItm := image.Rect(x, y, x+wd, y+h)
		pt := image.Point{xs, ys + sv.viewInfo.topPos} // - sv.viewInfo.topPos%h}

		if pt.In(rItm) {
			//			log.Println("i,", i, "x,", x, "y,", y, "xs,", xs, "ys,",
			//				ys,
			//				"ys+sv.viewInfo.topPos", ys+sv.viewInfo.topPos,
			//				"rItm y1-y2", walk.Point{rItm.Min.Y, rItm.Max.Y},
			//				"sv.viewInfo.topPos", sv.viewInfo.topPos,
			//				"sv.viewInfo.topPos%h", sv.viewInfo.topPos%h)

			//rTestGetItemAtScreenNB2 = &walk.Rectangle{rItm.Min.X, rItm.Min.Y - sv.viewInfo.topPos, rItm.Dx(), rItm.Dy()}
			return i
		}

		x += wd
	}
	return 0
}
func (sv *ScrollViewer) GetItemRectAtScreenNB2(xs, ys int) *image.Rectangle {

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
	idx := 0
	if fmt.Sprint(sv.evPaint) == fmt.Sprint(RedrawScreenNB2) {
		idx = sv.GetItemAtScreenNB2(x, y)
	} else {
		idx = sv.GetItemAtScreen(x, y)
	}

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
func (sv *ScrollViewer) recalc() int {
	h := sv.ViewHeight()
	switch sv.GetLayoutMode() {
	case 0, 1, 2, 3, 4:
		h = sv.NumRows() * sv.itemHeight
	case 5:
		h = sv.getTotalHeightNB2()
	}

	sv.scroller.SetRange(0, h-sv.ViewHeight())
	sv.scroller.SendMessage(win.WM_USER+21, 0, uintptr(sv.itemHeight*(sv.NumRowsVisible()-1))) //TBM_SETPAGESIZE
	sv.scroller.SendMessage(win.WM_USER+23, 0, uintptr(sv.itemHeight))                         //TBM_SETLINESIZE

	wstyle := win.GetWindowLong(sv.scroller.Handle(), win.GWL_STYLE)
	wstyle = wstyle | 0x0008 //0x0040
	win.SetWindowLong(sv.scroller.Handle(), win.GWL_STYLE, wstyle)

	sv.viewInfo.topPos = sv.scroller.Value()
	sv.viewInfo.numCols = sv.NumCols()
	sv.viewInfo.viewRows = sv.NumRowsVisible()
	sv.viewInfo.parentWidth = sv.Width()
	sv.viewInfo.initSLBuffer = false

	r := image.Rect(0, sv.viewInfo.topPos, sv.ViewWidth(), sv.viewInfo.topPos+sv.ViewHeight())
	sv.viewInfo.viewRect = r

	//log.Println("recalcSize ItemCount,ItemWidth,ItemHeight", sv.items, sv.itemWidth, sv.itemHeight)
	//	log.Println("recalcSize h,NumRows,NumCols", h, sv.NumRows(), sv.NumCols())
	//	log.Println("recalcSize", sv.scrollview.AsContainerBase().Bounds(), sv.canvasView.Bounds())
	return h
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
		sv.directoryMonitor.Close()
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
func (sv *ScrollViewer) SetScrollPosBy(val int) int {
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

	//r := image.Rect(0, val, sv.ViewWidth(), val+sv.Height())
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

	//	if sv.optionsPanel.Visible() {
	//		sv.optionsPanel.SetVisible(false)
	//	}
}

func (sv *ScrollViewer) SetItemSelected(index int) {
	if sv.SelectedIndex != index {
		sv.SelectedIndex = index
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

	//log.Println("Show: ", sv.scrollview.Bounds(), sv.canvasView.Bounds())
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

	//	if sv.optionsPanel.Visible() {
	//		sv.optionsPanel.SetVisible(false)
	//	}

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
