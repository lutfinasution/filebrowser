// Copyright 2010 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package walk

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
func (s *Splitter) SetWidgetVisible(widget Widget, newVal bool) (err error) {

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
			diff = -prev.Width()
		} else {
			diff = next.Width()
		}
		hndl.SetX(hndl.X() + diff)
	} else {
		if prev == widget {
			diff = -prev.Height()
		} else {
			diff = next.Height()
		}
		hndl.SetY(hndl.Y() + diff)
	}

	bh := hndl.Bounds()
	bp := prev.Bounds()
	bn := next.Bounds()
	hndl.SetVisible(newVal)
	widget.SetVisible(newVal)
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
