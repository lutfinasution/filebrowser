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
	Name          string
	Size          int64
	Modified      time.Time
	Type          string
	checked       bool
	Changed       bool
	Locked        bool
	Width, Height int
	LastState     string
	Imagedata     []byte
}

func (f FileInfo) HasData() bool {
	return (f.Imagedata != nil)
}

type FileInfoModel struct {
	walk.SortedReflectTableModelBase
	sortOrder walk.SortOrder
	dirPath   string
	items     []*FileInfo
}

func (f FileInfoModel) getFullPath(idx int) string {
	return filepath.Join(f.dirPath, f.items[idx].Name)
}

type ItmMap map[string]*FileInfo

var ItemsMap ItmMap

func NewFileInfoModel() *FileInfoModel {
	m := new(FileInfoModel)
	return m
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

func (m *FileInfoModel) SetDirPath(dirPath string) error {
	m.dirPath = dirPath
	m.items = nil

	if err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
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
			".gif", ".jpg", ".jpeg", ".png", ".webp":
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
		//--------------------------------------
		//create map containing the file infos
		//--------------------------------------
		if ItemsMap == nil {
			ItemsMap = make(map[string]*FileInfo)
		}

		for i := range m.items {
			fn := filepath.Join(dirPath, m.items[i].Name)

			if mp, ok := ItemsMap[fn]; !ok {
				ItemsMap[fn] = m.items[i]
			} else {
				mp.Changed = (m.items[i].Size != mp.Size) || (m.items[i].Modified != mp.Modified)
				mp.Size = m.items[i].Size
				mp.Modified = m.items[i].Modified
			}
		}

		//launch image cache setup
		setImgcache(numItems, dirPath)

		//setup folder watcher
		setFolderWatcher(dirPath)
	} else {
		//setup folder watcher
		setFolderWatcher("")
	}

	//Updating the paintwidget height & its container's height to reflect the num of items
	Mw.thumbView.SetItemCount(numItems)
	Mw.thumbView.ResetPos()

	Mw.pgBar.SetValue(0)
	Mw.MainWindow.SetTitle(dirPath + " (" + strconv.Itoa(numItems) + " files)")
	Mw.UpdateAddreebar(dirPath)
	log.Println("Files in path: ", numItems)

	return nil
}

type fsWatcher struct {
	lastwatchpath string
	watchdone     chan int
	watchwait     sync.WaitGroup
	watchevents   int64
	watchActive   bool
	watcher       *fsnotify.Watcher
	activeproc    bool
}

var fsw fsWatcher
var watchmap ItmMap

func FSsetNewItem(mkey string) {
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

		ItemsMap[mkey] = &FileInfo{
			Name:     name,
			Size:     info.Size(),
			Modified: info.ModTime(),
			Type:     imgType,
			Width:    imgInfo.Width,
			Height:   imgInfo.Height,
			//Changed:   true,
			LastState: "",
		}

		tableModel.items = append(tableModel.items, ItemsMap[mkey])
		tableModel.PublishRowsReset()

		submitChangedItem(mkey, ItemsMap[mkey])
		processChangedItem()

		numItems := len(tableModel.items)
		Mw.thumbView.SetItemCount(numItems)
		Mw.MainWindow.SetTitle(filepath.Dir(mkey) + " (" + strconv.Itoa(numItems) + " files)")

		submitChangedItem(mkey, ItemsMap[mkey])
		processChangedItem()

		log.Println("FSsetNewItem: ", mkey, ItemsMap[mkey])
	}
}

func FSremoveItem(mkey string, wasRenamed bool) {
	_, ok := ItemsMap[mkey]
	if ok {
		delete(ItemsMap, mkey)

		m := tableModel.items
		name := filepath.Base(mkey)
		for i := range m {
			if m[i].Name == name {
				m[i] = m[len(m)-1]
				tableModel.items = m[:len(m)-1]
				tableModel.PublishRowsReset()

				if !wasRenamed {
					log.Println("FSremoveItem: ", mkey)

					numItems := len(tableModel.items)
					Mw.thumbView.SetItemCount(numItems)
					Mw.MainWindow.SetTitle(filepath.Dir(mkey) + " (" + strconv.Itoa(numItems) + " files)")
				} else {
					log.Println("FSrenameItem --> FSremoveItem: ", mkey)
				}
				break
			}
		}
	}
}

