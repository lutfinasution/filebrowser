// Copyright 2017 MLN. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"

	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	//"runtime"
	//"strconv"
	//"sync"
	//"sync/atomic"
	"reflect"
	"time"
	"unsafe"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

import (
	"github.com/anthonynsimon/bild/transform"
	"github.com/lxn/walk"
	"github.com/lxn/win"
	"github.com/pixiv/go-libjpeg/jpeg"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	"golang.org/x/image/webp"
	//"golang.org/x/image/webp/nycbcra"
)

type ThumbSizes struct {
	tw, th int
	mx, my int
	txth   int
}

func (t ThumbSizes) twm() int {
	val := t.tw + 2*t.mx
	val += val % 2 //enforce even number
	return val
}

func (t ThumbSizes) thm() int {
	val := t.th + t.txth + 2*t.my
	val += val % 2 //enforce even number
	return val
}

type ProgresDrawer struct {
	progresWidget   *walk.WidgetBase
	progresWidth    int
	progresMaxValue int
}

func (pdraw *ProgresDrawer) DrawProgress(val int) {
	if pdraw.progresWidget != nil {
		pdraw.progresWidget.Synchronize(func() {
			cvs, _ := pdraw.progresWidget.CreateCanvas()
			brs, _ := walk.NewSolidColorBrush(walk.RGB(50, 120, 50))
			defer brs.Dispose()
			defer cvs.Dispose()

			val = int(math.Ceil(float64(pdraw.progresWidth) * float64(val) / float64(pdraw.progresMaxValue)))
			if val > pdraw.progresWidth {
				val = pdraw.progresWidth
			}
			r := walk.Rectangle{pdraw.progresWidget.Width() - pdraw.progresWidth - 30, 4, val, pdraw.progresWidget.Height() - 8}
			cvs.FillRectangle(brs, r)
		})
	}
}
func (pdraw *ProgresDrawer) Clear() {
	if pdraw.progresWidget != nil {
		pdraw.progresWidget.Synchronize(func() {
			pdraw.progresWidget.Invalidate()
		})
	}
}
func NewProgresDrawer(progresWidget *walk.WidgetBase, maxwidth, maxval int) *ProgresDrawer {
	if progresWidget != nil {
		pd := ProgresDrawer{progresWidget, maxwidth, maxval}
		pd.Clear()
		return &pd
	} else {
		return nil
	}
}

//var thumbR = ThumbRect{120, 75, 10, 10, 48}
//var AppDontProcessChangedItems bool

//func doRenderTasker(done chan int, fnames []*jobList) bool {
//	res := true
//	icount := 0
//loop:
//	for _, v := range fnames {
//		select {
//		case <-done:
//			break loop
//		default:
//			v.mutex.Lock()
//			if v.inProgress {
//				v.mutex.Unlock()
//				continue
//			} else {
//				v.inProgress = true
//			}
//			v.mutex.Unlock()

//			//			if v.inProgress {
//			//				//log.Println("v.inProgress...")
//			//				continue
//			//			} else {
//			//				v.mutex.Lock()
//			//				v.inProgress = true
//			//				v.mutex.Unlock()
//			//			}

//			if ok := processImageData(v.name); ok {
//				c := atomic.AddInt64(&workCounter, 1)

//				if c%10 == 0 {
//					workStatus.DrawProgress(int(workCounter))
//				}
//			}
//			icount++
//		}
//	}
//	donewait.Done()
//	//log.Println("doRenderTasker", icount)
//	return res
//}

func getOptimalThumbSize(dstW, dstH, srcW, srcH int) (int, int) {
	getW := func(h, ws, hs int) int { return int(math.Ceil(float64(h) / float64(hs) * float64(ws))) }
	getH := func(w, ws, hs int) int { return int(math.Ceil(float64(w) / float64(ws) * float64(hs))) }

	w := 0
	h := 0
	if srcW > srcH {
		w = dstW
		h = getH(w, srcW, srcH)

		if h > dstH {
			h = dstH
			w = getW(h, srcW, srcH)
		}
	} else {
		h = dstH
		w = getW(h, srcW, srcH)
		if w > dstW {
			w = dstW
			h = getH(w, srcW, srcH)
		}
	}

	return w, h
}

func processImageData(sv *ScrollViewer, mkey string, createthumb bool, imgsize *walk.Size) *image.RGBA {

	v, ok := sv.ItemsMap[mkey]
	if !ok {
		return nil
	}
	//	if v.Changed {
	//		log.Println("processImageData/want to process ", mkey, doCache, v.HasData(), v.Changed)
	//	}
	//Skip thumb creation if ItemsMap already has data.
	//and cache=true
	//and changed=false
	if createthumb && sv.doCache && v.HasData() && !v.Changed {
		return nil
	}
	//log.Println("processImageData/processing: ", mkey)

	//open
	file, err := os.Open(mkey)
	if err != nil {
		return nil
	}
	defer file.Close()

	//set desired scaled size
	resW := sv.itemSize.tw
	resH := sv.itemSize.th

	if !createthumb {
		resW = imgsize.Width
		resH = imgsize.Height
	}

	var img image.Image

	//Retrieve image dimension, etc based on type
	switch imgType := filepath.Ext(mkey); imgType {
	case ".bmp":
		img, err = bmp.Decode(file)
		if err != nil {
			//log.Fatal(err)
			return nil
		}
	case ".gif":
		img, err = gif.Decode(file)
		if err != nil {
			log.Fatal(err)
		}
	case ".jpg", ".jpeg":
		jopt := jpeg.DecoderOptions{ScaleTarget: image.Rect(0, 0, resW, resH)}

		img, err = jpeg.DecodeIntoRGBA(file, &jopt)
		if err != nil {
			log.Fatal(err)
		}
	case ".png":
		img, err = png.Decode(file)
		if err != nil {
			//log.Fatal(err)
			return nil
		}
	case ".tif", ".tiff":
		img, err = tiff.Decode(file)
		if err != nil {
			log.Fatal(err)
		}
	case ".webp":
		img, err = webp.Decode(file)
		if err != nil {
			return nil
		}
	}

	if img == nil {
		return nil
	}

	if img.Bounds().Dx() < 8 {
		return nil
	}

	var w, h int
	var mt *image.RGBA
	//Further scaling ops req to fit the src img
	//to the desired display size.
	if createthumb {
		w, h = getOptimalThumbSize(sv.itemSize.tw, sv.itemSize.th, img.Bounds().Dx(), img.Bounds().Dy())
		mt = transform.Resize(img, w, h, transform.NearestNeighbor)
	} else {
		w, h = getOptimalThumbSize(imgsize.Width, imgsize.Height, img.Bounds().Dx(), img.Bounds().Dy())
		mt = transform.Resize(img, w, h, transform.MitchellNetravali)
	}

	if createthumb {
		//Encode the scaled image & save to cache map
		jept := jpeg.EncoderOptions{Quality: 75, OptimizeCoding: false, DCTMethod: jpeg.DCTIFast}
		buf := new(bytes.Buffer)

		err = jpeg.Encode(buf, mt, &jept)
		if err == nil {
			v.Imagedata = make([]byte, buf.Len())
			buf.Read(v.Imagedata)

			v.thumbW, v.thumbH = w, h
			v.Changed = false
		} else {
			return nil
		}
	}
	return mt
}

