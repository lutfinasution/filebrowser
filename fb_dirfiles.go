package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

type FileInfo struct {
	Name           string
	index          int
	Size           int64
	Modified       time.Time
	Type           string
	checked        bool
	Changed        bool
	Locked         bool
	dbsynched      bool
	Width, Height  int
	thumbW, thumbH int
	ModState       string
	Info           string
	drawRect       walk.Rectangle
	Imagedata      []byte
}

func (f FileInfo) HasData() bool {
	return (f.Imagedata != nil)
}

type FileInfoModel struct {
	walk.SortedReflectTableModelBase
	viewer  *ScrollViewer
	dirPath string
	items   []*FileInfo
}

func NewFileInfoModel() *FileInfoModel {
	m := new(FileInfoModel)
	return m
}

func (f FileInfoModel) getFullPath(idx int) string {

	v := f.items[idx]

	if v.Info == "" {
		return filepath.Join(f.dirPath, v.Name)
	} else {
		return filepath.Join(v.Info, v.Name)
	}
}

func (f FileInfoModel) getFullItemPath(item *FileInfo) string {

	for _, v := range f.items {
		if v == item {
			if v.Info == "" {
				return filepath.Join(f.dirPath, v.Name)
			} else {
				return filepath.Join(v.Info, v.Name)
			}
		}
	}
	return ""
}
func (m *FileInfoModel) Items() interface{} {
	return m.items
}

// Called by the TableView to retrieve if a given row is checked.
func (m *FileInfoModel) Checked(row int) bool {
	return m.items[row].checked
}

// Called by the TableView when the user toggled the check box of a given row.
func (m *FileInfoModel) SetChecked(row int, checked bool) error {
	m.items[row].checked = checked

	return nil
}

func (m *FileInfoModel) Image(row int) interface{} {
	return filepath.Join(m.dirPath, m.items[row].Name)
}

func (m *FileInfoModel) BrowsePath(dirPath string, doHistory bool) error {
	if doHistory {
		AppSetDirSettings(m.viewer, m.dirPath)
	}

	m.dirPath = dirPath
	m.items = nil

	if err := filepath.Walk(dirPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if info == nil {
					return filepath.SkipDir
				}
			}
			name := info.Name()

			if path == dirPath || shouldExclude(name) {
				return nil
			}
			url := filepath.Join(dirPath, name)
			imgType := filepath.Ext(name)
			imgInfo := walk.Size{0, 0}

			//Retrieve image dimension, etc based on type
			switch imgType {
			case
				".bmp", ".gif", ".jpg", ".jpeg", ".png", ".tif", ".tiff", ".webp":
				imgInfo, err = GetImageInfo(url)

				item := &FileInfo{
					Name:     name,
					Size:     info.Size(),
					Modified: info.ModTime(),
					Type:     imgType,
					Width:    imgInfo.Width,
					Height:   imgInfo.Height,
					Changed:  false,
				}
				m.items = append(m.items, item)
			}
			if info.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}); err != nil {
		return err
	}

	m.PublishRowsReset()

	return nil
}

func (m *FileInfoModel) SetDirPath(dirPath string, doHistory bool) error {
	if doHistory {
		AppSetDirSettings(m.viewer, m.dirPath)
	}

	m.dirPath = dirPath
	m.items = nil

	if err := filepath.Walk(dirPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if info == nil {
					return filepath.SkipDir
				}
			}
			name := info.Name()

			if path == dirPath || shouldExclude(name) {
				return nil
			}
			url := filepath.Join(dirPath, name)
			imgType := filepath.Ext(name)
			imgInfo := walk.Size{0, 0}

			//Retrieve image dimension, etc based on type
			switch imgType {
			case
				".bmp", ".gif", ".jpg", ".jpeg", ".png", ".tif", ".tiff", ".webp":
				imgInfo, err = GetImageInfo(url)

				item := &FileInfo{
					Name:     name,
					Size:     info.Size(),
					Modified: info.ModTime(),
					Type:     imgType,
					Width:    imgInfo.Width,
					Height:   imgInfo.Height,
					Changed:  false,
				}
				m.items = append(m.items, item)
			}
			if info.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}); err != nil {
		return err
	}

	m.PublishRowsReset()

	numItems := len(m.items)
	if numItems > 0 {
		//create map containing the file infos
		Mw.thumbView.Run(dirPath, m, true)
	} else {
		Mw.thumbView.SetItemsCount(0)
	}

	Mw.StatusBar().Invalidate()
	Mw.MainWindow.SetTitle(dirPath + " (" + strconv.Itoa(numItems) + " files)")
	Mw.UpdateAddreebar(dirPath)
	log.Println("Files in path: ", numItems)

	return nil
}
func AppSetDirSettings(sv *ScrollViewer, dirPath string) {
	settings.Put(dirPath, strconv.Itoa(sv.viewInfo.topPos))
}
func AppGetDirSettings(sv *ScrollViewer, dirPath string) {
	if s, ok := settings.Get(dirPath); ok {
		val, _ := strconv.Atoi(s)

		sv.SetScrollPos(val)
	}
}

