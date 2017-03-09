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

	"github.com/lxn/walk"
	_ "github.com/mattn/go-sqlite3"
)

const sqlCreateTable = `CREATE TABLE IF NOT EXISTS usercache (
    uid INTEGER PRIMARY KEY AUTOINCREMENT,
	idpathcrc INTEGER,
	iditemcrc INTEGER,
	itemwidth INTEGER,
	itemheight INTEGER,
    itemdata BLOB,
	UNIQUE(idpathcrc, iditemcrc)
	);
	`

var CacheDB *sql.DB

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
	_, err = CacheDB.Exec(sqlCreateTable)
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

func (sv *ScrollViewer) CacheDBEnum(fpath string) int {
	if !sv.doCache {
		return 0
	}

	idpath := fmt.Sprint(crc32FromName(fpath))
	sSql := "select idpathcrc, iditemcrc, itemwidth,itemheight,itemdata from usercache where idpathcrc = " + idpath

	rows, err := CacheDB.Query(sSql)
	if err != nil {
		sv.doCache = false
		return 0
	}
	defer rows.Close()

	i := 0
	for rows.Next() {
		var id1, id2, imgw, imgh int
		var imgdata []byte

		err = rows.Scan(&id1, &id2, &imgw, &imgh, &imgdata)
		if err != nil {
			log.Fatal(err)
		}
		for k, v := range sv.ItemsMap {
			if crc32FromName(k) == uint32(id2) {
				v.Imagedata = imgdata
				v.thumbW = imgw
				v.thumbH = imgh

				//log.Println("CacheDBEnum", v.thumbW, v.thumbH)
				//				f, _ := os.Create("./bkp/0000" + v.Name + ".jpg")
				//				f.Write(imgdata)
				//				f.Close()

				i += 1
				break
			}
		}
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
	return i
}

func (sv *ScrollViewer) CacheDBUpdateMapItems(itmap ItmMap, fpath string) (int64, bool) {
	if !sv.doCache || len(itmap) == 0 {
		log.Println("CacheDBUpdateMapItems, exit !sv.doCache || len(itmap) == 0")
		return 0, false
	}

	defer func() {
		if err := recover(); err != nil { //catch
			log.Println("recover")
			//os.Exit(1)
		}
	}()

	tx, err := CacheDB.Begin()
	if err != nil {
		log.Fatal(err)
	}

	sSql := "INSERT OR REPLACE into usercache(idpathcrc, iditemcrc, itemwidth, itemheight, itemdata) values(?, ?, ?, ?, ?);"

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

		if fpath == "" {
			bDoit = true
		} else {
			bDoit = (filepath.Dir(k) == fpath)
		}
		if bDoit {
			buf := v.Imagedata

			//			f, _ := os.Create("./bkp/1111" + v.Name + ".jpeg")
			//			f.Write(buf)
			//			f.Close()

			res, err = stmt.Exec(crc32FromName(filepath.Dir(k)), crc32FromName(k), v.thumbW, v.thumbH, buf)
			if err != nil {
				log.Fatal(err)
			}
			v.Changed = false
			v.dbsynched = true

			i, _ := res.RowsAffected()
			rcount += i
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
	return rcount, true
}

func (sv *ScrollViewer) CacheDBUpdateItem(mkey string) error {
	if !sv.doCache {
		return nil
	}

	tx, err := CacheDB.Begin()
	if err != nil {
		return err
	}

	//sSql := "insert OR IGNORE into usercache(idpathcrc, iditemcrc, itemdata) values(?, ?, ?);"
	//sSql := "INSERT OR REPLACE into usercache(idpathcrc, iditemcrc, itemdata) values(?, ?, ?);"
	sSql := "INSERT OR REPLACE into usercache(idpathcrc, iditemcrc, itemwidth, itemheight, itemdata) values(?, ?, ?, ?, ?);"

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

	//log.Println("db write attempt for ", i, fpath)
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

	//sSql := "INSERT OR REPLACE into usercache(idpathcrc, iditemcrc, itemdata) values(?, ?, ?);"
	sSql := "INSERT OR REPLACE into usercache(idpathcrc, iditemcrc, itemwidth, itemheight, itemdata) values(?, ?, ?, ?, ?);"

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

	//log.Println("db write attempt for ", i, fpath)
	return err
}
