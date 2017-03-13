package main

import (
	"fmt"
	"image"
	//"log"
	"math"
)

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	//"github.com/lxn/win"
)

type ImageViewer struct {
	*walk.Composite
	MainWindow   *walk.MainWindow
	topComposite *walk.Composite
	viewBase     *walk.CustomWidget

	lblAddr *walk.Label
	cmbZoom *walk.ComboBox

	ImageName   string
	imagelist   *FileInfoModel
	imageBuffer *drawBuffer
}

func NewImageViewer(mainWindow *walk.MainWindow, parent walk.Container, imageName string, imgList *FileInfoModel) (*ImageViewer, error) {
	w := new(ImageViewer)
	w.Composite, _ = walk.NewComposite(parent)
	w.MainWindow = mainWindow
	w.ImageName = imageName
	w.imagelist = imgList

	myFont := *new(Font)
	myFont.PointSize = 10

	bldr := NewBuilder(w.Parent())

	if err := (Composite{
		AssignTo: &w.Composite,
		Name:     "ImageViewComposite",
		Layout:   VBox{Margins: Margins{Top: 0, Left: 0, Right: 0, Bottom: 0}, MarginsZero: true, SpacingZero: true},
		Children: []Widget{
			CustomWidget{
				Name:          "imageviewBase",
				AssignTo:      &w.viewBase,
				Paint:         w.onPaint,
				OnSizeChanged: w.onSizeChanged,
				OnMouseDown:   w.onMouseDown,
				OnMouseMove:   w.onMouseMove,
				OnMouseUp:     w.onMouseUp,
			},
			Composite{
				Layout:   Grid{Columns: 10, Margins: Margins{4, 4, 4, 2}, MarginsZero: true, SpacingZero: true},
				AssignTo: &w.topComposite,
				MinSize:  Size{0, 32},
				MaxSize:  Size{0, 32},
				Font:     myFont,
				Children: []Widget{
					ToolButton{Text: "X", OnClicked: func() {
						w.MainWindow.Close()
					}},
					ToolButton{
						Text:        "[_]",
						ToolTipText: "Toggle fullscreen",
						OnClicked: func() {
							w.MainWindow.SetFullscreen(!w.MainWindow.Fullscreen())
						}},
					HSpacer{},
					ToolButton{Text: "<-|", OnClicked: w.onImgNext, ToolTipText: "View previous image"},
					ToolButton{Text: "|->", OnClicked: w.onImgPrev, ToolTipText: "View next image"},
					HSpacer{},
					ToolButton{Text: "+", OnClicked: w.onZoomInc},
					ToolButton{Text: "-", OnClicked: w.onZoomDec},
					ComboBox{
						AssignTo: &w.cmbZoom,
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
						OnCurrentIndexChanged: w.onZoomChanged,
						MinSize:               Size{150, 0},
						MaxSize:               Size{200, 0},
					},
				},
			},
		},
	}.Create(bldr)); err != nil {
		return nil, err
	}

	w.viewBase.MouseWheel().Attach(w.onMouseWheel)

	br, _ := walk.NewSolidColorBrush(walk.RGB(40, 40, 40))
	w.topComposite.SetBackground(br)

	br, _ = walk.NewSolidColorBrush(walk.RGB(20, 20, 20))
	w.viewBase.SetBackground(br)
	w.viewBase.SetPaintMode(walk.PaintNoErase)
	w.viewBase.SetInvalidatesOnResize(true)

	w.loadImageToBuffer(imageName)

	w.cmbZoom.SetCurrentIndex(0)

	return w, nil
}

func (mw *ImageViewer) getCurrentImageInfo() string {
	if mw.imagelist == nil {
		return ""
	}

	for i, v := range mw.imagelist.items {
		if mw.imagelist.getFullPath(i) == mw.ImageName {

			info0 := v.Name
			info1 := fmt.Sprintf("    %d x %d, %d KB", v.Width, v.Height, v.Size/1024)
			info2 := v.Modified.Format("    Jan 2, 2006 3:04pm")
			info3 := ""
			if mw.imageBuffer != nil {
				if mw.imageBuffer.zoom == 0 {
					info3 = "   (fit display)"
				} else {
					info3 = fmt.Sprintf("   (%6.2fX zoom)", mw.imageBuffer.zoom)
				}
			}
			return info0 + info1 + info2 + info3
		}
	}

	return ""
}

func (mw *ImageViewer) getNextImage() string {
	if mw.imagelist == nil {
		return ""
	}

	for i, _ := range mw.imagelist.items {
		if mw.imagelist.getFullPath(i) == mw.ImageName {
			if i+1 < len(mw.imagelist.items) {
				mw.ImageName = mw.imagelist.getFullPath(i + 1)
				return mw.imagelist.getFullPath(i + 1)
			}
		}
	}

	return ""
}
func (mw *ImageViewer) getPrevImage() string {
	if mw.imagelist == nil {
		return ""
	}

	for i, _ := range mw.imagelist.items {
		if mw.imagelist.getFullPath(i) == mw.ImageName {
			if i-1 > 0 {
				mw.ImageName = mw.imagelist.getFullPath(i - 1)
				return mw.imagelist.getFullPath(i - 1)
			}
		}
	}

	return ""
}