func renderImageBuffer(sv *ScrollViewer, mkey string, buf []byte, dst *image.RGBA, x int, y int,
	selected bool, doBorder bool, doCentered bool) (walk.Size, error) {

	imgsize := walk.Size{0, 0}

	//set & draw outer border effect
	r := dst.Bounds()

	//draw the inner rect
	if doBorder || selected {
		if !selected {
			draw.Draw(dst, dst.Bounds(), &image.Uniform{color.RGBA{30, 30, 30, 255}}, image.ZP, draw.Src)
			r = r.Inset(1)
		} else {
			draw.Draw(dst, dst.Bounds(), &image.Uniform{color.RGBA{200, 100, 100, 255}}, image.ZP, draw.Src)
			r = r.Inset(3)
		}
	}
	draw.Draw(dst, r, &image.Uniform{color.RGBA{0, 0, 0, 255}}, image.ZP, draw.Src)

	if buf == nil {
		return imgsize, nil
	}

	//decode
	//jopt := jpeg.DecoderOptions{ScaleTarget: image.Rect(0, 0, sv.itemSize.tw, sv.itemSize.th)}
	jopt := jpeg.DecoderOptions{DCTMethod: jpeg.DCTIFast, DisableFancyUpsampling: true, DisableBlockSmoothing: true}

	buff := bytes.NewBuffer(buf)
	img, err := jpeg.DecodeIntoRGBA(buff, &jopt)
	if err != nil {
		return imgsize, err
	}

	//Further scaling ops req to fit the src img
	//to the desired display size.
	w, h := getOptimalThumbSize(sv.itemSize.tw, sv.itemSize.th, img.Bounds().Dx(), img.Bounds().Dy())
	mt := img

	if (img.Bounds().Dx() != w) || (img.Bounds().Dy() != h) {
		mt = transform.Resize(img, w, h, transform.NearestNeighbor)

		if sv.handleChangedItems {
			sv.contentMonitor.submitChangedItem(mkey, sv.ItemsMap[mkey])
		}
	}
	imgsize.Width = mt.Bounds().Dx()
	imgsize.Height = mt.Bounds().Dy()

	//centers x,y
	if doCentered {
		resW := sv.itemSize.twm()
		resH := sv.itemSize.thm() - sv.itemSize.txth
		x += int(math.Ceil(float64(resW-w) / 2))
		y += int(math.Ceil(float64(resH-h) / 2))
	}
	draw.Draw(dst, image.Rect(x, y, x+w, y+h), mt, mt.Bounds().Min, draw.Src)

	return imgsize, err
}

func renderBorder(sv *ScrollViewer, dst *image.RGBA, xOffset, yOffset, w, h int) {

	var clr = color.RGBA{210, 100, 100, 150}
	dop := draw.Src

	r := dst.Bounds()
	//r.Min.X += xOffset
	r.Min.Y += yOffset
	r.Max.X = w
	//r.Max.Y = h

	r2 := r
	r2.Max.Y = 3
	draw.Draw(dst, r2, &image.Uniform{clr}, image.ZP, dop)
	r2 = r
	r2.Min.Y = r2.Max.Y - 3
	draw.Draw(dst, r2, &image.Uniform{clr}, image.ZP, dop)
	r2 = r
	r2.Max.X = 3
	draw.Draw(dst, r2, &image.Uniform{clr}, image.ZP, dop)
	r2 = r
	r2.Min.X = r2.Max.X - 3
	draw.Draw(dst, r2, &image.Uniform{clr}, image.ZP, dop)
}

func RenderImage(name string) (walk.Size, error) {
	//open
	w := walk.Size{0, 0}
	return w, nil
}

func GetImageInfo(name string) (walk.Size, error) {
	//open
	w := walk.Size{0, 0}

	file, err := os.Open(name)
	if err != nil {
		log.Fatal(err)
		return w, err
	}
	defer file.Close()

	imgType := filepath.Ext(name)
	var imgcfg image.Config

	//Retrieve image dimension, etc based on type
	switch imgType {
	case ".bmp":
		imgcfg, err = bmp.DecodeConfig(file)
		if err != nil {
			//log.Fatal(err)
			return w, err
		}
	case ".gif":
		imgcfg, err = gif.DecodeConfig(file)
		if err != nil {
			log.Fatal(err)
		}
	case ".jpg", ".jpeg":
		imgcfg, err = jpeg.DecodeConfig(file)
		if err != nil {
			log.Fatal(err)
		}
	case ".png":
		imgcfg, err = png.DecodeConfig(file)
		if err != nil {
			log.Fatal(err)
		}
	case ".tif", ".tiff":
		imgcfg, err = tiff.DecodeConfig(file)
		if err != nil {
			log.Fatal(err)
		}
	case ".webp":
		imgcfg, err = webp.DecodeConfig(file)
		if err != nil {
			//log.Fatal(err)
			log.Println("error decoding : ", err)
			return w, err
		}
	}

	w.Width = imgcfg.Width
	w.Height = imgcfg.Height

	return w, err
}

