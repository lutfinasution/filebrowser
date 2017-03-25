// fb_cache
package main

import (
	//"bytes"
	"database/sql"
	"fmt"
	"hash/crc32"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lxn/walk"
	_ "github.com/mattn/go-sqlite3"
)

const sqlCreateTableCache = `CREATE TABLE IF NOT EXISTS usercache (
    uid INTEGER PRIMARY KEY AUTOINCREMENT,
	idpathcrc INTEGER,
	iditemcrc INTEGER,
	itemwidth INTEGER,
	itemheight INTEGER,
    itemdata BLOB,
	UNIQUE(idpathcrc, iditemcrc)
	);
	`
const sqlCreateTableAlbum = `CREATE TABLE IF NOT EXISTS useralbum (
    idalbum INTEGER PRIMARY KEY AUTOINCREMENT,
	albumname TEXT,
	albumdesc TEXT,
    albumdate DATETIME,
    albumsize INTEGER,
	albumcover BLOB,
	UNIQUE(albumname, albumdesc)
	);
	`
const sqlCreateTableAlbumItems = `CREATE TABLE IF NOT EXISTS useralbumitems (
    iditem INTEGER PRIMARY KEY AUTOINCREMENT,
	idalbum INTEGER,
	itemname TEXT,
	itempath TEXT,
    itemsize INTEGER,
    itemdate DATETIME,
	itemw INTEGER,
	itemh INTEGER,
	itemdata BLOB,
	UNIQUE(idalbum, itemname,itempath)
	);
	`

var CacheDB, AlbumDB *sql.DB

