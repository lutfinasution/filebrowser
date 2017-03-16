// Copyright 2010 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package walk

import (
	"log"
)

//type splitterLayoutItem struct {
//	size          int
//	stretchFactor int
//	growth        int
//	fixed         bool
//	visScale      float64 //add by mln
//}

func (s *splitterLayoutItem) Size() int {
	return s.size
}
func (s *splitterLayoutItem) SetSize(val int) {
	s.size = val
}
func (s *splitterLayoutItem) StretchFactor() int {
	return s.stretchFactor
}
func (s *splitterLayoutItem) SetStretchFactor(val int) {
	s.stretchFactor = val
}
func (s *splitterLayoutItem) Growth() int {
	return s.growth
}
func (s *splitterLayoutItem) SetGrowth(val int) {
	s.growth = val
}
func (s *splitterLayoutItem) Fixed() bool {
	return s.fixed
}
func (s *splitterLayoutItem) SetFixed(val bool) {
	s.fixed = val
}
func (s *splitterLayoutItem) Scale() float64 {
	return s.visScale
}
func (s *splitterLayoutItem) SetScale(val float64) {
	s.visScale = val
}

func (s *Splitter) SetSplitterPos(pos int) (err error) {

	handleIndex := 1 //s.children.Index(dragHandle)
	hndl := s.children.At(handleIndex)
	prev := s.children.At(handleIndex - 1)
	next := s.children.At(handleIndex + 1)

	prev.SetSuspended(true)
	defer prev.Invalidate()
	defer prev.SetSuspended(false)
	next.SetSuspended(true)
	defer next.Invalidate()
	defer next.SetSuspended(false)

	if s.Orientation() == Horizontal {
		hndl.SetX(hndl.X() + pos)
	} else {
		hndl.SetY(hndl.Y() + pos)
	}
	bh := hndl.Bounds()
	bp := prev.Bounds()
	bn := next.Bounds()

	var sizePrev int
	var sizeNext int

	if s.Orientation() == Horizontal {
		bp.Width = bh.X - bp.X
		bn.Width -= (bh.X + bh.Width) - bn.X
		bn.X = bh.X + bh.Width
		sizePrev = bp.Width
		sizeNext = bn.Width
	} else {
		bp.Height = bh.Y - bp.Y
		bn.Height -= (bh.Y + bh.Height) - bn.Y
		bn.Y = bh.Y + bh.Height
		sizePrev = bp.Height
		sizeNext = bn.Height
	}

	if e := prev.SetBounds(bp); e != nil {
		return
	}

	if e := next.SetBounds(bn); e != nil {
		return
	}

	layout := s.Layout().(*splitterLayout)
	layout.hwnd2Item[prev.Handle()].size = sizePrev
	layout.hwnd2Item[next.Handle()].size = sizeNext

	return nil //s.ContainerBase.onInsertedWidget(index, widget)
}
func (s *Splitter) SetWidgetWidth(widget Widget, newVal int) (err error) {

	if !s.children.Contains(widget) {
		return
	}

	handleIndex := 1 //s.children.Index(dragHandle)
	hndl := s.children.At(handleIndex)
	prev := s.children.At(handleIndex - 1)
	next := s.children.At(handleIndex + 1)

	prev.SetSuspended(true)
	defer prev.Invalidate()
	defer prev.SetSuspended(false)
	next.SetSuspended(true)
	defer next.Invalidate()
	defer next.SetSuspended(false)
	diff := 0
	if s.Orientation() == Horizontal {
		if prev == widget {
			diff = prev.Width() - newVal
		} else {
			diff = next.Width() - newVal
		}
		hndl.SetX(hndl.X() + diff)
	} else {
		if prev == widget {
			diff = prev.Height() - newVal
		} else {
			diff = next.Height() - newVal
		}
		hndl.SetY(hndl.Y() + diff)
	}

	bh := hndl.Bounds()
	bp := prev.Bounds()
	bn := next.Bounds()

	var sizePrev int
	var sizeNext int

	if s.Orientation() == Horizontal {
		bp.Width = bh.X - bp.X
		bn.Width -= (bh.X + bh.Width) - bn.X
		bn.X = bh.X + bh.Width
		sizePrev = bp.Width
		sizeNext = bn.Width
	} else {
		bp.Height = bh.Y - bp.Y
		bn.Height -= (bh.Y + bh.Height) - bn.Y
		bn.Y = bh.Y + bh.Height
		sizePrev = bp.Height
		sizeNext = bn.Height
	}

	if e := prev.SetBounds(bp); e != nil {
		return
	}

	if e := next.SetBounds(bn); e != nil {
		return
	}

	layout := s.Layout().(*splitterLayout)
	layout.hwnd2Item[prev.Handle()].size = sizePrev
	layout.hwnd2Item[next.Handle()].size = sizeNext

	return nil //s.ContainerBase.onInsertedWidget(index, widget)
}
func (s *Splitter) SetWidgetVisible(widget Widget, bVisible bool) (err error) {

	if !s.children.Contains(widget) {
		return
	}

	handleIndex := 1 //s.children.Index(dragHandle)
	hndl := s.children.At(handleIndex)
	prev := s.children.At(handleIndex - 1)
	next := s.children.At(handleIndex + 1)

	var sizePrev, sizeNext int
	var bh, bp, bn Rectangle

	layout := s.Layout().(*splitterLayout)

	bh = hndl.Bounds()
	log.Println("SetWidgetVisible, bVisible=", bVisible)

	sp := layout.hwnd2Item[prev.Handle()]
	sn := layout.hwnd2Item[next.Handle()]
	log.Println("SetWidgetVisible, prev scale factor", sp.visScale, sp.stretchFactor, sp.growth)
	log.Println("SetWidgetVisible, next scale factor", sn.visScale, sn.stretchFactor, sn.growth)

	if bVisible == false {
		bp = prev.Bounds()
		bn = next.Bounds()

		//record scale factors
		layout.hwnd2Item[prev.Handle()].visScale = float64(bp.Width) / float64(bp.Width+bh.Width+bn.Width)
		//layout.hwnd2Item[next.Handle()].visScale = float64(bn.Width) / float64(bp.Width+bh.Width+bn.Width)

		if widget == prev {
			bp.X = 0
			bp.Width = 0

			bh.X = 0

			bn.X = 0
			bn.Width = s.Width()
		} else {
			bp.X = 0
			bp.Width = s.Width()

			bh.X = bp.Width

			bn.X = 0
			bn.Width = 0
		}
		sizePrev = bp.Width
		sizeNext = bn.Width

	} else {
		sp := layout.hwnd2Item[prev.Handle()]
		//sn := layout.hwnd2Item[next.Handle()].visScale

		log.Println("SetWidgetVisible, scale factor", sp.visScale, sp.stretchFactor, sp.growth)

		bp.X = 0
		bp.Width = int(float64(s.Width()) * sp.visScale)

		bh.X = bp.Width

		bn.X = bh.X + bh.Width
		bn.Width = s.Width() - bp.Width - hndl.Width()

		sizePrev = bp.Width
		sizeNext = bn.Width
	}

	prev.SetBounds(bp)
	hndl.SetBounds(bh)
	next.SetBounds(bn)

	hndl.SetVisible(bVisible)
	widget.SetVisible(bVisible)

	//	diff := 0
	//	if s.Orientation() == Horizontal {
	//		if prev == widget {
	//			if !bVisible {
	//				diff = -prev.Width()
	//			} else {
	//				diff = prev.Width()
	//			}
	//		} else {
	//			if !bVisible {
	//				diff = next.Width()
	//			} else {
	//				diff = -next.Width()
	//			}
	//		}
	//		hndl.SetX(hndl.X() + diff)
	//	} else {
	//		if prev == widget {
	//			diff = -prev.Height()
	//		} else {
	//			diff = next.Height()
	//		}
	//		hndl.SetY(hndl.Y() + diff)
	//	}

	//	bh := hndl.Bounds()
	//	bp := prev.Bounds()
	//	bn := next.Bounds()

	//	hndl.SetVisible(bVisible)
	//	widget.SetVisible(bVisible)

	//	if s.Orientation() == Horizontal {
	//		bp.Width = bh.X - bp.X
	//		bn.Width -= (bh.X + bh.Width) - bn.X
	//		bn.X = bh.X + bh.Width
	//		sizePrev = bp.Width
	//		sizeNext = bn.Width
	//	} else {
	//		bp.Height = bh.Y - bp.Y
	//		bn.Height -= (bh.Y + bh.Height) - bn.Y
	//		bn.Y = bh.Y + bh.Height
	//		sizePrev = bp.Height
	//		sizeNext = bn.Height
	//	}

	//	if e := prev.SetBounds(bp); e != nil {
	//		return
	//	}
	//	if e := next.SetBounds(bn); e != nil {
	//		return
	//	}

	layout.hwnd2Item[prev.Handle()].size = sizePrev
	layout.hwnd2Item[next.Handle()].size = sizeNext

	//log.Println("sizePrev,sizeNext", sizePrev, sizeNext)
	return nil
}
