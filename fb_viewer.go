// Copyright 2011 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"image"
	//"image/draw"
	"log"
	//"strconv"
	//"time"
	//"os"
	"fmt"
	"math"
)

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	//"github.com/lxn/win"
)

type ImageViewWindow struct {
	*walk.MainWindow
	//toolbar *walk.CustomWidget
	topComposite *walk.Composite
	viewBase     *walk.CustomWidget

	lblAddr        *walk.Label
	cmbAddr        *walk.ComboBox
	btnOptions     *walk.PushButton
	menuItemAction *walk.Menu
	menuKeepLoc    *walk.Action
	ViewSlider     *walk.Slider
	prevFilePath   string
	imagelist      *FileInfoModel
	ImageName      string
	imageBuffer    *drawBuffer
}

func (mw *ImageViewWindow) onActionReload() {

}
func (mw *ImageViewWindow) onActionDelete() {

	//	fdelete := mw.thumbView.SelectedName()

	//	if fdelete != "" {
	//		if walk.MsgBox(mw, "Delete File", "Delete file "+fdelete,
	//			walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == win.IDYES {
	//			if err := os.Remove(mw.thumbView.SelectedNameFull()); err == nil {
	//				mw.thumbView.SelectedIndex = -1
	//			}
	//		}
	//	}
}
func (mw *ImageViewWindow) onActionRename() {

}
func (mw *ImageViewWindow) onActionCopyTo() {
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
func (mw *ImageViewWindow) onActionMoveTo() {
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

func (mw *ImageViewWindow) onActionKeepLoc() {
	mw.menuKeepLoc.SetChecked(!mw.menuKeepLoc.Checked())

	if !mw.menuKeepLoc.Checked() {
		mw.prevFilePath = ""
	}
}

func (mw *ImageViewWindow) OnToolbarClick(x, y int, button walk.MouseButton) {

}

func (mw *ImageViewWindow) onDrawPanelMouseDn(x, y int, button walk.MouseButton) {

}
func (mw *ImageViewWindow) onTableColClick(n int) {

}
func (mw *ImageViewWindow) onToolbarSizeChanged() {
	if mw.btnOptions != nil {
		mw.btnOptions.SetBounds(walk.Rectangle{mw.topComposite.Bounds().Width - 42, 7, 40, 28})
	}
}
func (mw *ImageViewWindow) getCurrentImageInfo() string {
	if mw.imagelist != nil {
		for i, v := range mw.imagelist.items {
			if mw.imagelist.getFullPath(i) == mw.ImageName {

				info0 := v.Name
				info1 := fmt.Sprintf("    %d x %d, %d KB", v.Width, v.Height, v.Size/1024)
				info2 := v.Modified.Format("    Jan 2, 2006 3:04pm")

				return info0 + info1 + info2
			}
		}
	}
	return ""
}

func (mw *ImageViewWindow) getNextImage() string {
	if mw.imagelist != nil {
		for i, _ := range mw.imagelist.items {
			if mw.imagelist.getFullPath(i) == mw.ImageName {
				if i+1 < len(mw.imagelist.items) {
					mw.ImageName = mw.imagelist.getFullPath(i + 1)
					return mw.imagelist.getFullPath(i + 1)
				}
			}
		}
	}
	return ""
}
func (mw *ImageViewWindow) getPrevImage() string {
	if mw.imagelist != nil {
		for i, _ := range mw.imagelist.items {
			if mw.imagelist.getFullPath(i) == mw.ImageName {
				if i-1 > 0 {
					mw.ImageName = mw.imagelist.getFullPath(i - 1)
					return mw.imagelist.getFullPath(i - 1)
				}
			}
		}
	}
	return ""
}

func NewImageViewWindow(parent *walk.MainWindow, imageName string, imglist *FileInfoModel) int {

	var tvw = new(ImageViewWindow)

	tvw.ImageName = imageName
	tvw.imagelist = imglist

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
		Name:     "ImageViewWindow",
		Title:    imageName,
		MinSize:  Size{400, 240},
		Size:     Size{800, 480},
		Layout:   VBox{Margins: Margins{Top: 0, Left: 0, Right: 0, Bottom: 0}, MarginsZero: true, SpacingZero: true},
		Children: []Widget{
			Composite{
				Layout:        Grid{Columns: 10, Margins: Margins{4, 4, 4, 2}, MarginsZero: true, SpacingZero: true},
				AssignTo:      &tvw.topComposite,
				MinSize:       Size{0, 32},
				MaxSize:       Size{0, 32},
				OnSizeChanged: tvw.onToolbarSizeChanged,
				Font:          myFont,
				Children: []Widget{
					ToolButton{Text: "X", OnClicked: func() {
						tvw.MainWindow.Close()
					}},
					ToolButton{
						Text:        "[=]",
						ToolTipText: "Toggle fullscreen",
						OnClicked: func() {
							tvw.MainWindow.SetFullscreen(!tvw.MainWindow.Fullscreen())
						}},

					HSpacer{},
					ToolButton{Text: "<-", OnClicked: tvw.onImgNext, ToolTipText: "View previous image"},
					ToolButton{Text: "->", OnClicked: tvw.onImgPrev, ToolTipText: "View next image"},
					HSpacer{},
					ToolButton{Text: "+", OnClicked: tvw.onZoomInc},
					ToolButton{Text: "-", OnClicked: tvw.onZoomDec},
					ComboBox{
						AssignTo: &tvw.cmbAddr,
						Editable: false,
						Model: []string{
							"Fit display",
							"Actual size",
							"Zoom in 1.25x",
							"Zoom in 1.50x",
							"Zoom in 2x",
							"Zoom in 3x",
							"Zoom in 4x",
							"Zoom in 5x",
							"Zoom in 10x",
							"Zoom out 2x",
							"Zoom out 3x",
							"Zoom out 4x",
							"Zoom out 5x",
						},
						OnCurrentIndexChanged: tvw.onZoomChanged,
						MinSize:               Size{300, 0},
						MaxSize:               Size{400, 0},
					},
				},
			},
			CustomWidget{
				Name:          "imageviewBase",
				AssignTo:      &tvw.viewBase,
				Paint:         tvw.onPaint,
				OnSizeChanged: tvw.onSizeChanged,
				OnMouseDown:   tvw.onMouseDown,
				OnMouseMove:   tvw.onMouseMove,
				OnMouseUp:     tvw.onMouseUp,
			},
		},
	}.Create()); err != nil {
		return 0
	}

	tvw.viewBase.MouseWheel().Attach(tvw.onMouseWheel)

	sbr := tvw.StatusBar()
	sbr.SetVisible(true)

	sbi := walk.NewStatusBarItem()
	sbi.SetWidth(500)
	sbr.Items().Add(sbi)

	tvw.viewBase.SetCursor(walk.CursorSizeAll())

	//	tvw.btnOptions, _ = walk.NewPushButton(tvw.topComposite)
	//	tvw.btnOptions.SetText("   ")
	//	img, _ := walk.NewImageFromFile("./image/menu.png")
	//	tvw.btnOptions.SetImage(img)
	//	tvw.btnOptions.SetImageAboveText(true)
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

	br, _ := walk.NewSolidColorBrush(walk.RGB(40, 40, 40))
	tvw.topComposite.SetBackground(br)

	br, _ = walk.NewSolidColorBrush(walk.RGB(20, 20, 20))
	tvw.viewBase.SetBackground(br)
	tvw.StatusBar().SetBackground(br)
	tvw.MainWindow.SetBackground(br)

	tvw.viewBase.SetPaintMode(walk.PaintNoErase)
	tvw.viewBase.SetInvalidatesOnResize(true)

	ft, _ := walk.NewFont(tvw.MainWindow.Font().Family(), 10, walk.FontBold)
	tvw.StatusBar().SetFont(ft)

	tvw.cmbAddr.SetCurrentIndex(0)
	//tvw.MainWindow.SetFullscreen(true)
	tvw.viewBase.SetFocus()

	tvw.loadImageToBuffer(imageName)

	/*-----------------------------
	   START THE WINDOW MAIN LOOP
	------------------------------*/
	id := tvw.MainWindow.Run()

	DeleteDrawBuffer(tvw.imageBuffer)

	if err := settings.Save(); err != nil {
		log.Fatal(err)
	}
	log.Print("exit form...")
	return id
}

