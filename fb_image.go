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
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

import (
	"github.com/anthonynsimon/bild/transform"
	"github.com/lxn/walk"
	"github.com/pixiv/go-libjpeg/jpeg"
	"golang.org/x/image/webp"
)

type ThumbRect struct {
	tw, th int
	mx, my int
	txth   int
}

func (t ThumbRect) twm() int {
	return t.tw + 2*t.mx
}

func (t ThumbRect) thm() int {
	return t.th + t.txth + 2*t.my
}

var thumbR = ThumbRect{120, 75, 10, 10, 48}
var done chan int
var donewait sync.WaitGroup

var doCache = false

//var doCache = true

func setImgcache(numJob int, dirpath string) bool {
	if done != nil {
		close(done)
		donewait.Wait()
	}
	Mw.pgBar.SetValue(0)
	Mw.pgBar.SetMarqueeMode(false)
	Mw.pgBar.SetRange(0, numJob)
	runtime.GC()

	var itms []string

	for l := 0; l < numJob; l++ {
		itms = append(itms, filepath.Join(dirpath, tableModel.items[l].Name))
	}

	grCount := runtime.NumCPU()
	wtCount := grCount
	c := int(numJob / grCount)

	if ((numJob % grCount) > 0) && (numJob < grCount) {
		wtCount = 1
	}

	donewait.Add(wtCount)
	done = make(chan int, 1)
	workCounter = 0

	go func() {
		k := 0
		t := time.Now()

		if grCount > numJob {
			grCount = 1
		}

		//Load data from cache
		if doCache {
			CacheDBEnum(dirpath)
		}

		for j := 0; j < grCount; j++ {
			iStart := k
			iStop := k + c

			if (((numJob % grCount) > 0) && (j == grCount-1)) || (grCount == 1) {
				iStop = numJob
			}

			go doRendering(done, itms, iStart, iStop)

			k = k + c
		}

		donewait.Wait()
		d := time.Since(t).Seconds()

		//stupid info updating
		Mw.Synchronize(func() {
			Mw.pgBar.SetValue(numJob)
			Mw.MainWindow.SetTitle(tableModel.dirPath + " (" + strconv.Itoa(numJob) + " files) in " + strconv.FormatFloat(d, 'f', 3, 64))
		})
		time.Sleep(time.Second)
		Mw.Synchronize(func() {
			Mw.paintWidget.Invalidate()
			Mw.pgBar.SetValue(0)
		})

		if doCache {
			cntupd, _ := CacheDBUpdate(dirpath)
			log.Println("Cache items updated: ", cntupd)
		}
	}()
	return true
}

var workCounter int64

func doRendering(done chan int, fnames []string, iStart, iStop int) bool {
	res := true
loop:
	for i := iStart; i < iStop; i++ {
		select {
		case <-done:
			res = false
			break loop
		default:
			if err := processImageData(fnames[i]); err != nil {
				res = false
			}
			c := atomic.AddInt64(&workCounter, 1)

			if c%10 == 0 {
				Mw.pgBar.Synchronize(func() {
					Mw.pgBar.SetValue(int(workCounter))
				})
			}
		}
	}
	donewait.Done()
	return res
}

func getOptimalThumbSize(dstW int, dstH int, srcW int, srcH int) (int, int) {
	getW := func(h int, ws int, hs int) int { return int(float32(h) / float32(hs) * float32(ws)) }
	getH := func(w int, ws int, hs int) int { return int(float32(w) / float32(ws) * float32(hs)) }

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

func processImageData(mkey string) error {

	v, ok := ItemsMap[mkey]
	if !ok {
		return nil
	}

	//Skip thumb creation if ItemsMap already has data.
	if doCache && v.Imagedata != nil {
		if !v.Changed {
			return nil
		}
	}
	//open
	file, err := os.Open(mkey)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer file.Close()

	//set desired scaled size
	resW := thumbR.tw
	resH := thumbR.th

	var img image.Image

	//Retrieve image dimension, etc based on type
	switch imgType := filepath.Ext(mkey); imgType {
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
			log.Fatal(err)
		}
	case ".webp":
		img, err = webp.Decode(file)
		if err != nil {
			err = nil
		}
	}

	if img == nil {
		return nil
	}

	if img.Bounds().Dx() < 8 {
		return nil
	}

	w, h := getOptimalThumbSize(thumbR.tw, thumbR.th, img.Bounds().Dx(), img.Bounds().Dy())

	//Further scaling ops req to fit the src img
	//to the desired display size.
	mt := transform.Resize(img, w, h, transform.NearestNeighbor)

	//Encode the scaled image & save to cache map
	jept := jpeg.EncoderOptions{Quality: 80, OptimizeCoding: false, DCTMethod: jpeg.DCTIFast}
	buf := new(bytes.Buffer)

	err = jpeg.Encode(buf, mt, &jept)
	if err == nil {
		v.Imagedata = make([]byte, buf.Len())
		buf.Read(v.Imagedata)
		v.Changed = false
	} else {
		log.Fatal(err)
	}

	return nil
}

