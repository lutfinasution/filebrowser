// fb_processor.go
package main

import (
	//"fmt"
	//"image"
	"log"
	//"math"
	"path/filepath"
	"runtime"
	//"strconv"
	"sync"
	"sync/atomic"
	"time"
	//"unsafe"
)

import (
	"github.com/lxn/walk"
)

type workitem struct {
	path string
	done bool
}
type WorkMap map[string]*workitem
type ImageProcessor struct {
	workmap       WorkMap
	doCancelation bool
	donewait      sync.WaitGroup
	workerWaiter  sync.WaitGroup
	imageWorkChan []chan string
	workStatus    *ProgresDrawer
	workCounter   uint64
	statuswidget  *walk.StatusBar
	statusfunc    func(i int)
	infofunc      func(numjob int, d float64)
}

func (ip *ImageProcessor) setstatuswidget(widget *walk.StatusBar) {
	ip.statuswidget = widget
}
func (ip *ImageProcessor) Close(sv *ScrollViewer) bool {

	ip.donewait.Add(1)
	ip.workCounter = 0
	gorCount := runtime.NumCPU()

	go func() {
		log.Println("Terminating all ImageProcessor goroutines")

		//setup waiters
		ip.workerWaiter.Add(gorCount)

		for i := 0; i < gorCount; i++ {
			key := ""

			//send exit data through
			//worker channel
			ip.imageWorkChan[0] <- key
		}
		//wait for all workers to finish
		ip.workerWaiter.Wait()
		ip.donewait.Done()

		log.Println("ImageProcessor goroutines all terminated")

	}()

	return true
}
func (ip *ImageProcessor) Run(sv *ScrollViewer, jobList []*FileInfo, dirPaths []string) bool {
	runtime.GC()
	sv.handleChangedItems = false

	if ip == nil {
		return false
	}

	if ip.workmap == nil {
		ip.workmap = make(WorkMap)
	}

	if len(ip.workmap) > 0 {
		ip.doCancelation = true
		ip.donewait.Wait()
		ip.doCancelation = false
	}

	numJob := len(jobList)

	//add work items to workmap
	for _, v := range jobList {
		if v.Info == "" {
			ip.workmap[filepath.Join(dirPaths[0], v.Name)] = &workitem{path: "x"} //dirpath
		} else {
			ip.workmap[filepath.Join(v.Info, v.Name)] = &workitem{path: "x"}
		}
	}

	//determine the num of worker goroutines
	gorCount := runtime.NumCPU()
	ip.donewait.Add(1)
	ip.workCounter = 0

	if ip.imageWorkChan == nil {
		ip.imageWorkChan = make([]chan string, gorCount)
		for i := range ip.imageWorkChan {
			ip.imageWorkChan[i] = make(chan string)
		}
		//--------------------------
		//run the worker goroutines
		//--------------------------
		for j := 0; j < gorCount; j++ {
			//go ip.doRenderTasker(sv, j+1, ip.imageWorkChan[j])    //with individual channel
			go ip.doRenderTasker(sv, j+1, ip.imageWorkChan[0]) //with just one shared channel
		}
	}

	go func(itms WorkMap) {
		t := time.Now()

		getNextItem := func() (res string) {
			for key, _ := range itms {
				res = key
				delete(itms, key)
				break
			}
			return res
		}

		//Load data from cache
		n := sv.CacheDBEnum(dirPaths)

		log.Println("CacheDBEnum found", n, "in", dirPaths)

		if ip.statuswidget != nil {
			ip.workStatus = NewProgresDrawer(ip.statuswidget.AsWidgetBase(), 240, numJob)
		}

		//setup waiters
		ip.workerWaiter.Add(len(itms))

		//run the distributor
		i := 0

	loop:
		for {
			if !ip.doCancelation {
				n := 0
				key := getNextItem()
				if key != "" {
					//send data through worker channel
					ip.imageWorkChan[n] <- key

					if ip.statusfunc != nil {
						ip.statusfunc(i)
					}
					i++
				} else {
					break loop
				}
			}

			if i%4 == 0 {
				if ip.workStatus != nil {
					ip.workStatus.DrawProgress(i)
				}
			}
		}

		if ip.infofunc != nil {
			d := time.Since(t).Seconds()
			ip.infofunc(numJob, d)

			sv.recalc()
		}

		//wait for all workers to finish
		ip.workerWaiter.Wait()

		if ip.workStatus != nil {
			ip.workStatus.Clear()
		}

		//update db for items in this path only
		cntupd, _ := sv.CacheDBUpdateMapItems(sv.ItemsMap, dirPaths)

		sv.canvasView.Synchronize(func() {
			sv.canvasView.Invalidate()
		})

		sv.handleChangedItems = true

		if !ip.doCancelation {
			wc := atomic.LoadUint64(&ip.workCounter)
			log.Println("Cache items processed: ", wc)
			log.Println("Cache items updated: ", cntupd)
		}
		ip.donewait.Done()
	}(ip.workmap)

	return true
}