func (mw *ImageViewWindow) setStatusText() {
	cvs, _ := mw.StatusBar().CreateCanvas()

	cvs.FillRectangle(mw.StatusBar().Background(), mw.StatusBar().ClientBounds())
	cvs.DrawText(mw.getCurrentImageInfo(), mw.StatusBar().Font(), walk.RGB(200, 200, 200), mw.StatusBar().ClientBounds(),
		walk.TextCenter|walk.TextVCenter|walk.TextSingleLine)

	cvs.Dispose()

	cvs, _ = mw.topComposite.CreateCanvas()

	r := mw.cmbAddr.Bounds()
	r.X -= 140
	r.Width = 80
	cvs.FillRectangle(mw.topComposite.Background(), r)
	cvs.DrawText("Zoom: ", mw.topComposite.Font(), walk.RGB(210, 210, 210), r,
		walk.TextRight|walk.TextVCenter|walk.TextSingleLine)

	cvs.Dispose()
}
func (mw *ImageViewWindow) onImgNext() {
	if mw.imageBuffer != nil {
		fnext := mw.getNextImage()
		if fnext != "" {
			mw.loadImageToBuffer(fnext)
			mw.viewBase.Invalidate()
		}
	}
}
func (mw *ImageViewWindow) onImgPrev() {
	if mw.imageBuffer != nil {
		fprev := mw.getPrevImage()
		if fprev != "" {
			mw.loadImageToBuffer(fprev)
			mw.repaint()
		}
	}
}
func (mw *ImageViewWindow) onZoomInc() {
	if mw.imageBuffer != nil {
		if mw.imageBuffer.zoom == 0 {
			mw.imageBuffer.zoom = float64(mw.imageBuffer.zoomSize().Width) / float64(mw.imageBuffer.size.Width)
		}
		mw.imageBuffer.zoom += 0.25
		mw.repaint()
	}
}
func (mw *ImageViewWindow) onZoomDec() {
	if mw.imageBuffer != nil {
		if mw.imageBuffer.zoom-0.25 >= 0.50 {
			mw.imageBuffer.zoom -= 0.25

			mw.repaint()
		}
	}
}
func (mw *ImageViewWindow) onSizeChanged() {
	if mw.imageBuffer != nil {
		mw.imageBuffer.viewinfo.viewRect = image.Rect(0, 0, mw.viewBase.Width(), mw.viewBase.Height())
	}
}

