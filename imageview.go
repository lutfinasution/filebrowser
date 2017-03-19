package main

import (
	"fmt"
	"image"
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

	OnViewImage func(idx int)
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

func (imv *ImageViewer) getCurrentImageInfo() string {
	if imv.imagelist == nil {
		return ""
	}

	for i, v := range imv.imagelist.items {

		if imv.imagelist.getFullPath(i) == imv.ImageName {
			info0 := v.Name
			info1 := fmt.Sprintf("    %d x %d, %d KB", v.Width, v.Height, v.Size/1024)
			info2 := v.Modified.Format("    Jan 2, 2006 3:04pm")
			info3 := ""
			if imv.imageBuffer != nil {
				if imv.imageBuffer.zoom == 0 {
					info3 = "   (fit display)"
				} else {
					info3 = fmt.Sprintf("   (%6.2fX zoom)", imv.imageBuffer.zoom)
				}
			}
			return info0 + info1 + info2 + info3
		}
	}

	return ""
}

func (imv *ImageViewer) getNextImage() string {
	if imv.imagelist == nil {
		return ""
	}

	for i, _ := range imv.imagelist.items {
		if imv.imagelist.getFullPath(i) == imv.ImageName {
			if i+1 < len(imv.imagelist.items) {
				if imv.OnViewImage != nil {
					imv.OnViewImage(i + 1)
				}
				imv.ImageName = imv.imagelist.getFullPath(i + 1)
				return imv.ImageName
			}
		}
	}

	return ""
}
func (imv *ImageViewer) getPrevImage() string {
	if imv.imagelist == nil {
		return ""
	}

	for i, _ := range imv.imagelist.items {
		if imv.imagelist.getFullPath(i) == imv.ImageName {
			if i-1 > 0 {
				if imv.OnViewImage != nil {
					imv.OnViewImage(i - 1)
				}

				imv.ImageName = imv.imagelist.getFullPath(i - 1)
				return imv.ImageName
			}
		}
	}

	return ""
}

func (imv *ImageViewer) setStatusText() {
	sb := imv.MainWindow.StatusBar()

	if sb.Visible() {
		cvs, _ := sb.CreateCanvas()

		cvs.FillRectangle(sb.Background(), sb.ClientBounds())

		stxt := imv.getCurrentImageInfo()
		cvs.DrawText(stxt, sb.Font(), walk.RGB(200, 200, 200), sb.ClientBounds(),
			walk.TextCenter|walk.TextVCenter|walk.TextSingleLine)

		cvs.Dispose()
	}

	cvs, _ := imv.topComposite.CreateCanvas()

	r := imv.cmbZoom.Bounds()
	r.X -= 140
	r.Width = 80
	cvs.FillRectangle(imv.topComposite.Background(), r)
	cvs.DrawText("Zoom: ", imv.topComposite.Font(), walk.RGB(210, 210, 210), r,
		walk.TextRight|walk.TextVCenter|walk.TextSingleLine)

	cvs.Dispose()
}
func (imv *ImageViewer) onImgNext() {
	if imv.imageBuffer != nil {
		fnext := imv.getNextImage()
		if fnext != "" {
			imv.loadImageToBuffer(fnext)
			imv.repaint()
			imv.viewBase.SetFocus()
		}
	}
}
func (imv *ImageViewer) onImgPrev() {
	if imv.imageBuffer != nil {
		fprev := imv.getPrevImage()
		if fprev != "" {
			imv.loadImageToBuffer(fprev)
			imv.repaint()
			imv.viewBase.SetFocus()
		}
	}
}
func (imv *ImageViewer) onZoomInc() {
	if imv.imageBuffer != nil {
		if imv.imageBuffer.zoom == 0 {
			zoomNow := float64(imv.imageBuffer.zoomSize().Width) / float64(imv.imageBuffer.size.Width)
			imv.imageBuffer.zoom = 0.25 * math.Ceil((100*zoomNow)/(100*0.25))
		} else {
			imv.imageBuffer.zoom += 0.25
		}
		imv.repaint()

		imv.cmbZoom.SetCurrentIndex(-1)
		imv.setStatusText()
		imv.viewBase.SetFocus()
	}
}
func (imv *ImageViewer) onZoomDec() {
	if imv.imageBuffer != nil {
		newZoom := imv.imageBuffer.zoom - 0.25
		sizeZoom := imv.imageBuffer.zoomSizeAt(newZoom)
		sizeFit := imv.imageBuffer.fitSize()

		if sizeZoom.Width > sizeFit.Width || sizeZoom.Height > sizeFit.Height {
			imv.imageBuffer.zoom -= 0.25

		} else {
			imv.imageBuffer.zoom = 0
		}

		imv.repaint()
		imv.cmbZoom.SetCurrentIndex(-1)
		imv.setStatusText()
		imv.viewBase.SetFocus()
	}
}
func (imv *ImageViewer) onSizeChanged() {
	if imv.imageBuffer != nil {
		imv.imageBuffer.viewinfo.viewRect = image.Rect(0, 0, imv.viewBase.Width(), imv.viewBase.Height())
	}
}

func (imv *ImageViewer) onMouseWheel(x, y int, btn walk.MouseButton) {

	// scroll direction
	d := int(int32(btn) >> 16)
	b := int(int32(btn) & 0xFFFF)

	if imv.imageBuffer != nil {
		if d < 0 {
			if b == int(walk.RightButton) {
				imv.onZoomInc()
			} else {
				imv.onImgNext()
			}
		} else {
			if b == int(walk.RightButton) {
				imv.onZoomDec()
			} else {
				imv.onImgPrev()
			}
		}
	}
}

func (imv *ImageViewer) onMouseDown(x, y int, btn walk.MouseButton) {
	if imv.imageBuffer != nil {
		if btn == walk.LeftButton {
			imv.imageBuffer.viewinfo.mouseposX = x
			imv.imageBuffer.viewinfo.mouseposY = y
		}
	}
	imv.viewBase.SetFocus()
}
func (imv *ImageViewer) onMouseMove(x, y int, btn walk.MouseButton) {

	if imv.imageBuffer == nil || btn != walk.LeftButton {
		return
	}

	vi := &imv.imageBuffer.viewinfo

	if imv.imageBuffer.canPan() == false || vi.currentPos == nil {
		return
	}

	canMove := false

	if imv.imageBuffer.canPanX() {
		if x-vi.mouseposX > 0 {
			//prevent L-R panning
			if vi.currentPos.X > 0 {
				return
			}
		} else {
			//prevent R-L panning
			if vi.currentPos.X+imv.imageBuffer.zoomSize().Width < vi.viewRect.Dx() {
				return
			}
		}
		canMove = true
		vi.mousemoveX = x - vi.mouseposX
	}
	if imv.imageBuffer.canPanY() {
		if y-vi.mouseposY > 0 {
			//prevent U-D panning
			if vi.currentPos.Y > 0 {
				return
			}
		} else {
			//prevent D-U panning
			if vi.currentPos.Y+imv.imageBuffer.zoomSize().Height < vi.viewRect.Dy() {
				return
			}
		}
		canMove = true
		vi.mousemoveY = y - vi.mouseposY
	}

	if canMove {
		imv.repaint()
	}
}
func (imv *ImageViewer) onMouseUp(x, y int, btn walk.MouseButton) {
	if imv.imageBuffer != nil {
		if imv.imageBuffer.canPan() {
			vi := &imv.imageBuffer.viewinfo

			vi.offsetX += int(math.Ceil(float64(vi.mousemoveX) / imv.imageBuffer.zoom))
			vi.offsetY += int(math.Ceil(float64(vi.mousemoveY) / imv.imageBuffer.zoom))

			vi.mousemoveX = 0
			vi.mousemoveY = 0
			vi.mouseposX = 0
			vi.mouseposY = 0

			imv.repaint()
		}
	}
}

func (imv *ImageViewer) onPaint(canvas *walk.Canvas, updateRect walk.Rectangle) error {
	if imv.imageBuffer != nil {
		DrawImage(imv.ImageName, imv.imageBuffer, canvas)

		imv.setStatusText()
	}
	return nil
}
func (imv *ImageViewer) repaint() {
	if imv.imageBuffer != nil {
		canvas, _ := imv.viewBase.CreateCanvas()

		DrawImage(imv.ImageName, imv.imageBuffer, canvas)

		canvas.Dispose()

		imv.setStatusText()
	}
}

func (imv *ImageViewer) onZoomChanged() {
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
	if imv.imageBuffer == nil {
		return
	}

	switch imv.cmbZoom.CurrentIndex() {
	case 0:
		imv.imageBuffer.zoom = 0
	case 1:
		imv.imageBuffer.zoom = 1.0
	case 2:
		imv.imageBuffer.zoom = 1.25
	case 3:
		imv.imageBuffer.zoom = 1.50
	case 4:
		imv.imageBuffer.zoom = 2.0
	case 5:
		imv.imageBuffer.zoom = 3.0
	case 6:
		imv.imageBuffer.zoom = 4.0
	case 7:
		imv.imageBuffer.zoom = 5.0
	case 8:
		imv.imageBuffer.zoom = 10.0
	case 9:
		imv.imageBuffer.zoom = 0.5
	case 10:
		imv.imageBuffer.zoom = 0.33
	case 11:
		imv.imageBuffer.zoom = 0.25
	case 12:
		imv.imageBuffer.zoom = 0.20
	}

	imv.repaint()

	if imv.imageBuffer.canPan() {
		imv.viewBase.SetCursor(walk.CursorSizeAll())
	} else {
		imv.viewBase.SetCursor(walk.CursorArrow())
	}
	defer imv.viewBase.SetFocus()
}

func (imv *ImageViewer) loadImageToBuffer(imgName string) bool {
	//-----------------------------------------
	// load full size image to draw buffer
	//-----------------------------------------
	imgSize, _ := GetImageInfo(imgName)
	w, h := imgSize.Width, imgSize.Height

	img := processImageData(nil, imgName, false, &walk.Size{w, h})

	if img != nil {
		if imv.imageBuffer == nil {
			imv.imageBuffer = NewDrawBuffer(w, h)
		} else {
			lastzoom := imv.imageBuffer.zoom

			DeleteDrawBuffer(imv.imageBuffer)
			imv.imageBuffer = NewDrawBuffer(w, h)

			imv.imageBuffer.zoom = lastzoom
		}
		drawImageRGBAToDIB(nil, img, imv.imageBuffer, 0, 0, w, h)

		imv.imageBuffer.viewinfo.viewRect = image.Rect(0, 0, imv.viewBase.Width(), imv.viewBase.Height())

		imv.setStatusText()
	}
	return true
}

func (imv *ImageViewer) Close() bool {

	DeleteDrawBuffer(imv.imageBuffer)

	return true
}