func renderScreenBufferA(sv *ScrollViewer, canvas *walk.Canvas, iStart int, iStop int, drawPosY int) (res int) {
	w := sv.itemSize.twm()
	h := sv.itemSize.thm()
	imgBase := image.NewRGBA(image.Rect(0, 0, w, h))
	k := 0
	res = 0
	for i := iStart; i < iStop; i++ {
		if i >= 0 {
			mkey := sv.itemsModel.getFullPath(i)
			if v, ok := sv.ItemsMap[mkey]; ok {
				buf := v.Imagedata

				_, err := renderImageBuffer(sv, mkey, buf, imgBase, 0, 0, sv.SelectedIndex == i, true, true)
				if err != nil {
					log.Println("renderImageBuffer error decoding : ", mkey, len(buf))
				}
				info1 := fmt.Sprintf("%d x %d", v.Width, v.Height)
				info2 := fmt.Sprintf("%d ", v.Size)
				drawtext(sv, []string{v.Name, info1, info2}, imgBase, imgBase.Bounds())

				col := i % sv.NumCols()
				row := k / sv.NumCols()

				drawImageRGBAToCanvas(sv, imgBase, canvas.HDC(), w*col, h*row+drawPosY, w, h)
				res++
			}
		}
		k++
	}
	return res
}
func createDrawDibsection(ws, hs int) (win.HBITMAP, unsafe.Pointer) {
	var bi win.BITMAPV5HEADER

	bi.BiSize = uint32(unsafe.Sizeof(bi))
	bi.BiWidth = int32(ws)
	bi.BiHeight = -int32(hs)
	bi.BiPlanes = 1
	bi.BiBitCount = 32
	bi.BiCompression = win.BI_BITFIELDS
	// The following mask specification specifies a supported 32 BPP
	// alpha format for Windows XP.
	bi.BV4RedMask = 0x00FF0000
	bi.BV4GreenMask = 0x0000FF00
	bi.BV4BlueMask = 0x000000FF
	bi.BV4AlphaMask = 0xFF000000

	var winDibPtr unsafe.Pointer
	hdc := win.GetDC(0)
	defer win.ReleaseDC(0, hdc)

	// Create the DIB section with an alpha channel.
	winDib := win.CreateDIBSection(hdc, &bi.BITMAPINFOHEADER, win.DIB_RGB_COLORS, &winDibPtr, 0, 0)

	switch winDib {
	case 0, win.ERROR_INVALID_PARAMETER:
		return 0, nil //newError("CreateDIBSection failed")
	}

	return winDib, winDibPtr
}
func drawImageRGBAToCanvas2(im *image.RGBA, cvs *walk.Canvas, winDib win.HBITMAP, winDibPtr unsafe.Pointer, xs, ys, ws, hs int) error {

	// Fill the image
	bitmap_array := (*[1 << 30]byte)(winDibPtr)

	y0 := 0
	x0 := 0
	y1 := im.Bounds().Max.Y
	x1 := im.Bounds().Max.X

	b := im.Pix
	is := 0
	i := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			bitmap_array[i+3] = b[is+3]
			bitmap_array[i+2] = b[is+0]
			bitmap_array[i+1] = b[is+1]
			bitmap_array[i+0] = b[is+2]
			is += 4
			i += 4
		}
		i += (1000 - x1) * 4
	}

	hdc := win.GetDC(0)
	defer win.ReleaseDC(0, hdc)

	winHDC := win.CreateCompatibleDC(hdc)
	winHDC0 := win.HDC(win.SelectObject(winHDC, win.HGDIOBJ(winDib)))

	win.BitBlt(cvs.HDC(), int32(xs), int32(ys), int32(ws), int32(hs), winHDC, 0, 0, win.SRCCOPY)

	win.SelectObject(winHDC0, win.HGDIOBJ(winDib))
	win.DeleteDC(winHDC)

	return nil
}
func drawImageRGBAToCanvas(sv *ScrollViewer, im *image.RGBA, cvsHDC win.HDC, xs, ys, ws, hs int) error {
	var bi win.BITMAPV5HEADER

	bi.BiSize = uint32(unsafe.Sizeof(bi))
	bi.BiWidth = int32(im.Bounds().Dx())
	bi.BiHeight = -int32(im.Bounds().Dy())
	bi.BiPlanes = 1
	bi.BiBitCount = 32
	bi.BiCompression = win.BI_BITFIELDS
	// The following mask specification specifies a supported 32 BPP
	// alpha format for Windows XP.
	bi.BV4RedMask = 0x00FF0000
	bi.BV4GreenMask = 0x0000FF00
	bi.BV4BlueMask = 0x000000FF
	bi.BV4AlphaMask = 0xFF000000

	var winDibPtr unsafe.Pointer
	hdc := win.GetDC(0)
	defer win.ReleaseDC(0, hdc)

	// Create the DIB section with an alpha channel.
	winDib := win.CreateDIBSection(hdc, &bi.BITMAPINFOHEADER, win.DIB_RGB_COLORS, &winDibPtr, 0, 0)

	switch winDib {
	case 0, win.ERROR_INVALID_PARAMETER:
		return nil //newError("CreateDIBSection failed")
	}
	defer win.DeleteObject(win.HGDIOBJ(winDib))

	// Fill the image

	bitmap_array := (*[1 << 30]byte)(unsafe.Pointer(winDibPtr))
	y1 := im.Bounds().Max.Y
	x1 := im.Bounds().Max.X
	b := im.Pix
	for i := 0; i < y1*x1*4; i += 4 {
		bitmap_array[i+3] = b[i+3]
		bitmap_array[i+2] = b[i+0]
		bitmap_array[i+1] = b[i+1]
		bitmap_array[i+0] = b[i+2]
	}

	sv.drawerMutex.Lock()
	//
	winHDC := win.CreateCompatibleDC(hdc)
	winHDC0 := win.HDC(win.SelectObject(winHDC, win.HGDIOBJ(winDib)))

	win.BitBlt(cvsHDC, int32(xs), int32(ys), int32(ws), int32(hs), winHDC, 0, 0, win.SRCCOPY)

	win.SelectObject(winHDC0, win.HGDIOBJ(winDib))
	win.DeleteDC(winHDC)
	//
	sv.drawerMutex.Unlock()

	return nil
}