func renderImageDataTo(mkey string, buf []byte, dst *image.RGBA, x int, y int, selected bool) error {
	if buf == nil {
		return nil
	}

	//set desired scaled size
	resW := thumbR.twm()
	resH := thumbR.thm() - thumbR.txth

	//set & draw outer border effect
	var r image.Rectangle
	if !selected {
		draw.Draw(dst, dst.Bounds(), &image.Uniform{color.RGBA{80, 80, 80, 255}}, image.ZP, draw.Src)
		r = dst.Bounds().Inset(1)
	} else {
		draw.Draw(dst, dst.Bounds(), &image.Uniform{color.RGBA{200, 100, 100, 255}}, image.ZP, draw.Src)
		r = dst.Bounds().Inset(3)
	}
	//draw the inner rect
	draw.Draw(dst, r, &image.Uniform{color.RGBA{0, 0, 0, 255}}, image.ZP, draw.Src)

	//decode
	jopt := jpeg.DecoderOptions{ScaleTarget: image.Rect(0, 0, thumbR.tw, thumbR.th)}

	buff := bytes.NewBuffer(buf)
	img, err := jpeg.DecodeIntoRGBA(buff, &jopt)
	if err != nil {
		return err
	}

	w, h := getOptimalThumbSize(thumbR.tw, thumbR.th, img.Bounds().Dx(), img.Bounds().Dy())

	//Further scaling ops req to fit the src img
	//to the desired display size.
	mt := img

	if (img.Bounds().Dx() != w) || (img.Bounds().Dy() != h) {
		mt = transform.Resize(img, w, h, transform.NearestNeighbor)

		submitChangedItem(mkey, ItemsMap[mkey])
	}

	//centers x,y
	x += int(float32(resW-w) / 2)
	y += int(float32(resH-h) / 2)

	draw.Draw(dst, image.Rect(x, y, x+w, y+h), mt, mt.Bounds().Min, draw.Src)

	return err
}

var cList chan *FileInfo
var changeMap ItmMap

func removeChangedItem(mkey string) {
	if changeMap != nil {
		if _, ok := changeMap[mkey]; ok {
			delete(changeMap, mkey)
		}
	}
}

func submitChangedItem(mkey string, cItm *FileInfo) {
	if changeMap == nil {
		changeMap = make(ItmMap)
	}
	changeMap[mkey] = cItm
	changeMap[mkey].Changed = true
}

func processChangedItem() {
	if changeMap == nil {
		return
	}

	go func() {
		for k, _ := range changeMap {
			//log.Println("CacheDBUpdateItemFromBuffer: ", v.Name)

			processImageData(k)

			CacheDBUpdateItem(k)

			removeChangedItem(k)
		}
		log.Println("CacheDBUpdateItemFromBuffer: ", len(changeMap))
	}()
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
	case ".webp":
		imgcfg, err = webp.DecodeConfig(file)
		if err != nil {
			//log.Fatal(err)
			err = nil
		}
	}

	w.Width = imgcfg.Width
	w.Height = imgcfg.Height

	return w, err
}

func createBaseBitmap(focused bool) (*walk.Bitmap, error) {
	bounds := walk.Rectangle{Width: thumbR.tw, Height: thumbR.th}

	bmp, err := walk.NewBitmap(bounds.Size())
	if err != nil {
		return nil, err
	}

	succeeded := false
	defer func() {
		if !succeeded {
			bmp.Dispose()
		}
	}()

	canvas, err := walk.NewCanvasFromImage(bmp)
	if err != nil {
		return nil, err
	}
	defer canvas.Dispose()

	brushColor := walk.RGB(155, 155, 255)
	if focused {
		brushColor = walk.RGB(155, 155, 155)
	}

	brush, err := walk.NewSolidColorBrush(brushColor)
	if err != nil {
		return nil, err
	}
	defer brush.Dispose()

	if err := canvas.FillRectangle(brush, bounds); err != nil {
		return nil, err
	}

	rectPen, err := walk.NewCosmeticPen(walk.PenSolid, walk.RGB(80, 80, 80))
	if err != nil {
		return nil, err
	}
	defer rectPen.Dispose()

	if err := canvas.DrawRectangle(rectPen, bounds); err != nil {
		return nil, err
	}

	//	font, err := walk.NewFont("Times New Roman", 40, walk.FontBold|walk.FontItalic)
	//	if err != nil {
	//		return nil, err
	//	}
	//	defer font.Dispose()

	//	if err := canvas.DrawText(drawstr, font, walk.RGB(0, 0, 0), bounds, walk.TextWordbreak); err != nil {
	//		return nil, err
	//	}

	succeeded = true

	return bmp, nil
}