func (mw *ImageViewer) setStatusText() {
	sb := mw.MainWindow.StatusBar()

	if sb.Visible() {
		cvs, _ := sb.CreateCanvas()

		cvs.FillRectangle(sb.Background(), sb.ClientBounds())

		stxt := mw.getCurrentImageInfo()
		cvs.DrawText(stxt, sb.Font(), walk.RGB(200, 200, 200), sb.ClientBounds(),
			walk.TextCenter|walk.TextVCenter|walk.TextSingleLine)

		cvs.Dispose()
	}

	cvs, _ := mw.topComposite.CreateCanvas()

	r := mw.cmbZoom.Bounds()
	r.X -= 140
	r.Width = 80
	cvs.FillRectangle(mw.topComposite.Background(), r)
	cvs.DrawText("Zoom: ", mw.topComposite.Font(), walk.RGB(210, 210, 210), r,
		walk.TextRight|walk.TextVCenter|walk.TextSingleLine)

	cvs.Dispose()
}
func (mw *ImageViewer) onImgNext() {
	if mw.imageBuffer != nil {
		fnext := mw.getNextImage()
		if fnext != "" {
			mw.loadImageToBuffer(fnext)
			mw.repaint()
			mw.viewBase.SetFocus()
		}
	}
}
func (mw *ImageViewer) onImgPrev() {
	if mw.imageBuffer != nil {
		fprev := mw.getPrevImage()
		if fprev != "" {
			mw.loadImageToBuffer(fprev)
			mw.repaint()
			mw.viewBase.SetFocus()
		}
	}
}
func (mw *ImageViewer) onZoomInc() {
	if mw.imageBuffer != nil {
		if mw.imageBuffer.zoom == 0 {
			zoomNow := float64(mw.imageBuffer.zoomSize().Width) / float64(mw.imageBuffer.size.Width)
			mw.imageBuffer.zoom = 0.25 * math.Ceil((100*zoomNow)/(100*0.25))
		} else {
			mw.imageBuffer.zoom += 0.25
		}
		mw.repaint()

		mw.cmbZoom.SetCurrentIndex(-1)
		mw.setStatusText()
		mw.viewBase.SetFocus()
	}
}
func (mw *ImageViewer) onZoomDec() {
	if mw.imageBuffer != nil {
		newZoom := mw.imageBuffer.zoom - 0.25
		sizeZoom := mw.imageBuffer.zoomSizeAt(newZoom)
		sizeFit := mw.imageBuffer.fitSize()

		if sizeZoom.Width > sizeFit.Width || sizeZoom.Height > sizeFit.Height {
			mw.imageBuffer.zoom -= 0.25

		} else {
			mw.imageBuffer.zoom = 0
		}

		mw.repaint()
		mw.cmbZoom.SetCurrentIndex(-1)
		mw.setStatusText()
		mw.viewBase.SetFocus()
	}
}
func (mw *ImageViewer) onSizeChanged() {
	if mw.imageBuffer != nil {
		mw.imageBuffer.viewinfo.viewRect = image.Rect(0, 0, mw.viewBase.Width(), mw.viewBase.Height())
	}
}

func (mw *ImageViewer) onMouseWheel(x, y int, btn walk.MouseButton) {

	// scroll direction
	d := int(int32(btn) >> 16)
	b := int(int32(btn) & 0xFFFF)

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

func (mw *ImageViewer) onMouseDown(x, y int, btn walk.MouseButton) {
	if mw.imageBuffer != nil {
		if btn == walk.LeftButton {
			mw.imageBuffer.viewinfo.mouseposX = x
			mw.imageBuffer.viewinfo.mouseposY = y
		}
	}
	mw.viewBase.SetFocus()
}
func (mw *ImageViewer) onMouseMove(x, y int, btn walk.MouseButton) {

	if mw.imageBuffer == nil || btn != walk.LeftButton {
		return
	}

	vi := &mw.imageBuffer.viewinfo

	if mw.imageBuffer.canPan() == false || vi.currentPos == nil {
		return
	}

	canMove := false

	if mw.imageBuffer.canPanX() {
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
		canMove = true
		vi.mousemoveX = x - vi.mouseposX
	}
	if mw.imageBuffer.canPanY() {
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
		canMove = true
		vi.mousemoveY = y - vi.mouseposY
	}

	if canMove {
		mw.repaint()
	}
}
func (mw *ImageViewer) onMouseUp(x, y int, btn walk.MouseButton) {
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

func (mw *ImageViewer) onPaint(canvas *walk.Canvas, updateRect walk.Rectangle) error {
	if mw.imageBuffer != nil {
		DrawImage(mw.ImageName, mw.imageBuffer, canvas)

		mw.setStatusText()
	}
	return nil
}
func (mw *ImageViewer) repaint() {
	if mw.imageBuffer != nil {
		canvas, _ := mw.viewBase.CreateCanvas()

		DrawImage(mw.ImageName, mw.imageBuffer, canvas)

		canvas.Dispose()

		mw.setStatusText()
	}
}

func (mw *ImageViewer) onZoomChanged() {
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

	switch mw.cmbZoom.CurrentIndex() {
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

	mw.repaint()

	if mw.imageBuffer.canPan() {
		mw.viewBase.SetCursor(walk.CursorSizeAll())
	} else {
		mw.viewBase.SetCursor(walk.CursorArrow())
	}
	defer mw.viewBase.SetFocus()
}

func (mw *ImageViewer) loadImageToBuffer(imgName string) bool {
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
			lastzoom := mw.imageBuffer.zoom

			DeleteDrawBuffer(mw.imageBuffer)
			mw.imageBuffer = NewDrawBuffer(w, h)

			mw.imageBuffer.zoom = lastzoom
		}
		drawImageRGBAToDIB(nil, img, mw.imageBuffer, 0, 0, w, h)

		mw.imageBuffer.viewinfo.viewRect = image.Rect(0, 0, mw.viewBase.Width(), mw.viewBase.Height())

		mw.setStatusText()
	}
	return true
}

func (mw *ImageViewer) Close() bool {

	DeleteDrawBuffer(mw.imageBuffer)

	return true
}