func DrawPreview(sv *ScrollViewer, idx int) (rPreview *walk.Rectangle) {

	rPreview = nil
	w := int(float64(sv.ViewWidth()) * 0.75)
	h := int(float64(sv.ViewHeight()) * 0.75)

	img := processImageData(sv, sv.itemsModel.getFullPath(idx), false, &walk.Size{w, h})

	if img != nil {
		x := (sv.ViewWidth() - img.Bounds().Dx()) / 2
		y := (sv.ViewHeight() - img.Bounds().Dy() - 30) / 2

		cvs, _ := sv.canvasView.CreateCanvas()
		defer cvs.Dispose()

		//clear screen canvas
		sv.Repaint()

		//draw white frame rect
		br, _ := walk.NewSolidColorBrush(walk.RGB(255, 255, 255))
		defer br.Dispose()

		r := walk.Rectangle{x - 10, y - 10, img.Bounds().Dx() + 20, img.Bounds().Dy() + 50}

		//cache background area
		if sv.previewBackground != nil {
			sv.previewBackground.Dispose()
			sv.previewBackground = nil
		}
		sv.previewBackground, _ = walk.NewBitmap(walk.Size{r.Width, r.Height})
		cvb, _ := walk.NewCanvasFromImage(sv.previewBackground)
		defer cvb.Dispose()

		win.BitBlt(cvb.HDC(), 0, 0, int32(r.Width), int32(r.Height),
			cvs.HDC(), int32(r.X), int32(r.Y), win.SRCCOPY)

		cvs.FillRectangle(br, r)

		rPreview = &walk.Rectangle{x - 10, y - 10, img.Bounds().Dx() + 20, img.Bounds().Dy() + 50}

		v := sv.itemsModel.items[idx]
		info0 := v.Name
		info1 := fmt.Sprintf("%d x %d, %d KB", v.Width, v.Height, v.Size/1024)
		info2 := v.Modified.Format("Jan 2, 2006 3:04pm")

		//draw name bottom center
		r = walk.Rectangle{x - 6, y + img.Bounds().Dy() + 4, img.Bounds().Dx() + 12, 16}
		ft, _ := walk.NewFont(sv.canvasView.Font().Family(), 10, walk.FontBold)
		cvs.DrawText(info0, ft, walk.RGB(0, 0, 0), r, walk.TextCenter|walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis)
		ft.Dispose()

		//draw other info bottom center
		r.Y += 15
		ft, _ = walk.NewFont(sv.canvasView.Font().Family(), 10, 0)
		cvs.DrawText(info2+"   "+info1, ft, walk.RGB(0, 0, 0), r, walk.TextCenter|walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis)
		ft.Dispose()

		drawImageRGBAToCanvas(sv, img, cvs.HDC(), x, y, img.Bounds().Dx(), img.Bounds().Dy())
	}
	return rPreview
}

