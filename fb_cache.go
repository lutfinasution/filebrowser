// fb_cache
package main

import (
	"database/sql"
	"fmt"
	"hash/crc32"
	"log"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

const sqlCreateTable = `CREATE TABLE IF NOT EXISTS usercache (
    uid INTEGER PRIMARY KEY AUTOINCREMENT,
	idpathcrc INTEGER,
	iditemcrc INTEGER,
    itemdata BLOB,
	UNIQUE(idpathcrc, iditemcrc)
	);
	`

var CacheDB *sql.DB
var doCache = false

func crc32FromName(name string) uint32 {
	return crc32.ChecksumIEEE([]byte(name))
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func CreateCacheDB(fdbname string) bool {
	var err error
	fdbname = "./cache.db"
	CacheDB, err = sql.Open("sqlite3", fdbname)
	checkErr(err)

	//Create main table if not exists
	_, err = CacheDB.Exec(sqlCreateTable)
	checkErr(err)

	log.Println("db opened")
	return true
}

func CloseCacheDB() bool {
	if CacheDB != nil {
		CacheDB.Close()
	}
	log.Println("db closed")
	return true
}

func CacheDBEnum(fpath string) int {
	if !doCache {
		return 0
	}

	idpath := fmt.Sprint(crc32FromName(fpath))
	sSql := "select idpathcrc, iditemcrc, itemdata from usercache where idpathcrc = " + idpath

	rows, err := CacheDB.Query(sSql)
	if err != nil {
		doCache = false
		return 0
	}
	defer rows.Close()

	i := 0
	for rows.Next() {
		var id1, id2 int
		var imgdata []byte

		err = rows.Scan(&id1, &id2, &imgdata)
		if err != nil {
			log.Fatal(err)
		}
		for k, v := range ItemsMap {
			if crc32FromName(k) == uint32(id2) {
				v.Imagedata = imgdata
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

//func CacheDBUpdate(fpath string) (int64, bool) {
//	if !doCache {
//		return 0, false
//	}

//	defer func() {
//		if err := recover(); err != nil { //catch
//			log.Println("recover")
//			//os.Exit(1)
//		}
//	}()

//	tx, err := CacheDB.Begin()
//	if err != nil {
//		log.Fatal(err)
//	}

//	sSql := "INSERT OR REPLACE into usercache(idpathcrc, iditemcrc, itemdata) values(?, ?, ?);"

//	stmt, err := tx.Prepare(sSql)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer stmt.Close()

//	var res sql.Result
//	i := int64(0)
//	for k, v := range ItemsMap {
//		if filepath.Dir(k) == fpath {
//			//if v.Changed {
//			buf := v.Imagedata

//			res, err = stmt.Exec(crc32FromName(filepath.Dir(k)), crc32FromName(k), buf)
//			if err != nil {
//				log.Fatal(err)
//			}
//			v.Changed = false
//			rcnt, _ := res.RowsAffected()
//			i += rcnt
//			//}
//		}
//	}

//	err = tx.Commit()
//	if err != nil {
//		log.Fatal(err)
//	}

//	//log.Println("db write attempt for ", i, fpath)
//	return i, true
//}

func CacheDBUpdateMapItems(itmap ItmMap, fpath string) (int64, bool) {
	if !doCache || len(itmap) == 0 {
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

	sSql := "INSERT OR REPLACE into usercache(idpathcrc, iditemcrc, itemdata) values(?, ?, ?);"

	stmt, err := tx.Prepare(sSql)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	var res sql.Result
	var bDoit bool
	i := int64(0)
	for k, v := range itmap {

		if fpath == "" {
			bDoit = true
		} else {
			bDoit = (filepath.Dir(k) == fpath)
		}
		if bDoit {
			buf := v.Imagedata

			res, err = stmt.Exec(crc32FromName(filepath.Dir(k)), crc32FromName(k), buf)
			if err != nil {
				log.Fatal(err)
			}
			v.Changed = false
			rcnt, _ := res.RowsAffected()
			i += rcnt
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}

	//log.Println("db write attempt for ", i, fpath)
	return i, true
}

func CacheDBUpdateItem(mkey string) error {
	if !doCache {
		return nil
	}

	tx, err := CacheDB.Begin()
	if err != nil {
		return err
	}

	//sSql := "insert OR IGNORE into usercache(idpathcrc, iditemcrc, itemdata) values(?, ?, ?);"
	sSql := "INSERT OR REPLACE into usercache(idpathcrc, iditemcrc, itemdata) values(?, ?, ?);"

	stmt, err := tx.Prepare(sSql)
	if err != nil {
		return err
	}
	defer stmt.Close()

	var res sql.Result

	if v, ok := ItemsMap[mkey]; ok {
		buf := v.Imagedata

		res, err = stmt.Exec(crc32FromName(filepath.Dir(mkey)), crc32FromName(mkey), buf)
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

func CacheDBUpdateItemFromBuffer(mkey string, buf []byte) error {
	if !doCache || buf == nil {
		return nil
	}

	tx, err := CacheDB.Begin()
	if err != nil {
		return err
	}

	sSql := "INSERT OR REPLACE into usercache(idpathcrc, iditemcrc, itemdata) values(?, ?, ?);"

	stmt, err := tx.Prepare(sSql)
	if err != nil {
		return err
	}
	defer stmt.Close()

	var res sql.Result

	res, err = stmt.Exec(crc32FromName(filepath.Dir(mkey)), crc32FromName(mkey), buf)
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