func RedrawScreen(canvas *walk.Canvas, updateBounds walk.Rectangle, viewbounds walk.Rectangle) error {
	if (ItemsMap == nil) || (len(tableModel.items) == 0) {
		return nil
	}
	t := time.Now()

	rUpdate := image.Rect(0, updateBounds.Top(), updateBounds.Width, updateBounds.Bottom())

	//create a base rgba image, will be used multiple times
	w := thumbR.twm()
	h := thumbR.thm()
	imgBase := image.NewRGBA(image.Rect(0, 0, w, h))

	icount := 0
	numcols := int(math.Trunc(float64(viewbounds.Width) / float64(w)))
	//numrows := int(math.Ceil(float64(viewbounds.Height) / float64(h)))

	for i := range tableModel.items {

		x := (i % numcols) * w
		y := int(i/numcols) * h

		rItm := image.Rect(x, y, x+w, y+h)

		if rUpdate.Intersect(rItm) != image.ZR {
			icount++
			//Retrieve the stored image data in the ItemsMap,
			//render the stored image data to base rgba image
			mkey := tableModel.getFullPath(i)

			if v, ok := ItemsMap[mkey]; ok {
				if buf := v.Imagedata; buf != nil {
					err := renderImageDataTo(mkey, buf, imgBase, 0, 0, tableView.CurrentIndex() == i)
					if err != nil {
						log.Println("error decoding : ", mkey, len(buf))
					}
					info1 := fmt.Sprintf("%d x %d", v.Width, v.Height)
					info2 := fmt.Sprintf("%d ", v.Size)
					drawtext([]string{v.Name, info1, info2}, imgBase)
				}
			}
			//Draw to screen
			if wbmp, err := walk.NewBitmapFromImage(imgBase); err == nil {
				if err := canvas.DrawImage(wbmp, walk.Point{x, y}); err != nil {
					return err
				}
				defer wbmp.Dispose()
			}
		}
	}
	defer processChangedItem()
	log.Println("redraw time", icount, "items in", time.Since(t).Seconds())

	//	yTop := mw.scrollWidget.AsContainerBase().Y()

	//	if yTop%thumbR.thm() != 0 {
	//		numrows += 1
	//	}
	//	startrow := (yTop / thumbR.thm())
	//	nextrow := int(math.Ceil(float64(-yTop+viewbounds.Height) / float64(thumbR.thm())))

	//	log.Println("viewspec: ytop:", yTop, "numcols:", numcols, "numrows:", numrows, "startrow:", startrow, "nextrow", nextrow, viewbounds)

	return nil
}

var (
	dpi      = flag.Float64("dpi", 96, "screen resolution in Dots Per Inch")
	fontfile = flag.String("fontfile", "../../golang/freetype/testdata/luxisr.ttf", "filename of the ttf font")
	hinting  = flag.String("hinting", "none", "none | full")
	size     = flag.Float64("size", 10, "font size in points")
	spacing  = flag.Float64("spacing", 1.1, "line spacing (e.g. 2 means double spaced)")
	wonb     = flag.Bool("whiteonblack", false, "white text on a black background")
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

func drawtext(text []string, dst *image.RGBA) {
	// Read the font data
	if fontBytes == nil {
		if res := inittext(); !res {
			return
		}
	}

	fg := image.NewUniform(color.RGBA{200, 200, 200, 255})
	imgW := thumbR.twm()

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

	y := thumbR.thm() - thumbR.txth + int(math.Ceil(*size**dpi/72))
	dy := int(math.Ceil(*size * *spacing * *dpi / 72))

	for _, s := range text {
		//perform text truncation to fit text area
		w := d.MeasureString(s)
		wa := fixed.I(thumbR.tw)
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
			X: (fixed.I(imgW) - w) / 2,
			Y: fixed.I(y),
		}
		d.DrawString(s)
		y += dy
	}
}