func abs(val int) int {
	if val < 0 {
		return -val
	} else {
		return val
	}
}
func RedrawScreenSLB(sv *ScrollViewer, canvas *walk.Canvas, updateBounds walk.Rectangle, viewbounds walk.Rectangle) error {

	var cleaner = func(area walk.Rectangle, offsetX int, offsetY int) {
		brush, _ := walk.NewSolidColorBrush(walk.RGB(0, 0, 0))

		if offsetX != 0 {
			area.X = offsetX
		}
		if offsetY != 0 {
			area.Y -= offsetY
		}
		canvas.FillRectangle(brush, area)

		defer brush.Dispose()
	}

	//default drawing ops, clearing the canvas, when no content is available
	if sv.itemsCount == 0 || sv.ItemsMap == nil {
		cleaner(viewbounds, 0, 0)
		return nil
	}
	t := time.Now()

	//local vars
	icount := 0
	w := sv.itemSize.twm()
	h := sv.itemSize.thm()
	vi := &sv.viewInfo
	numcols := sv.NumCols()
	numrows := sv.NumRowsVisible()
	//topRow := vi.topPos / h
	topRow := int(math.Ceil(float64(vi.topPos) / float64(h)))
	iStart, iStop := 0, 0
	drawY := abs(0)
	offsetY := (vi.lastPos - vi.topPos)
	//shiftSize := abs(offsetY) % h //h //* numcols-->could be any n, n < h
	//	if shiftSize == 0 {
	//		shiftSize = h
	//	}

	if numcols == 0 {
		return nil
	}

	if !vi.initSLBuffer {
		vi.initSLBuffer = true

		if sv.bmpCntr != nil {
			sv.cvsCntr.Dispose()
			sv.bmpCntr.Dispose()
			sv.cvsCntr = nil
			sv.bmpCntr = nil

		}
		bm := image.NewRGBA(image.Rect(0, 0, w*numcols, (4+numrows)*h))
		sv.bmpCntr, _ = walk.NewBitmapFromImage(bm)
		sv.cvsCntr, _ = walk.NewCanvasFromImage(sv.bmpCntr)

		iStart = (topRow - 2) * numcols
		iStop = iStart + numcols*(4+numrows)
		if iStop > sv.itemsCount {
			iStop = sv.itemsCount
		}
		drawY = 0
		icount = renderScreenBufferA(sv, sv.cvsCntr, iStart, iStop, drawY)

		//log.Println("initBuffer  topRow,iStart,iStop,numrows", topRow, iStart, iStop, numrows)
		vi.lastPos = vi.topPos
	}

	switch {
	case vi.lastPos == vi.topPos:
	case vi.lastPos < vi.topPos: //scroll-down
		//shift Center image buffer UP
		win.BitBlt(sv.cvsCntr.HDC(), 0, int32(offsetY), int32(sv.bmpCntr.Size().Width), int32(sv.bmpCntr.Size().Height),
			sv.cvsCntr.HDC(), 0, 0, win.SRCCOPY)

		//Fill the right buffer with data
		iStart = (topRow + (numrows + 1)) * numcols
		iStop = iStart + numcols

		if iStop > sv.itemsCount {
			iStop = sv.itemsCount
		}
		//drawY = bmpCntr.Size().Height - h

		if vi.topPos%h != 0 {
			drawY = sv.bmpCntr.Size().Height - vi.topPos%(h)
		} else {
			drawY = sv.bmpCntr.Size().Height - h
		}

		//log.Println("scroll-down  vi.topPos,topRow,abs(offsetY)", vi.topPos, topRow, abs(offsetY))

		vi.lastMovePos += abs(offsetY)
		if vi.lastMovePos >= h {
			vi.lastMovePos = 0
		}

	case vi.lastPos > vi.topPos && (vi.topPos >= 0): //scroll-up
		//shift Center image buffer DOWN
		win.BitBlt(sv.cvsCntr.HDC(), 0, int32(offsetY),
			int32(sv.bmpCntr.Size().Width), int32(sv.bmpCntr.Size().Height),
			sv.cvsCntr.HDC(), 0, 0, win.SRCCOPY)

		//Fill the left buffer with data
		iStart = (topRow - 2) * numcols
		iStop = iStart + numcols

		if iStart < 0 {
			iStart = 0
		}
		//drawY = 0
		if vi.topPos%h != 0 {
			drawY = -vi.topPos % (h)
		} else {
			drawY = 0
		}

		vi.lastMovePos += abs(offsetY)
		if vi.lastMovePos >= h {
			vi.lastMovePos = 0
		}
		log.Println("scroll-up  vi.topPos,topRow,abs(offsetY)", vi.topPos, topRow, abs(offsetY))
		//log.Println("scroll-up  topRow,iStart,iStop,numrows", topRow, iStart, iStop, numrows)
	}

	//Blast to screen
	win.BitBlt(canvas.HDC(), 0, 0, int32(w*numcols), int32(h*numrows),
		sv.cvsCntr.HDC(), 0, int32(2*h), win.SRCCOPY)

	//ReFill the buffer with data
	if vi.lastPos != vi.topPos { //}&& vi.lastMovePos == 0 {
		icount = renderScreenBufferA(sv, sv.cvsCntr, iStart, iStop, drawY)
		log.Println("ReFilled the buffer with data  topRow,iStart,iStop,drawY", topRow, iStart, iStop, drawY)
	}

	vi.lastPos = vi.topPos

	d := time.Since(t).Seconds()
	drawStat.add(d)
	log.Println("RedrawScreen rendering ", icount, "items in ",
		fmt.Sprintf("%6.3f", d),
		fmt.Sprintf("%6.3f", drawStat.avg()))

	if sv.handleChangedItems {
		defer sv.contentMonitor.processChangedItem(sv, false)
	}
	return nil
}

type drawstats struct {
	data []float64
}

func (ds *drawstats) avg() float64 {
	var fsum float64
	for _, v := range ds.data {
		fsum += v
	}
	return fsum / float64(len(ds.data))
}
func (ds *drawstats) add(num float64) {
	if len(ds.data) > 5000 {
		ds.data = []float64{}
	}
	ds.data = append(ds.data, num)
}

var drawStat drawstats

