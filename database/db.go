package database

import (
	"github.com/go-while/nodare-db-dev/logger"
	"sync"
	"time"
)

type XDBS struct {
	mux  sync.RWMutex
	dbs  map[string]*XDatabase
	logs ilog.ILOG
}

type XDatabase struct {
	XDICK    *XDICK
	BootT    int64
	HashMode int
}

func NewDBS(logs ilog.ILOG) *XDBS {
	xdb := &XDBS{
		dbs:  make(map[string]*XDatabase, 16),
		logs: logs,
	}
	return xdb
} // end func NewDBS

func (x *XDBS) AddDB(ident string, db *XDatabase) bool {
	x.mux.Lock()
	defer x.mux.Unlock()
	if x.dbs[ident] != nil {
		x.logs.Error("AddDB ident not unique!")
		return false
	}
	x.dbs[ident] = db
	return true
} // end func AddDB

func (x *XDBS) GetDB(ident string) (db *XDatabase) {
	x.mux.RLock()
	defer x.mux.RUnlock()
	if x.dbs[ident] != nil {
		x.logs.Error("GetDB ident='%s' not found!", ident)
		return nil
	}
	db = x.dbs[ident]
	return
} // end func AddDB

func NewDICK(logs ilog.ILOG, sub_dicks int, hashmode int) *XDatabase {

	xdick := NewXDICK(logs, sub_dicks, hashmode)
	if SYSMODE == 2 && hashmode == HASH_siphash {
		xdick.GenerateSALT()
	}
	db := &XDatabase{
		XDICK:    xdick,
		BootT:    time.Now().Unix(),
		HashMode: hashmode, // must not change on runtime
	}
	return db
} // end func NewDICK

func (db *XDatabase) Get(key string, val *string) bool {
	return db.XDICK.Get(key, val)
}

func (db *XDatabase) Set(key string, value string, overwrite bool) bool {
	return db.XDICK.Set(key, value, overwrite)
}

func (db *XDatabase) Del(key string) bool {
	return db.XDICK.Del(key)
}