func (ip *ImageProcessor) doRenderTasker(sv *ScrollViewer, id int, fnames chan string) bool {
	//icount := 0
loop:
	for v := range fnames {
		if v != "" {
			if processImageData(sv, v, true, nil) != nil {
				atomic.AddUint64(&ip.workCounter, 1)
			}

			//decrement the wait counter
			ip.workerWaiter.Done()
		} else {
			log.Println("doRenderTasker exiting..this-should-not-have-happened.")
			break loop
		}
	}
	return true
}

type ContentMonitor struct {
	imageprocessor *ImageProcessor
	changeMap      ItmMap
	doneMap        ItmMap
	activated      bool
	infofunc       func()
	itmMutex       sync.Mutex
	runMutex       sync.Mutex
	statuswidget   *walk.StatusBar
}

func (im *ContentMonitor) setstatuswidget(widget *walk.StatusBar) {
	im.statuswidget = widget
}
func (im *ContentMonitor) removeChangedItem(mkey string) {
	if im.changeMap != nil {
		if _, ok := im.changeMap[mkey]; ok {
			delete(im.changeMap, mkey)
		}
	}
}
func (im *ContentMonitor) removeChangedItems(cmp ItmMap) {
	if cmp != nil {
		for k, _ := range cmp {
			delete(cmp, k)
		}
	}
}
func (im *ContentMonitor) submitChangedItem(mkey string, cItm *FileInfo) {
	if im.changeMap == nil {
		im.changeMap = make(ItmMap)
	}
	if im.doneMap == nil {
		im.doneMap = make(ItmMap)
	}

	//new item must not already be in the doneMap & changeMap
	im.itmMutex.Lock()
	defer im.itmMutex.Unlock()

	if cItm != nil {
		if _, ok := im.doneMap[mkey]; !ok {
			if _, ok := im.changeMap[mkey]; !ok {
				im.changeMap[mkey] = cItm
				im.changeMap[mkey].Changed = true

				log.Println("submitChangedItem: ", mkey)
			}
		}
	} else {
		log.Println("submitChangedItem, cItm == nil", cItm, mkey)
	}
}

func (im *ContentMonitor) processChangedItem(sv *ScrollViewer, repaint bool) {
	if im.changeMap == nil {
		return
	}
	if len(im.changeMap) == 0 {
		return
	}
	if !im.activated {
		//copy changeMap to a string slice
		//important to stability
		im.activated = true

		var worklist []string
		for key, _ := range im.changeMap {
			worklist = append(worklist, key)
		}

		go func(workitmlist []string) {
			ires := 0
			//im.activated = true
			log.Println("processChangedItem ---------------------------------")

			jobStatus := NewProgresDrawer(im.statuswidget.AsWidgetBase(), 100, len(workitmlist))

			if im.imageprocessor.imageWorkChan != nil {
				im.imageprocessor.workerWaiter.Add(len(workitmlist))

				for i, key := range workitmlist {
					//send data to workers by writing
					//to the common channel

					//key shouldn't already be in doneMap
					if _, ok := im.doneMap[key]; !ok {

						im.imageprocessor.imageWorkChan[0] <- key

						im.itmMutex.Lock()
						im.doneMap[key] = &FileInfo{Name: key}
						im.itmMutex.Unlock()

						v := im.doneMap[key]
						v.dbsynched = false

						if jobStatus != nil {
							jobStatus.DrawProgress(i)
						}
						ires = i
					}
				}
				im.imageprocessor.workerWaiter.Wait()
			}
			if len(workitmlist) > 0 {
				n, _ := sv.CacheDBUpdateMapItems(im.doneMap, []string{""})

				im.itmMutex.Lock()
				im.removeChangedItems(im.changeMap)
				im.itmMutex.Unlock()

				if repaint && ires > 0 {
					if im.infofunc != nil {
						im.infofunc()
					}
					sv.canvasView.Synchronize(func() {
						sv.canvasView.Invalidate()
					})
				}

				log.Println("processChangedItem/processImageData: ", ires+1)
				log.Println("processChangedItem/CacheDBUpdateMapItems: ", n)
				jobStatus.Clear()
			}

			im.activated = false
		}(worklist)
	}
}