func drawfuncNB(sv *ScrollViewer, path string, data *FileInfo) {

	imgBase := image.NewRGBA(image.Rect(0, 0, data.drawRect.Width, data.drawRect.Height))

	mkey := filepath.Join(path, data.Name)
	buf := data.Imagedata

	_, err := renderImageBuffer(sv, mkey, buf, imgBase, 0, 0, sv.SelectedIndex == data.index, true, true)
	if err != nil {
		log.Println("error decoding : ", mkey, len(buf))
	}

	var textout []string

	if sv.viewInfo.showName {
		textout = append(textout, data.Name)
	}
	if sv.viewInfo.showDate {
		textout = append(textout, data.Modified.Format("Jan 2, 2006 3:04pm"))
	}
	if sv.viewInfo.showInfo {
		textout = append(textout, fmt.Sprintf("%d x %d", data.Width, data.Height)+"  "+fmt.Sprintf("%d KB", data.Size/1024))
	}
	drawtext(sv, textout, imgBase, imgBase.Bounds())

	//Draw to screen
	cvs, _ := sv.canvasView.CreateCanvas()

	drawImageRGBAToCanvas(sv, imgBase, cvs.HDC(), data.drawRect.X, data.drawRect.Y,
		data.drawRect.Width, data.drawRect.Height)

	cvs.Dispose()
}
func RedrawScreenNB(sv *ScrollViewer, canvas *walk.Canvas, updateBounds walk.Rectangle, viewbounds walk.Rectangle) error {

	var cleaner = func(cvs *walk.Canvas, area walk.Rectangle, offsetX int, offsetY int) {
		brush, _ := walk.NewSolidColorBrush(walk.RGB(20, 20, 20))

		if offsetX != 0 {
			area.X = offsetX
		}
		if offsetY != 0 {
			area.Y -= offsetY
		}
		cvs.FillRectangle(brush, area)

		defer brush.Dispose()
	}

	//default drawing ops, clearing the canvas, when no content is available
	if sv.itemsCount == 0 || sv.ItemsMap == nil {
		cleaner(canvas, viewbounds, 0, 0)
		return nil
	}

	t := time.Now()

	rUpdate := image.Rect(0, updateBounds.Top(), updateBounds.Width, updateBounds.Bottom())

	w := sv.itemSize.twm()
	h := sv.itemSize.thm()
	//-------------------------------------
	//Begin Screen adjustments
	//-------------------------------------
	iscrollSize := sv.viewInfo.lastPos - sv.viewInfo.topPos
	if abs(iscrollSize) > 2*sv.itemHeight {
		iscrollSize = 2 * sv.itemHeight * iscrollSize / abs(iscrollSize)
	}
	//shift onscreen image according to scroll direction
	if iscrollSize != 0 {
		win.BitBlt(canvas.HDC(),
			0, int32(iscrollSize), int32(sv.ViewWidth()), int32(sv.ViewHeight()),
			canvas.HDC(), 0, 0, win.SRCCOPY)
	}
	sv.viewInfo.lastPos = sv.viewInfo.topPos
	//-------------------------------------
	//End Screen adjustments
	//-------------------------------------
	//icount := 0
	numcols := int(math.Trunc(float64(viewbounds.Width) / float64(w)))

	if numcols == 0 {
		return nil
	}
	workmap := make(ItmMap, len(sv.itemsModel.items))

	for i := range sv.itemsModel.items {
		x := (i % numcols) * w
		y := int(i/numcols) * h

		rItm := image.Rect(x, y+1, x+w, y+h-1)
		//---------------------------------------------------------
		//Only perform drawing ops on items within the Update rect
		//---------------------------------------------------------
		if rUpdate.Intersect(rItm) != image.ZR {
			//icount++
			mkey := sv.itemsModel.getFullPath(i)

			if v, ok := sv.ItemsMap[mkey]; ok {
				//record n store the calculated screen coordinates
				//to workmap
				v.drawRect = walk.Rectangle{x, y - sv.viewInfo.topPos, w, h}
				workmap[mkey] = v
			}
		}
	}

	//experimental drawing w/ goroutines
	if sv.drawersChan == nil {
		sv.drawersChan = make(chan *FileInfo)
	}
	if !sv.drawersActive {
		sv.drawersActive = true
		sv.drawerFunc = drawfuncNB
		sv.drawersCount = 4

		for g := 0; g < sv.drawersCount; g++ {
			//---------------------------------------------------
			go func(path string, datachan chan *FileInfo) {
				for v := range datachan {
					if v != nil {
						sv.drawerFunc(sv, path, v)
						sv.drawersWait.Done()
					} else {
						sv.drawersWait.Done()
						log.Println("screen drawer goroutine, exiting...bye")
						break
					}
				}
			}(sv.itemsModel.dirPath, sv.drawersChan)
			//---------------------------------------------------
		}
	} else {
		if fmt.Sprint(sv.drawerFunc) != fmt.Sprint(drawfuncNB) {
			log.Println("Different drawerfunc is in use. Switching draw func to drawfuncNB")
			sv.drawerFunc = drawfuncNB
		}
	}

	getNextItem := func() (res *FileInfo) {
		for k, v := range workmap {
			res = v
			delete(workmap, k)
			break
		}
		return res
	}

	//setup sync waiter to the number of
	//items to process
	sv.drawersWait.Add(len(workmap))

	//launch the driver loop to send data
	//to the previously launched goroutines.
loop:
	for {
		v := getNextItem()

		if v != nil {
			sv.drawersChan <- v
		} else {
			break loop
		}
	}
	//wait for all workitem to be processed
	sv.drawersWait.Wait()

	b := sv.canvasView.Bounds()
	//cleanup code
	//right side
	if w*numcols < b.Right() {
		rClear := walk.Rectangle{w * numcols, 0, b.Right() - w*numcols, b.Bottom()}
		cleaner(canvas, rClear, 0, 0)
	}

	//end of items side
	numitem := sv.itemsCount
	numodd := numitem % sv.NumCols()
	if numodd > 0 && sv.viewInfo.topPos == sv.scroller.MaxValue() {
		rClear := walk.Rectangle{numodd * sv.itemWidth, b.Bottom() - sv.itemHeight,
			b.Right() - numodd*sv.itemWidth, sv.itemHeight}

		cleaner(canvas, rClear, 0, 0)
	}

	d := time.Since(t).Seconds()
	drawStat.add(d)
	//log.Println("RedrawScreen rendering: ", icount, "items in ", fmt.Sprintf("%6.3f", d), fmt.Sprintf("%6.3f", drawStat.avg()))

	if sv.handleChangedItems {
		defer sv.contentMonitor.processChangedItem(sv, false)
	}
	return nil
}
func drawfuncNB2(sv *ScrollViewer, path string, data *FileInfo) {

	mkey := filepath.Join(path, data.Name)
	buf := data.Imagedata

	ws, hs := sv.itemSize.tw, sv.itemSize.th
	img := image.NewRGBA(image.Rect(0, 0, data.drawRect.Width, hs))
	wd, hd := getOptimalThumbSize(ws, hs, data.Width, data.Height)

	_, err := renderImageBuffer(sv, mkey, buf, img, 0, hs-hd, false, false, false)
	if err != nil {
		log.Println("error decoding : ", mkey, len(buf), wd)
	}

	if sv.SelectedIndex == data.index {
		renderBorder(sv, img, 0, 0, wd, hs)
	}

	//Draw to screen
	//cvs, _ := sv.canvasView.CreateCanvas()
	//sv.drawerMutex.Lock()
	cvs := sv.drawerHDC

	drawImageRGBAToCanvas(sv, img, cvs, data.drawRect.X, data.drawRect.Y,
		data.drawRect.Width, data.drawRect.Height)
	//sv.drawerMutex.Unlock()
	//cvs.Dispose()
}