type DirectoryMonitor struct {
	viewer        *ScrollViewer
	watchmap      ItmMap
	lastwatchpath string
	watchdone     chan int
	watchwait     sync.WaitGroup
	watchevents   int64
	watchActive   bool
	watcher       *fsnotify.Watcher
	activeproc    bool
	infofunc      func(dir string)
	imagemon      *ContentMonitor
}

func (dm *DirectoryMonitor) FSsetNewItem(mkey string) {

	info, err := os.Lstat(mkey)
	if err != nil {
		return
	}

	name := info.Name()
	imgType := filepath.Ext(name)
	imgInfo := walk.Size{0, 0}

	//Retrieve image dimension, etc based on type
	switch imgType {
	case
		".gif", ".jpg", ".jpeg", ".png", ".webp":
		imgInfo, err = GetImageInfo(mkey)

		//if item already exists,
		if v, ok := dm.viewer.ItemsMap[mkey]; ok {
			v.Size = info.Size()
			v.Modified = info.ModTime()
			v.Width = imgInfo.Width
			v.Height = imgInfo.Height
		} else {
			dm.viewer.ItemsMap[mkey] = &FileInfo{
				Name:     name,
				Size:     info.Size(),
				Modified: info.ModTime(),
				Type:     imgType,
				Width:    imgInfo.Width,
				Height:   imgInfo.Height,
				ModState: "",
				//Changed:   true,
			}
			//adding new item to list
			dm.viewer.itemsModel.items = append(dm.viewer.itemsModel.items, dm.viewer.ItemsMap[mkey])
			dm.viewer.SetItemsCount(len(dm.viewer.itemsModel.items))
		}
		dm.imagemon.submitChangedItem(mkey, dm.viewer.ItemsMap[mkey])

		log.Println("FSsetNewItem: ", mkey, dm.viewer.ItemsMap[mkey])
	}
}

func (dm *DirectoryMonitor) FSremoveItem(mkey string, wasRenamed bool) {
	_, ok := dm.viewer.ItemsMap[mkey]
	if ok {
		delete(dm.viewer.ItemsMap, mkey)

		m := dm.viewer.itemsModel.items
		name := filepath.Base(mkey)
		for i := range m {
			if m[i].Name == name {
				m[i] = m[len(m)-1]

				//remove item from list
				dm.viewer.itemsModel.items = m[:len(m)-1]
				dm.viewer.SetItemsCount(len(dm.viewer.itemsModel.items))

				if !wasRenamed {
					log.Println("FSremoveItem: ", mkey)
				} else {
					log.Println("FSrenameItem --> FSremoveItem: ", mkey)
				}
				break
			}
		}
	}
}

func (dm *DirectoryMonitor) FSrenameItem(mkey string) {
	dm.FSremoveItem(mkey, true)
}