func (mw *ImageViewWindow) onMouseWheel(x, y int, btn walk.MouseButton) {

	// scroll direction
	d := int(int32(btn) >> 16)
	b := int(int32(btn) & 0xFFFF)

	//log.Println(b, int(int32(btn)>>16), int(int32(btn)&0xFFFF))

	if mw.imageBuffer != nil {
		if d < 0 {
			if b == int(walk.RightButton) {
				mw.onZoomInc()
			} else {
				mw.onImgNext()
			}
		} else {
			if b == int(walk.RightButton) {
				mw.onZoomDec()
			} else {
				mw.onImgPrev()
			}
		}
	}
}

func (mw *ImageViewWindow) onMouseDown(x, y int, btn walk.MouseButton) {
	if mw.imageBuffer != nil {
		if btn == walk.LeftButton {
			mw.imageBuffer.viewinfo.mouseposX = x
			mw.imageBuffer.viewinfo.mouseposY = y
		}
	}
	mw.viewBase.SetFocus()
}
func (mw *ImageViewWindow) onMouseMove(x, y int, btn walk.MouseButton) {
	if mw.imageBuffer != nil {
		if btn == walk.LeftButton && mw.imageBuffer.canPan() {
			vi := &mw.imageBuffer.viewinfo

			if vi.currentPos != nil {
				if x-vi.mouseposX > 0 {
					//prevent L-R panning
					if vi.currentPos.X > 0 {
						return
					}
				} else {
					//prevent R-L panning
					if vi.currentPos.X+mw.imageBuffer.zoomSize().Width < vi.viewRect.Dx() {
						return
					}
				}
				if y-vi.mouseposY > 0 {
					//prevent U-D panning
					if vi.currentPos.Y > 0 {
						return
					}
				} else {
					//prevent D-U panning
					if vi.currentPos.Y+mw.imageBuffer.zoomSize().Height < vi.viewRect.Dy() {
						return
					}
				}
			}

			vi.mousemoveX = x - vi.mouseposX
			vi.mousemoveY = y - vi.mouseposY

			mw.repaint()
		}
	}
}
func (mw *ImageViewWindow) onMouseUp(x, y int, btn walk.MouseButton) {
	if mw.imageBuffer != nil {
		if mw.imageBuffer.canPan() {
			vi := &mw.imageBuffer.viewinfo

			vi.offsetX += int(math.Ceil(float64(vi.mousemoveX) / mw.imageBuffer.zoom))
			vi.offsetY += int(math.Ceil(float64(vi.mousemoveY) / mw.imageBuffer.zoom))

			vi.mousemoveX = 0
			vi.mousemoveY = 0
			vi.mouseposX = 0
			vi.mouseposY = 0

			mw.repaint()
		}
	}
}