func RedrawScreenNB2(sv *ScrollViewer, canvas *walk.Canvas, updateBounds walk.Rectangle, viewbounds walk.Rectangle) error {

	cleaner := func(cvs *walk.Canvas, area walk.Rectangle, offsetX int, offsetY int) {
		brush, _ := walk.NewSolidColorBrush(walk.RGB(20, 20, 20))

		if offsetX != 0 {
			area.X = offsetX
		}
		if offsetY != 0 {
			area.Y -= offsetY
		}
		cvs.FillRectangle(brush, area)

		defer brush.Dispose()
	}

	//default drawing ops, clearing the canvas, when no content is available
	if sv.itemsCount == 0 || sv.ItemsMap == nil {
		cleaner(canvas, viewbounds, 0, 0)
		return nil
	}

	t := time.Now()

	if sv.drawerBitmap == nil {
		img := image.NewRGBA(image.Rect(0, 0, sv.ViewWidth(), sv.ViewHeight()))
		sv.drawerBitmap, _ = walk.NewBitmapFromImage(img)
		sv.drawerCanvas, _ = walk.NewCanvasFromImage(sv.drawerBitmap)
		sv.drawerHDC = sv.drawerCanvas.HDC()
	}
	if sv.drawerBitmap.Size().Width != sv.ViewWidth() || sv.drawerBitmap.Size().Height != sv.ViewHeight() {
		sv.drawerCanvas.Dispose()
		sv.drawerBitmap.Dispose()

		img := image.NewRGBA(image.Rect(0, 0, sv.ViewWidth(), sv.ViewHeight()))
		sv.drawerBitmap, _ = walk.NewBitmapFromImage(img)
		sv.drawerCanvas, _ = walk.NewCanvasFromImage(sv.drawerBitmap)
		sv.drawerHDC = sv.drawerCanvas.HDC()
	}

	rUpdate := image.Rect(0, updateBounds.Top(), updateBounds.Width, updateBounds.Bottom())

	w := sv.itemSize.tw
	h := sv.itemSize.th
	//-------------------------------------
	//Begin Screen adjustments
	//-------------------------------------
	iscrollSize := sv.viewInfo.lastPos - sv.viewInfo.topPos
	if abs(iscrollSize) > 2*h {
		iscrollSize = 2 * h * iscrollSize / abs(iscrollSize)
	}
	//shift onscreen image according to scroll direction
	if iscrollSize != 0 {
		//		win.BitBlt(canvas.HDC(),
		//			0, int32(iscrollSize), int32(sv.ViewWidth()), int32(sv.ViewHeight()-iscrollSize),
		//			canvas.HDC(), 0, 0, win.SRCCOPY)

		win.BitBlt(sv.drawerCanvas.HDC(),
			0, int32(iscrollSize), int32(sv.ViewWidth()), int32(sv.ViewHeight()-iscrollSize),
			sv.drawerCanvas.HDC(), 0, 0, win.SRCCOPY)

	}
	sv.viewInfo.lastPos = sv.viewInfo.topPos
	//-------------------------------------
	//End Screen adjustments
	//-------------------------------------

	icount := 0
	x := 0
	y := 0
	wmax := sv.ViewWidth()
	numItems := len(sv.itemsModel.items)

	workmap := make(ItmMap, numItems)

	for i, val := range sv.itemsModel.items {

		wd, hd := getOptimalThumbSize(w, h, val.Width, val.Height)
		if wd+hd == 0 {
			continue
		}
		if x+wd > wmax { //check if next image will fit, > means wont fit
			if wmax-x > 6*wd/10 { //check again, if at least 3/4 will fit
				wd = wmax - x
			} else {
				x = 0
				y += h
			}
		}
		rItm := image.Rect(x, y, x+wd, y+h)
		//---------------------------------------------------------
		//Only perform drawing ops on items within the Update rect
		//---------------------------------------------------------
		if rUpdate.Intersect(rItm) != image.ZR {
			icount++

			w0 := wd
			wnxt := 0
			if i+1 < numItems {
				valx := sv.itemsModel.items[i+1]
				wnxt, _ = getOptimalThumbSize(w, h, valx.Width, valx.Height)
			}

			if wmax-(x+wd) <= 6*wnxt/10 || i == numItems-1 {
				wd = wmax - x
			}
			mkey := sv.itemsModel.getFullPath(i)

			//record n store the calculated screen coordinates
			//to workmap
			val.drawRect = walk.Rectangle{x, y - sv.viewInfo.topPos, wd, h}
			workmap[mkey] = val
			wd = w0
		}
		x += wd
	}

	//experimental drawing w/ goroutines
	if sv.drawersChan == nil {
		sv.drawersChan = make(chan *FileInfo)
	}
	if !sv.drawersActive {
		sv.drawersActive = true
		sv.drawerFunc = drawfuncNB2
		sv.drawersCount = 4

		for g := 0; g < sv.drawersCount; g++ {
			//---------------------------------------------------
			go func(path string, datachan chan *FileInfo) {
				for v := range datachan {
					if v != nil {
						sv.drawerFunc(sv, path, v)
						sv.drawersWait.Done()
					} else {
						sv.drawersWait.Done()
						log.Println("screen drawer goroutine, exiting...bye")
						break
					}
				}
			}(sv.itemsModel.dirPath, sv.drawersChan)
			//---------------------------------------------------
		}
	} else {
		if fmt.Sprint(sv.drawerFunc) != fmt.Sprint(drawfuncNB2) {
			log.Println("Different drawerfunc is in use. Switching draw func to drawfuncNB2")
			sv.drawerFunc = drawfuncNB2
		}
	}

	getNextItem := func() (res *FileInfo) {
		for k, v := range workmap {
			res = v
			delete(workmap, k)
			break
		}
		return res
	}
	// setup sync waiter to the number of
	// items to process
	sv.drawersWait.Add(len(workmap))

	// launch the driver loop to send data
	// to the previously launched goroutines.
loop:
	for {
		v := getNextItem()

		if v != nil {
			sv.drawersChan <- v
		} else {
			break loop
		}
	}
	//wait for all workitem to be processed
	sv.drawersWait.Wait()

	win.BitBlt(canvas.HDC(), 0, 0, int32(sv.ViewWidth()), int32(sv.ViewHeight()), sv.drawerCanvas.HDC(), 0, 0, win.SRCCOPY)

	d := time.Since(t).Seconds()
	drawStat.add(d)

	log.Println("RedrawScreen rendering ", icount, "items in ", fmt.Sprintf("%6.3f", d), fmt.Sprintf("%6.3f", drawStat.avg()))

	if sv.handleChangedItems {
		defer sv.contentMonitor.processChangedItem(sv, false)
	}
	return nil
}

