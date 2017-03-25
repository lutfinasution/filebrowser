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

type workinfo struct {
	name     string
	item     *FileInfo
	skipdone bool
	//	dummy1     bool
	//	dummy2     bool
	//	dummy3     bool
	//dummy1 [3]byte
}
type WorkMap map[string]int
type ImageProcessor struct {
	Active        bool
	doCancelation bool
	donewait      sync.WaitGroup
	workerWaiter  sync.WaitGroup
	workSlice     []workinfo
	imageWorkChan []chan workinfo
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
			ip.imageWorkChan[0] <- workinfo{name: key}
		}
		//wait for all workers to finish
		ip.workerWaiter.Wait()
		ip.donewait.Done()

		log.Println("ImageProcessor goroutines all terminated")

	}()

	return true
}

func (ip *ImageProcessor) Stop(sv *ScrollViewer) bool {

	if len(ip.workSlice) > 0 {
		log.Println("ImageProcessor.Stop terminating active running driver...")
		ip.doCancelation = true

		ip.donewait.Wait()

		ip.doCancelation = false
		log.Println("ImageProcessor.Stop active running driver terminated")
	}
	return ip.Active
}

func (ip *ImageProcessor) Init(sv *ScrollViewer) {
	gorCount := runtime.NumCPU()

	if ip.imageWorkChan == nil {
		ip.imageWorkChan = make([]chan workinfo, gorCount)

		for i := range ip.imageWorkChan {
			//ip.imageWorkChan[i] = make(chan string, gorCount) // unbuffered
			ip.imageWorkChan[i] = make(chan workinfo) // buffered
		}
		//--------------------------
		//run the worker goroutines
		//--------------------------
		for j := 0; j < gorCount; j++ {
			//go ip.doRenderTasker(sv, j+1, ip.imageWorkChan[j])    //with individual channel
			go ip.doRenderTasker(sv, j+1, ip.imageWorkChan[0]) //with just one shared channel
		}
	}
}
func (ip *ImageProcessor) Run(sv *ScrollViewer, jobList []*FileInfo, dirPaths []string) bool {

	if ip == nil {
		return false
	}
	if ip.Active {
		return false
	}

	//runtime.GC()
	sv.handleChangedItems = false
	ip.donewait.Add(1)

	//determine the num of worker goroutines
	//gorCount := runtime.NumCPU()

	if ip.imageWorkChan == nil {
		ip.Init(sv)
	}

	numJob := len(jobList)
	ip.workSlice = make([]workinfo, numJob)

	//add work items to workSlice
	for i, v := range jobList {
		if v.URL == "" {
			ip.workSlice[i].name = filepath.Join(dirPaths[0], v.Name)
		} else {
			ip.workSlice[i].name = filepath.Join(v.URL, v.Name)
		}
		ip.workSlice[i].item = v
	}
	//----------------------------------------------------------
	// run the driver goroutine, writing items to imageWorkChan
	//----------------------------------------------------------
	go func(itms []workinfo) {
		ip.Active = true
		t := time.Now()

		//Load data from cache
		n := sv.CacheDBEnum(dirPaths)

		log.Println("CacheDBEnum found", n, "in", dirPaths)

		if ip.statuswidget != nil {
			ip.workStatus = NewProgresDrawer(ip.statuswidget.AsWidgetBase(), 240, numJob)
		}

		//setup waiters
		numTask := len(itms)
		numWait := numTask

		// Second Phase run, to process image data
		// run the distributor
		ip.workCounter = 0
		ip.workerWaiter.Add(numTask)
		c := 0
		for i := 0; i < numTask; i++ {
			if !ip.doCancelation {
				nextItem := itms[i]
				//send data through worker channel

				ip.imageWorkChan[c] <- workinfo{name: nextItem.name}

				if ip.statusfunc != nil {
					ip.statusfunc(i)
				}

				if i%4 == 0 && ip.workStatus != nil {
					ip.workStatus.DrawProgress(i)
				}
			} else {
				log.Println("ImageProcessor.Run, exit loop...at", i)
				break
			}
			numWait--
		}

		if !ip.doCancelation {
			//wait for all workers to finish
			ip.workerWaiter.Wait()

			if ip.infofunc != nil {
				d := time.Since(t).Seconds()
				ip.infofunc(numJob, d)
			}
		} else {
			log.Println("ImageProcessor.Run, decrementing ip.workerWaiter by ", numWait)
			log.Println("ImageProcessor.Run, waiting for workers task completion...")
			// very important thing to do
			// otherwise the ip.workerWaiter.Wait() call
			// will wait on a wrong value, forever.
			//
			ip.workerWaiter.Add(-numWait)
			ip.workerWaiter.Wait()

			log.Println("ImageProcessor.Run, all workers task completed")
		}

		if ip.workStatus != nil {
			ip.workStatus.Clear()
		}

		//update db for items in this path only
		cntupdated, cntfailed, _ := sv.CacheDBUpdateMapItems(sv.ItemsMap, dirPaths)

		if !ip.doCancelation {
			sv.canvasView.Synchronize(func() {
				sv.canvasView.Invalidate()
			})
			wc := atomic.LoadUint64(&ip.workCounter)
			log.Println("Total items processed: ", wc)
			log.Println("Cache items updated: ", cntupdated, "failed: ", cntfailed)
		}

		sv.handleChangedItems = true
		ip.donewait.Done()
		ip.workSlice = []workinfo{}
		ip.Active = false

	}(ip.workSlice)

	return true
}
func (ip *ImageProcessor) doRenderTasker(sv *ScrollViewer, id int, fnames chan workinfo) bool {

	for v := range fnames {
		if v.name != "" {
			if v.item != nil {
				sz, _ := GetImageInfo(v.name)
				v.item.Width = sz.Width
				v.item.Height = sz.Height
			} else {
				if processImageData(sv, v.name, true, nil) != nil {
					atomic.AddUint64(&ip.workCounter, 1)
				}
			}

			//decrement the wait counter
			if !v.skipdone {
				ip.workerWaiter.Done()
			}
		} else {
			log.Println("doRenderTasker exiting..this-should-not-have-happened.")
			break
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

						im.imageprocessor.imageWorkChan[0] <- workinfo{name: key}

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
				nUpdated, nFailed, _ := sv.CacheDBUpdateMapItems(im.doneMap, []string{""})

				im.itmMutex.Lock()
				im.removeChangedItems(im.changeMap)
				im.itmMutex.Unlock()

				if repaint && ires+1 > 0 {
					if im.infofunc != nil {
						im.infofunc()
					}
					sv.scrollview.Synchronize(func() {
						sv.canvasView.Invalidate()
					})
				}

				log.Println("processChangedItem/processImageData: ", ires+1)
				log.Println("processChangedItem/CacheDBUpdateMapItems, updated: ", nUpdated, "failed:", nFailed)
				jobStatus.Clear()
			}

			im.activated = false
		}(worklist)
	}
}