func (dm *DirectoryMonitor) processWatcher() bool {
	var t time.Time
	var i int64
	timer := time.NewTicker(time.Millisecond * 1)
	hasEvent := false
	res := false
	log.Println("Starting watch processor ", time.Now())
loop:
	for {
		select {
		case <-dm.watchdone:
			res = false
			break loop
		default:
			<-timer.C
			//continue watching, reduce watchevent counter by 1
			if dm.watchevents > 0 {
				i = atomic.AddInt64(&dm.watchevents, -1)

				hasEvent = true
				t = time.Now()
				log.Println("Event detected at ", t, i)
			}

			//exit loop when counter is 0
			//delay by 3 sec
			if hasEvent && (dm.watchevents == 0) {
				if time.Since(t) > time.Second*3 {
					log.Println("Event detection expire at ", time.Now())
					res = true
					break loop
				}
			}
		}
	}

	timer.Stop()
	dm.watchwait.Done()

	if hasEvent {
		log.Println("processWatcher found", len(dm.watchmap))

		for k, v := range dm.watchmap {
			switch v.ModState {
			case "modify", "create":
				dm.FSsetNewItem(k)
			case "remove":
				dm.FSremoveItem(k, false)
			case "rename":
				dm.FSrenameItem(k)
			}
			delete(dm.watchmap, k)
		}

		if dm.viewer != nil {
			//this will handle changes in the underlying data
			dm.viewer.directoryMonitorInfoHandler(dm.lastwatchpath)
		}
	}
	dm.activeproc = false
	log.Println("Closing watch processor ", time.Now())

	return res
}

func dorecover() {
	recover()
	log.Printf("recovering")
}
func (dm *DirectoryMonitor) Close() {

	if dm.watchActive {
		dm.watchdone <- 1
		dm.watchwait.Wait()

		dm.watcher.Remove(dm.lastwatchpath)
		dm.watchActive = false
		log.Println("closing watch on last: ", dm.lastwatchpath)
	}
}
func (dm *DirectoryMonitor) setFolderWatcher(watchpath string) {
	var err error

	if dm.activeproc {
		return
	}

	if dm.watchActive {
		log.Printf("setFolderWatcher entering...")

		if (dm.lastwatchpath == watchpath) || (watchpath == "") {
			log.Printf("skip watch, same path")
			return
		}

		log.Println("attempting to close watch on last: ", dm.lastwatchpath, dm.activeproc)

		dm.watchwait.Wait()

		err = dm.watcher.Remove(dm.lastwatchpath)
		dm.watchActive = false
		log.Println("closing watch on last: ", dm.lastwatchpath)
	}

	if watchpath == "" {
		dm.watchActive = false
		dm.lastwatchpath = watchpath
		log.Printf("skip watch, empty path")
		return
	}

	if dm.watcher == nil {
		dm.watcher, err = fsnotify.NewWatcher()
		if err != nil {
			log.Fatal(err)
		}
	}

	dm.watchevents = 0
	go func() {
		for {
			select {
			case event := <-dm.watcher.Events:
				hasEvent := false
				evType := ""
				if event.Op&fsnotify.Write == fsnotify.Write {
					evType = "modify"
					hasEvent = true
				}
				if event.Op&fsnotify.Create == fsnotify.Create {
					evType = "create"
					hasEvent = true
				}
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					evType = "remove"
					hasEvent = true
				}
				if event.Op&fsnotify.Rename == fsnotify.Rename {
					evType = "rename"
					hasEvent = true
				}

				if hasEvent {
					dm.watchevents += 1
					log.Println(evType+": ", event.Name)

					if dm.watchmap == nil {
						dm.watchmap = make(ItmMap)
					}

					dm.watchmap[event.Name] = &FileInfo{Name: event.Name, ModState: evType}

					if !dm.activeproc {
						dm.activeproc = true
						dm.watchwait.Add(1)
						dm.watchdone = make(chan int, 1)

						go dm.processWatcher()
					}
				}

			case err := <-dm.watcher.Errors:
				if err != nil {
					log.Println("error:", err)
				}
			}
		}
	}()

	err = dm.watcher.Add(watchpath)
	dm.lastwatchpath = watchpath
	if err != nil {
		log.Fatal(err)
	}

	dm.watchActive = true
	log.Printf("starting watch on " + watchpath)
}

func shouldExclude(name string) bool {
	switch name {
	case "System Volume Information", "pagefile.sys", "swapfile.sys":
		return true
		//	case "$RECYCLE.BIN":
		//		return true
	}

	return false
}