func FSrenameItem(mkey string) {
	FSremoveItem(mkey, true)
}

func processWatcher(wfs *fsWatcher) bool {
	var t time.Time
	var i int64
	timer := time.NewTicker(time.Millisecond * 1)
	hasEvent := false
	res := false
	log.Println("Starting watch processor ", time.Now())
loop:
	for {
		select {
		case <-wfs.watchdone:
			res = false
			break loop
		default:
			<-timer.C
			//continue watching, reduce watchevent counter by 1
			if wfs.watchevents > 0 {
				i = atomic.AddInt64(&wfs.watchevents, -1)

				hasEvent = true
				t = time.Now()
				log.Println("Event detected at ", t, i)
			}

			//exit loop when counter is 0
			//delay by 3 sec
			if hasEvent && (wfs.watchevents == 0) {
				if time.Since(t) > time.Second*3 {
					log.Println("Event detection expire at ", time.Now())
					res = true
					break loop
				}
			}
		}
	}

	timer.Stop()
	wfs.watchwait.Done()

	if hasEvent {
		//tableModel.SetDirPath(tableModel.dirPath)

		for k, v := range watchmap {
			switch v.LastState {
			case "modify", "create":
				FSsetNewItem(k)
			case "remove":
				FSremoveItem(k, false)
			case "rename":
				FSrenameItem(k)
			}
			delete(watchmap, k)
		}

	}
	fsw.activeproc = false
	log.Println("Closing watch processor ", time.Now())

	return res
}

func dorecover() {
	recover()
	log.Printf("recovering")
}

func setFolderWatcher(watchpath string) {
	var err error
	if fsw.activeproc {
		return
	}

	if fsw.watchActive {
		log.Printf("setFolderWatcher entering...")

		if (fsw.lastwatchpath == watchpath) || (watchpath == "") {
			log.Printf("skip watch, same path")
			return
		}

		log.Println("attempting to close watch on last: ", fsw.lastwatchpath, fsw.activeproc)

		//if fsw.watchdone != nil {
		//		if fsw.activeproc {
		//			defer dorecover()
		//			close(fsw.watchdone)
		//		}
		fsw.watchwait.Wait()

		err = fsw.watcher.Remove(fsw.lastwatchpath)
		fsw.watchActive = false
		log.Println("closing watch on last: ", fsw.lastwatchpath)
	}

	if watchpath == "" {
		fsw.watchActive = false
		fsw.lastwatchpath = watchpath
		log.Printf("skip watch, empty path")
		return
	}

	if fsw.watcher == nil {
		fsw.watcher, err = fsnotify.NewWatcher()
		if err != nil {
			log.Fatal(err)
		}
	}

	fsw.watchevents = 0
	go func() {
		for {
			select {
			case event := <-fsw.watcher.Events:
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
					fsw.watchevents += 1
					log.Println(evType+": ", event.Name)

					if watchmap == nil {
						watchmap = make(ItmMap)
					}

					watchmap[event.Name] = &FileInfo{Name: event.Name, LastState: evType}

					if !fsw.activeproc {
						fsw.activeproc = true
						fsw.watchwait.Add(1)
						fsw.watchdone = make(chan int, 1)

						go processWatcher(&fsw)
					}
				}

			case err := <-fsw.watcher.Errors:
				if err != nil {
					log.Println("error:", err)
				}
			}
		}
	}()

	err = fsw.watcher.Add(watchpath)
	fsw.lastwatchpath = watchpath
	if err != nil {
		log.Fatal(err)
	}

	fsw.watchActive = true
	log.Printf("starting watch on " + watchpath)
}

func shouldExclude(name string) bool {
	switch name {
	case "System Volume Information", "pagefile.sys", "swapfile.sys":
		return true
	}

	return false
}

func OnTableSelectedIndexesChanged() {
	//fmt.Printf("SelectedIndexes: %v\n", tableView.SelectedIndexes())
}

func OnTableCurrentIndexChanged() {
	var url string
	if index := tableView.CurrentIndex(); index > -1 {
		name := tableModel.items[index].Name

		dir := tableModel.dirPath
		url = filepath.Join(dir, name)

		//		switch filepath.Ext(name) {
		//		case
		//			".jpg", ".jpeg":
		//RenderImage(url)
		//		}

		//Mw.paintWidget.Invalidate()

	}
	Mw.MainWindow.SetTitle(url)
}