func (mw *ImageViewWindow) onPaint(canvas *walk.Canvas, updateRect walk.Rectangle) error {

	DrawImage(mw.ImageName, mw.imageBuffer, canvas)

	mw.setStatusText()
	return nil
}
func (mw *ImageViewWindow) repaint() {
	canvas, _ := mw.viewBase.CreateCanvas()

	DrawImage(mw.ImageName, mw.imageBuffer, canvas)

	canvas.Dispose()

	mw.setStatusText()
}

func (mw *ImageViewWindow) onZoomChanged() {
	//							"Actual size",
	//							"Fit display",
	//							"Zoom 1.25x",
	//							"Zoom 1.50x",
	//							"Zoom 2x",
	//							"Zoom 3x",
	//							"Zoom 4x",
	//							"Zoom 5x",
	//							"Zoom 10x",
	//							"Zoom out 2x",
	//							"Zoom out 3x",
	//							"Zoom out 4x",
	//							"Zoom out 5x",
	if mw.imageBuffer == nil {
		return
	}

	switch mw.cmbAddr.CurrentIndex() {
	case 0:
		mw.imageBuffer.zoom = 0
	case 1:
		mw.imageBuffer.zoom = 1.0
	case 2:
		mw.imageBuffer.zoom = 1.25
	case 3:
		mw.imageBuffer.zoom = 1.50
	case 4:
		mw.imageBuffer.zoom = 2.0
	case 5:
		mw.imageBuffer.zoom = 3.0
	case 6:
		mw.imageBuffer.zoom = 4.0
	case 7:
		mw.imageBuffer.zoom = 5.0
	case 8:
		mw.imageBuffer.zoom = 10.0
	case 9:
		mw.imageBuffer.zoom = 0.5
	case 10:
		mw.imageBuffer.zoom = 0.33
	case 11:
		mw.imageBuffer.zoom = 0.25
	case 12:
		mw.imageBuffer.zoom = 0.20
	}

	if mw.imageBuffer.canPan() {
		mw.viewBase.SetCursor(walk.CursorSizeAll())
	} else {
		mw.viewBase.SetCursor(walk.CursorArrow())
	}
	mw.viewBase.Invalidate()
}

func (mw *ImageViewWindow) loadImageToBuffer(imgName string) bool {
	//-----------------------------------------
	// load full size image to draw buffer
	//-----------------------------------------
	imgSize, _ := GetImageInfo(imgName)
	w, h := imgSize.Width, imgSize.Height

	img := processImageData(nil, imgName, false, &walk.Size{w, h})

	if img != nil {
		if mw.imageBuffer == nil {
			mw.imageBuffer = NewDrawBuffer(w, h)
		} else {
			DeleteDrawBuffer(mw.imageBuffer)
			mw.imageBuffer = NewDrawBuffer(w, h)
		}
		drawImageRGBAToDIB(nil, img, mw.imageBuffer, 0, 0, w, h)

		mw.imageBuffer.viewinfo.viewRect = image.Rect(0, 0, mw.viewBase.Width(), mw.viewBase.Height())

		mw.setStatusText()
	}
	return true
}
