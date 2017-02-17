// Copyright 2012 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	//"log"
	//	"time"
	"fmt"
	"math"
	"unsafe"
)

import (
	"github.com/lxn/walk"
	//. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

var cmb *walk.ComboBox

//var scvr *ScrollViewer

type cmbdata struct {
	Id  int
	Val string
}

type ScrollViewer struct {
	scrollview  *walk.ScrollView
	canvasView  *walk.CustomWidget
	items       int
	itemWidth   int
	itemHeight  int
	evPaint     walk.PaintFunc
	evMouseDown walk.MouseEventHandler
}

func NewScrollViewer(parent walk.Container, paintfunc walk.PaintFunc, itmCount, itmWidth, itmHeight int) (*ScrollViewer, error) {
	var err error
	svr := &ScrollViewer{
		//		scrollview:        svw,
		//		canvasView:        cw,
		items:      itmCount,
		itemWidth:  itmWidth,
		itemHeight: itmHeight,
	}
	svr.scrollview, err = walk.NewScrollView(parent)
	lyt := walk.NewVBoxLayout()
	svr.scrollview.SetLayout(lyt)
	parent.Children().Add(svr.scrollview)

	svr.canvasView, _ = walk.NewCustomWidget(svr.scrollview, 0, svr.onPaint)
	svr.scrollview.Children().Add(svr.canvasView)

	svr.evPaint = paintfunc
	svr.SetEventSizeChanged(svr.onSizeChanged)
	svr.canvasView.MouseDown().Attach(svr.OnMouseDown)

	//	br, err := walk.NewSolidColorBrush(walk.RGB(200, 200, 50))
	//	svr.canvasView.SetBackground(br)
	return svr, err
}
func (sv *ScrollViewer) SetEventPaint(eventproc walk.PaintFunc) {
	sv.evPaint = eventproc
}
func (sv *ScrollViewer) SetEventSizeChanged(eventproc walk.EventHandler) {
	sv.scrollview.SizeChanged().Attach(eventproc)
}

func (sv *ScrollViewer) oncanvasViewpaint(canvas *walk.Canvas, updaterect walk.Rectangle) error {
	return nil
}
func (sv *ScrollViewer) OnMouseDown(x, y int, button walk.MouseButton) {
	if sv.evMouseDown != nil {
		sv.evMouseDown(x, y, button)
		//return
	}
	sv.scrollview.SetFocus()
	//sv.canvasView.Invalidate()
}
func (sv *ScrollViewer) onSizeChanged() {
	sv.recalcSize(false)
	sv.canvasView.Invalidate()
}
func (sv *ScrollViewer) recalcSize(resetparent bool) int {
	h := sv.NumRows() * sv.itemHeight
	if h != 0 {

		sv.canvasView.SetMinMaxSize(walk.Size{0, h}, walk.Size{0, 0})

		if resetparent {
			sv.scrollview.SetHeight(sv.scrollview.Height() + 1)
		}
		if sv.canvasView.Height() != h {
			sv.canvasView.SetHeight(h)
		}
		if sv.scrollview.AsContainerBase().Height() != h {
			sv.scrollview.AsContainerBase().SetHeight(h)
		}
	}
	//	log.Println("recalcSize ItemCount,ItemWidth,ItemHeight", sv.items, sv.itemWidth, sv.itemHeight)
	//	log.Println("recalcSize h,NumRows,NumCols", h, sv.NumRows(), sv.NumCols())
	//	log.Println("recalcSize", sv.scrollview.AsContainerBase().Bounds(), sv.canvasView.Bounds())

	return h
}
func (sv *ScrollViewer) Bounds() walk.Rectangle {
	return sv.scrollview.AsContainerBase().Bounds()
}
func (sv *ScrollViewer) SetFocus() {
	sv.scrollview.SetFocus()
}

func (sv *ScrollViewer) SetContextMenu(menu *walk.Menu) {
	sv.canvasView.SetContextMenu(menu)
}

func (sv *ScrollViewer) SetItemCount(ic int) {
	if sv.items != ic {
		sv.items = ic
		sv.recalcSize(true)
	}
}
func (sv *ScrollViewer) SetItemSize(w, h int) {
	if sv.itemWidth != w || sv.itemHeight != h {
		sv.itemWidth = w
		sv.itemHeight = h
		sv.recalcSize(true)
	}
}

func (sv *ScrollViewer) SetMouseDown(evt walk.MouseEventHandler) {
	sv.evMouseDown = evt
}
func (sv *ScrollViewer) Invalidate() {
	sv.canvasView.Invalidate()
}
func (sv *ScrollViewer) Show() {
	sv.scrollview.SetVisible(true)
}
func (sv *ScrollViewer) Width() int {
	return sv.scrollview.AsContainerBase().Width()
}
func (sv *ScrollViewer) Height() int {
	return sv.scrollview.AsContainerBase().Height()
}
func (sv *ScrollViewer) ViewHeight() int {
	return sv.canvasView.Height()
}

func (sv *ScrollViewer) NumRows() int {
	ic := sv.items
	nc := sv.NumCols()
	return int(math.Ceil(float64(ic) / float64(nc)))
}
func (sv *ScrollViewer) NumCols() int {
	return int(math.Trunc(float64(sv.Width()) / float64(sv.itemWidth)))
}

func (sv *ScrollViewer) ResetPos() {
	sv.scrollview.AsContainerBase().SetY(0)
	var si win.SCROLLINFO
	si.CbSize = uint32(unsafe.Sizeof(si))
	si.FMask = win.SIF_POS
	si.NPos = 0
	win.SetScrollInfo(sv.scrollview.Handle(), win.SB_VERT, &si, true)
}

var ft *walk.Font

func (sv *ScrollViewer) onPaint(canvas *walk.Canvas, updaterect walk.Rectangle) error {
	if sv.evPaint != nil {
		sv.evPaint(canvas, updaterect)
		return nil
	}

	if ft == nil {
		ft, _ = walk.NewFont("arial", 20, walk.FontBold)
	}
	p, _ := walk.NewCosmeticPen(walk.PenSolid, walk.RGB(0, 0, 0))
	defer p.Dispose()

	w := sv.itemWidth
	h := sv.itemHeight
	num := sv.items
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