func DrawTestImage(fpath string) (int, bool) {
	w := 160 //thumbR.tw
	h := 90  //thumbR.th

	imgrgba := image.NewRGBA(image.Rect(0, 0, w, h))
	num := 311

	bm, _ := walk.NewBitmapFromImage(imgrgba)
	cvs, _ := walk.NewCanvasFromImage(bm)
	br, _ := walk.NewSolidColorBrush(walk.RGB(20, 20, 20))
	ft, _ := walk.NewFont("Times New Roman", 40, walk.FontBold)

	for i := 0; i < num; i++ {
		s := fmt.Sprintf("%04d", i)
		opt := walk.TextCenter | walk.TextVCenter | walk.TextSingleLine

		cvs.FillRectangle(br, cvs.Bounds())
		cvs.DrawText(s, ft, walk.RGB(180, 180, 240), walk.Rectangle{0, 0, w, h}, opt)

		img, _ := walkBitmapToImageRGBA(bm)

		if img != nil {
			f, err := os.Create(filepath.Join(fpath, "testimg"+fmt.Sprintf("%04d", i)+".png"))
			if err == nil {
				err = png.Encode(f, img)
			} else {
				return i, false
			}
			f.Close()
		}
	}

	defer br.Dispose()
	defer cvs.Dispose()
	defer bm.Dispose()
	defer ft.Dispose()
	return num, true
}
func SaveWalkBitmap(bm *walk.Bitmap, name string) {
	img, _ := walkBitmapToImageRGBA(bm)

	if img != nil {
		f, err := os.Create(name)
		if err == nil {
			err = png.Encode(f, img)
		}
		f.Close()
	}
}

func walkBitmapToImageRGBA(bm *walk.Bitmap) (*image.RGBA, error) {

	hBmp := win.HBITMAP(reflect.ValueOf(bm).Elem().Field(0).Uint())

	var bi win.BITMAPINFO
	bi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bi.BmiHeader))
	hdc := win.GetDC(0)
	if ret := win.GetDIBits(hdc, hBmp, 0, 0, nil, &bi, win.DIB_RGB_COLORS); ret == 0 {
		return nil, nil
	}

	buf := make([]byte, bi.BmiHeader.BiSizeImage)
	bi.BmiHeader.BiCompression = win.BI_RGB
	if ret := win.GetDIBits(hdc, hBmp, 0, uint32(bi.BmiHeader.BiHeight), &buf[0], &bi, win.DIB_RGB_COLORS); ret == 0 {
		return nil, nil
	}

	width := int(bi.BmiHeader.BiWidth)
	height := int(bi.BmiHeader.BiHeight)
	im := image.NewRGBA(image.Rect(0, 0, width, height))

	n := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := buf[n+2]
			g := buf[n+1]
			b := buf[n+0]
			n += 4
			im.Set(x, height-y, color.RGBA{r, g, b, 255})
		}
	}
	return im, nil
}

var (
	dpi = flag.Float64("dpi", 96, "screen resolution in Dots Per Inch")
	//fontfile = flag.String("fontfile", "../../golang/freetype/testdata/luxisr.ttf", "filename of the ttf font")
	fontfile = flag.String("fontfile", "../../golang/freetype/testdata/LaoUI.ttf", "filename of the ttf font")

	hinting = flag.String("hinting", "none", "none | full")
	size    = flag.Float64("size", 10, "font size in points")
	spacing = flag.Float64("spacing", 1.1, "line spacing (e.g. 2 means double spaced)")
	wonb    = flag.Bool("whiteonblack", false, "white text on a black background")
)

var fontBytes []byte
var fnt *truetype.Font

func inittext() bool {
	var err error

	fontBytes, err = ioutil.ReadFile(*fontfile)
	if err != nil {
		log.Println(err)
		return false
	}
	fnt, err = truetype.Parse(fontBytes)
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}

func drawtext(sv *ScrollViewer, text []string, dst *image.RGBA, rdst image.Rectangle) {
	// Read the font data
	if fontBytes == nil {
		if res := inittext(); !res {
			return
		}
	}

	fg := image.NewUniform(color.RGBA{200, 200, 200, 255})
	imgW := sv.itemSize.twm()

	// Draw the text.
	h := font.HintingNone
	switch *hinting {
	case "full":
		h = font.HintingFull
	}
	d := &font.Drawer{
		Dst: dst,
		Src: fg,
		Face: truetype.NewFace(fnt, &truetype.Options{
			Size:    *size,
			DPI:     *dpi,
			Hinting: h,
		}),
	}

	y := sv.itemSize.thm() - sv.itemSize.txth - 3 + int(math.Ceil(*size**dpi/72))
	dy := int(math.Ceil(*size * *spacing * *dpi / 72))

	for _, s := range text {
		//perform text truncation to fit text area
		w := d.MeasureString(s)
		wa := fixed.I(sv.itemSize.tw)
		if w > wa {
			sr := []rune(s)
			ss := ""

			for i := 0; i < len(s); i++ {
				w = d.MeasureString(ss)
				if w > wa {
					s = ss + "..."
					break
				}
				ss = ss + string(sr[i])
			}
		}

		d.Dot = fixed.Point26_6{
			//			X: (fixed.I(imgW) - w) / 2,
			//			Y: fixed.I(y),
			X: fixed.I(rdst.Min.X) + (fixed.I(imgW)-w)/2,
			Y: fixed.I(y),
		}
		d.DrawString(s)

		y += dy
	}
}
