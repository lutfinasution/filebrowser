// fb_customscrollcomposite.go
package main

import (
	"log"
	//"syscall"
	"unsafe"
)
import (
	"github.com/lxn/walk"
	"github.com/lxn/win"
)

type CustomScrollComposite struct {
	*walk.Composite
	host        *ScrollViewer
	scrollStep  int
	thumbscaler int //to be able to adjust thumb size, for easier grip
}

func NewCustomScrollComposite(parent walk.Container, svw *ScrollViewer) (*CustomScrollComposite, error) {
	var err error

	cmp := new(CustomScrollComposite)
	cmp.host = svw
	if cmp.Composite, err = walk.NewComposite(parent); err != nil {
		log.Fatal(err)
	}
	// enable vertical scrollbars
	wstyle := win.GetWindowLong(cmp.Handle(), win.GWL_STYLE)
	wstyle = wstyle | win.WS_VSCROLL | win.WS_TABSTOP
	win.SetWindowLong(cmp.Handle(), win.GWL_STYLE, wstyle)

	if err = walk.InitWrapperWindow(cmp); err != nil {
		log.Fatal(err)
	}
	return cmp, err
}

func (sv *CustomScrollComposite) MaxValue() int {
	var si win.SCROLLINFO
	si.CbSize = uint32(unsafe.Sizeof(si))
	si.FMask = win.SIF_PAGE | win.SIF_RANGE

	win.GetScrollInfo(sv.Handle(), win.SB_VERT, &si)

	return int(si.NMax - int32(si.NPage))
}
func (sv *CustomScrollComposite) Value() int {
	var si win.SCROLLINFO
	si.CbSize = uint32(unsafe.Sizeof(si))
	si.FMask = win.SIF_POS

	win.GetScrollInfo(sv.Handle(), win.SB_VERT, &si)

	return int(si.NPos)
}
func (sv *CustomScrollComposite) SetValue(val int) int {
	var si win.SCROLLINFO
	si.CbSize = uint32(unsafe.Sizeof(si))
	si.FMask = win.SIF_POS
	si.NPos = int32(val)

	pos := win.SetScrollInfo(sv.Handle(), win.SB_VERT, &si, true)

	return int(pos)
}
func (sv *CustomScrollComposite) updateScrollbar(max, page, step, scale int) bool {
	var si win.SCROLLINFO

	si.CbSize = uint32(unsafe.Sizeof(si))
	si.FMask = win.SIF_PAGE | win.SIF_RANGE
	si.NMin = 0
	si.NMax = int32(max + page*scale)
	si.NPage = uint32(page * scale)
	//si.NPos = int32(sv.scroller.Value())

	sv.scrollStep = step
	sv.thumbscaler = scale
	win.SetScrollInfo(sv.Handle(), win.SB_VERT, &si, true)
	return true
}
func (sv *CustomScrollComposite) WndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	if sv.Composite != nil {
		switch msg {
		case win.WM_VSCROLL:
			//log.Println("WndProc: WM_VSCROLL", wParam)
			val := -sv.scroll(win.SB_VERT, win.LOWORD(uint32(wParam)))
			if win.LOWORD(uint32(wParam)) != win.SB_ENDSCROLL {
				sv.host.SetScroll(val)
			}
		case win.WM_MOUSEWHEEL:
			var cmd uint16
			if delta := int16(win.HIWORD(uint32(wParam))); delta < 0 {
				cmd = win.SB_LINEDOWN
			} else {
				cmd = win.SB_LINEUP
			}
			sv.host.SetScroll(-sv.scroll(win.SB_VERT, cmd))

			return 0
			//		case win.WM_CHAR:
			//			log.Println("WndProc: WM_CHAR", wParam)

			//		case win.WM_KEYDOWN:
			//			log.Println("WndProc: WM_KEYDOWN", wParam)

		case win.WM_GETDLGCODE:
			return win.DLGC_WANTALLKEYS | win.DLGC_WANTARROWS
		}
	}

	return sv.WidgetBase.WndProc(hwnd, msg, wParam, lParam)
}

func (sv *CustomScrollComposite) scroll(sb int32, cmd uint16) int {
	var pos int32
	var si win.SCROLLINFO
	si.CbSize = uint32(unsafe.Sizeof(si))
	si.FMask = win.SIF_PAGE | win.SIF_POS | win.SIF_RANGE | win.SIF_TRACKPOS

	win.GetScrollInfo(sv.Handle(), sb, &si)

	pos = si.NPos

	switch cmd {
	case win.SB_LINEUP: // == win.SB_LINELEFT
		pos -= int32(sv.scrollStep)

	case win.SB_LINEDOWN: // == win.SB_LINERIGHT
		pos += int32(sv.scrollStep)

	case win.SB_PAGEUP: // == win.SB_PAGELEFT
		pos -= int32(si.NPage / uint32(sv.thumbscaler))

	case win.SB_PAGEDOWN: // == win.SB_PAGERIGHT
		pos += int32(si.NPage / uint32(sv.thumbscaler))

	case win.SB_THUMBTRACK:
		pos = si.NTrackPos
	}

	if pos < 0 {
		pos = 0
	}
	if pos > si.NMax+1-int32(si.NPage) {
		pos = si.NMax + 1 - int32(si.NPage)
	}

	si.FMask = win.SIF_POS
	si.NPos = pos
	win.SetScrollInfo(sv.Handle(), sb, &si, true)

	return -int(pos)
}
