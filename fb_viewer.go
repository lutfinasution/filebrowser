package main

import (
	"log"
)

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	//"github.com/lxn/win"
)

type ImageViewWindow struct {
	*walk.MainWindow
	viewerbase     *ImageViewer
	menuItemAction *walk.Menu
	menuKeepLoc    *walk.Action
	prevFilePath   string
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

func NewImageViewWindow(parent *walk.MainWindow, imageName string, imglist *FileInfoModel, synchfunc func(idx int)) int {

	var tvw = new(ImageViewWindow)

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

	if err := (MainWindow{
		AssignTo: &tvw.MainWindow,
		Name:     "ImageViewWindow",
		Title:    imageName,
		MinSize:  Size{400, 240},
		Size:     Size{800, 480},
		Layout:   VBox{Margins: Margins{Top: 0, Left: 0, Right: 0, Bottom: 0}, MarginsZero: true, SpacingZero: true},
		Children: []Widget{},
	}.Create()); err != nil {
		return 0
	}

	// launch imageviewer component
	tvw.viewerbase, _ = NewImageViewer(tvw.MainWindow, tvw.MainWindow, imageName, imglist)
	tvw.viewerbase.OnViewImage = synchfunc

	sbr := tvw.StatusBar()
	sbr.SetVisible(true)

	sbi := walk.NewStatusBarItem()
	sbi.SetWidth(500)
	sbr.Items().Add(sbi)

	br, _ := walk.NewSolidColorBrush(walk.RGB(20, 20, 20))
	tvw.StatusBar().SetBackground(br)
	tvw.MainWindow.SetBackground(br)

	ft, _ := walk.NewFont(tvw.MainWindow.Font().Family(), 10, walk.FontBold)
	tvw.StatusBar().SetFont(ft)

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

	tvw.viewerbase.SetContextMenu(menu)

	/*-----------------------------
	   START THE WINDOW MAIN LOOP
	------------------------------*/
	id := tvw.MainWindow.Run()

	tvw.viewerbase.Close()

	if err := settings.Save(); err != nil {
		log.Fatal(err)
	}
	log.Print("exit form...")
	return id
}