func crc32FromName(name string) uint32 {
	return crc32.ChecksumIEEE([]byte(name))
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func (sv *ScrollViewer) OpenCacheDB(fdbname string) bool {
	var err error

	if CacheDB == nil {
		spath, _ := walk.AppDataPath() //C:\Users\streaming\AppData\Roaming
		fdbname = filepath.Join(spath, "lutfinas", "GoImageBrowser", "cache", "cache.db")

		if _, err = os.Stat(fdbname); err != nil {
			err = os.MkdirAll(filepath.Dir(fdbname), 0644)
			if err != nil {
				log.Println("db path create error", filepath.Dir(fdbname))
				return false
			}
		}

		CacheDB, err = sql.Open("sqlite3", fdbname)
		checkErr(err)
	}

	//Create main table if not exists
	_, err = CacheDB.Exec(sqlCreateTableCache)
	checkErr(err)

	log.Println("db opened", fdbname)
	return true
}

func (sv *ScrollViewer) CloseCacheDB() bool {
	if CacheDB != nil {
		CacheDB.Close()
		CacheDB = nil
	}
	log.Println("db closed")
	return true
}

func (sv *ScrollViewer) CacheDBEnum(fpaths []string) int {
	if !sv.doCache {
		return 0
	}
	var sSql, idpath string
	var tMatch float64

	t := time.Now()

	if len(fpaths) > 1 {
		for i := 0; i < len(fpaths); i++ {
			fpaths[i] = fmt.Sprint(crc32FromName(fpaths[i]))
		}
		idpath = "(" + strings.Join(fpaths, ",") + ")"

		sSql = `select idpathcrc, iditemcrc, itemwidth,itemheight,itemdata 
			 	from usercache where idpathcrc in ` + idpath
	} else {
		idpath = fmt.Sprint(crc32FromName(fpaths[0]))

		sSql = `select idpathcrc, iditemcrc, itemwidth,itemheight,itemdata
				from usercache where idpathcrc = ` + idpath
	}

	rows, err := CacheDB.Query(sSql)
	if err != nil {
		sv.doCache = false
		return 0
	}
	defer rows.Close()

	tMatch = 0

	// new n improved method using temp keyed map
	// to store map items for fast record matching.
	// the map key is set to the to be matched type.
	matchMap := make(map[uint32]*FileInfo)
	for k, v := range sv.ItemsMap {
		matchMap[crc32FromName(k)] = v
	}

	i := 0
	for rows.Next() {
		var id1, id2, imgw, imgh int
		var imgdata []byte

		err = rows.Scan(&id1, &id2, &imgw, &imgh, &imgdata)
		if err != nil {
			log.Fatal(err)
		}

		t1 := time.Now()
		//		for k, v := range sv.ItemsMap {
		//			if crc32FromName(k) == uint32(id2) {
		//				v.Imagedata = imgdata
		//				v.thumbW = imgw
		//				v.thumbH = imgh

		//				i += 1
		//				break
		//			}
		//		}

		// new and improved db rec mapping to mapitems
		// on 5000+ db rec can save 2+ seconds.
		if v, ok := matchMap[uint32(id2)]; ok {
			v.Imagedata = imgdata
			v.thumbW = imgw
			v.thumbH = imgh
			i += 1
		}

		tMatch += time.Since(t1).Seconds()
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("CacheDBEnum, elapsed", time.Since(t).Seconds(), "TotalMatchTime:", tMatch)
	return i
}
func (sv *ScrollViewer) CacheDBUpdateMapItems(itmap ItmMap, fpaths []string) (resUpdated int64, reFailed int64, result bool) {

	if !sv.doCache || len(itmap) == 0 {
		log.Println("CacheDBUpdateMapItems, exit !sv.doCache || len(itmap) == 0")
		return 0, 0, false
	}

	defer func() {
		if err := recover(); err != nil { //catch
			log.Println("recover")
		}
	}()

	tx, err := CacheDB.Begin()
	if err != nil {
		log.Fatal(err)
	}

	sSql := `INSERT OR REPLACE into usercache(idpathcrc, iditemcrc, itemwidth, itemheight, itemdata) 
			 values(?, ?, ?, ?, ?);`

	stmt, err := tx.Prepare(sSql)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	var res sql.Result
	var bDoit bool

	rcount := int64(0)
	for k, v := range itmap {
		if v.dbsynched {
			continue
		}

		// determine if item should be included in the update
		if fpaths[0] == "" {
			bDoit = true
		} else {
			//bDoit = (filepath.Dir(k) == fpaths)
			bDoit = false
			for _, vv := range fpaths {
				if filepath.Dir(k) == vv {
					bDoit = true
					break
				}
			}
		}
		if bDoit {
			buf := v.Imagedata
			if len(buf) == 0 {
				reFailed += 1
				log.Println("CacheDBUpdateMapItems, skip, item has no data", rcount, v.Name)
				continue
			}
			res, err = stmt.Exec(crc32FromName(filepath.Dir(k)), crc32FromName(k), v.thumbW, v.thumbH, buf)
			if err != nil {
				log.Fatal(err)
			}
			v.Changed = false
			v.dbsynched = true

			i, _ := res.RowsAffected()
			resUpdated += i
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
	return resUpdated, reFailed, reFailed == 0
}

func (sv *ScrollViewer) CacheDBUpdateItem(mkey string) error {
	if !sv.doCache {
		return nil
	}

	tx, err := CacheDB.Begin()
	if err != nil {
		return err
	}

	sSql := `INSERT OR REPLACE into usercache(idpathcrc, iditemcrc, itemwidth, itemheight, itemdata) 
			 values(?, ?, ?, ?, ?);`

	stmt, err := tx.Prepare(sSql)
	if err != nil {
		return err
	}
	defer stmt.Close()

	var res sql.Result

	if v, ok := sv.ItemsMap[mkey]; ok {
		buf := v.Imagedata

		res, err = stmt.Exec(crc32FromName(filepath.Dir(mkey)), crc32FromName(mkey), v.thumbW, v.thumbH, buf)
		if err != nil {
			return err
		}
		rcnt, _ := res.RowsAffected()
		if rcnt != 0 {
			v.Changed = false
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return err
}

func (sv *ScrollViewer) CacheDBUpdateItemFromBuffer(mkey string, buf []byte, w int, h int) error {
	if !sv.doCache || buf == nil {
		return nil
	}

	tx, err := CacheDB.Begin()
	if err != nil {
		return err
	}

	sSql := `INSERT OR REPLACE into usercache(idpathcrc, iditemcrc, itemwidth, itemheight, itemdata) 
		     values(?, ?, ?, ?, ?);`

	stmt, err := tx.Prepare(sSql)
	if err != nil {
		return err
	}
	defer stmt.Close()

	var res sql.Result

	res, err = stmt.Exec(crc32FromName(filepath.Dir(mkey)), crc32FromName(mkey), w, h, buf)
	if err != nil {
		return err
	}
	rcnt, _ := res.RowsAffected()
	if rcnt != 0 {

	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return err
}

//------------------------------------------------
// ALBUM DB
//------------------------------------------------
func OpenAlbumDB(fdbname string) bool {
	var err error

	if AlbumDB == nil {
		spath, _ := walk.AppDataPath() //C:\Users\streaming\AppData\Roaming
		fdbname = filepath.Join(spath, "lutfinas", "GoImageBrowser", "cache", "album.db")

		if _, err = os.Stat(fdbname); err != nil {
			err = os.MkdirAll(filepath.Dir(fdbname), 0644)
			if err != nil {
				log.Println("album db path create error", filepath.Dir(fdbname))
				return false
			}
		}

		AlbumDB, err = sql.Open("sqlite3", fdbname)
		checkErr(err)
	}

	//Create album table if not exists
	_, err = AlbumDB.Exec(sqlCreateTableAlbum)
	checkErr(err)

	//Create items table if not exists
	_, err = AlbumDB.Exec(sqlCreateTableAlbumItems)
	checkErr(err)

	log.Println("album db opened", fdbname)
	return true
}

func (sv *ScrollViewer) CloseAlbumDB() bool {
	if AlbumDB != nil {
		AlbumDB.Close()
		AlbumDB = nil
	}
	log.Println("album db closed")
	return true
}

func (sv *ScrollViewer) AlbumDBEnum(filter string) int {

	sSql := `select a.idalbum, a.albumname,a.albumdesc,a.albumdate, 
			 count(ai.iditem) items, ifnull(a.albumcover,min(ai.itemdata)) image 
			 from useralbum a left join useralbumitems ai 
			 on a.idalbum=ai.idalbum 
			 group by a.idalbum;`

	rows, err := AlbumDB.Query(sSql)
	if err != nil {
		log.Println(err.Error())
		return 0
	}
	defer rows.Close()

	i := 0
	for rows.Next() {
		var id int
		var data1, data2 string
		var size int64
		var date time.Time
		var imgdata []byte

		err = rows.Scan(&id, &data1, &data2, &date, &size, &imgdata)
		if err != nil {
			log.Fatal(err)
		}

		// Create or mod map item
		sv.itemsModel.items = append(sv.itemsModel.items,
			&FileInfo{index: id,
				Name:      data1,
				URL:       data2,
				Modified:  date,
				Size:      size,
				Imagedata: imgdata,
			})

		i += 1
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
	return i
}
func (sv *ScrollViewer) AlbumDBEnumByNameItems(albumname string) (res []*FileInfo) {

	sSql := `select idalbum from useralbum where albumname='` + albumname + "'"

	rows, err := AlbumDB.Query(sSql)
	if err != nil {
		log.Println(err.Error())
		return res
	}
	defer rows.Close()

	var id int
	for rows.Next() {
		err = rows.Scan(&id)
	}

	return sv.AlbumDBEnumItems(id)
}

func (sv *ScrollViewer) AlbumDBEnumItems(idAlbum int) (res []*FileInfo) {

	sSql := `select iditem,itemname,itempath,itemw,itemh,itemdata 
			 from useralbumitems
			 where idalbum=` + strconv.Itoa(idAlbum)

	rows, err := AlbumDB.Query(sSql)
	if err != nil {
		log.Println(err.Error())
		return res
	}
	defer rows.Close()

	i := 0
	for rows.Next() {
		var id, w, h int
		var data1, data2 string
		var size int64
		var date time.Time
		var imgdata []byte

		//err = rows.Scan(&id, &data1, &data2, &size, &date, &w, &h, &imgdata)
		err = rows.Scan(&id, &data1, &data2, &w, &h, &imgdata)
		if err != nil {
			log.Fatal(err)
		}

		// Create or mod map item
		res = append(res,
			&FileInfo{index: id,
				Name:      data1,
				URL:       data2,
				Modified:  date,
				Size:      size,
				Width:     w,
				Height:    h,
				Imagedata: imgdata,
			})

		i += 1
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
	return res
}
func (sv *ScrollViewer) AlbumDBGetItem(idItem string) (res *FileInfo) {

	sSql := `select iditem,itemname,itempath,itemw,itemh,itemdata 
			 from useralbumitems
			 where itemname='` + idItem + "'"

	rows, err := AlbumDB.Query(sSql)
	if err != nil {
		log.Println(err.Error())
		return res
	}
	defer rows.Close()

	i := 0
	for rows.Next() {
		var id, w, h int
		var data1, data2 string
		var size int64
		var date time.Time
		var imgdata []byte

		err = rows.Scan(&id, &data1, &data2, &w, &h, &imgdata)
		if err != nil {
			log.Fatal(err)
		}

		// Create or mod map item
		res = &FileInfo{index: id,
			Name:      data1,
			URL:       data2,
			Modified:  date,
			Size:      size,
			Width:     w,
			Height:    h,
			Imagedata: imgdata,
		}

		i += 1
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
	return res
}

// Insert or update an album in useralbum.
func (sv *ScrollViewer) AlbumDBUpdateAlbum(item *FileInfo) (rcnt int64, err error) {
	if AlbumDB == nil {
		OpenAlbumDB("")
	}
	tx, err := AlbumDB.Begin()
	if err != nil {
		return 0, err
	}

	sSql := `INSERT OR REPLACE into useralbum(albumname, albumdesc, albumdate, albumsize, albumcover) 
			 values(?, ?, ?, ?, ?);`

	if item.index != -1 {
		sSql = `INSERT OR REPLACE into useralbum(idalbum, albumname, albumdesc, albumdate, albumsize, albumcover) 
				values(?, ?, ?, ?, ?, ?);`
	}

	stmt, err := tx.Prepare(sSql)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	var res sql.Result

	buf := item.Imagedata
	date := time.Now()

	if item.index == -1 {
		res, err = stmt.Exec(item.Name, item.URL, date, item.Size, buf)
	} else {
		res, err = stmt.Exec(item.index, item.Name, item.URL, date, item.Size, buf)
	}
	if err != nil {
		log.Println("AlbumDBUpdateItem", err.Error())
		return 0, err
	}
	rcnt, _ = res.RowsAffected()

	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	log.Println("album db upsert: ", rcnt)
	return rcnt, err
}

// Insert or update an album items in useralbumitems.
func (sv *ScrollViewer) AlbumDBUpdateItems(idAlbum int, items []*FileInfo) (rcnt int64, err error) {
	if AlbumDB == nil {
		OpenAlbumDB("")
	}
	tx, err := AlbumDB.Begin()
	if err != nil {
		return 0, err
	}

	sSql := `INSERT OR REPLACE into useralbumitems(idalbum, itemname, itempath, itemw, itemh, itemdata) 
	         values(?, ?, ?, ?, ?, ?);`

	stmt, err := tx.Prepare(sSql)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	var res sql.Result
	var ires int64
	for _, v := range items {
		res, err = stmt.Exec(idAlbum, v.Name, v.URL, v.Width, v.Height, v.Imagedata)
		if err != nil {
			return 0, err
		}
		rcnt, _ = res.RowsAffected()
		ires += rcnt
	}
	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	log.Println("album items db upsert: ", rcnt)
	return ires, err
}

func (sv *ScrollViewer) AlbumDBDeleteItems(items []*FileInfo) (rcnt int64, err error) {
	if AlbumDB == nil {
		OpenAlbumDB("")
	}
	tx, err := AlbumDB.Begin()
	if err != nil {
		return 0, err
	}

	sSql := `delete from useralbumitems where iditem = ?;`

	stmt, err := tx.Prepare(sSql)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	var res sql.Result
	var ires int64

	for _, v := range items {
		res, err = stmt.Exec(v.index)
		if err != nil {
			return 0, err
		}
		rcnt, _ = res.RowsAffected()

		if rcnt > 0 {
			//change this to indicate deleted item
			//to be processed in the source object
			v.index = -1
		}
		ires += rcnt
	}
	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	//log.Println("album items db delete: ", rcnt)
	return ires, err
}
