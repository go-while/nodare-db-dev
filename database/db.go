package database

import (
	"github.com/go-while/nodare-db-dev/logger"
	"sync"
	"time"
)

type XDBS struct {
	mux  sync.RWMutex
	dbs  map[string]*XDatabase
	Logs ilog.ILOG
	HashMode int
	SubDicks int
}

type XDatabase struct {
	XDICK    *XDICK
	BootT    int64
	HashMode int
}

func NewDBS(logs ilog.ILOG, sub_dicks int, hashmode int) *XDBS {
	xdb := &XDBS{
		dbs:  make(map[string]*XDatabase, 16),
		Logs: logs,
		HashMode: hashmode,
		SubDicks: sub_dicks,
	}
	return xdb
} // end func NewDBS


func (x *XDBS) AddDB(ident string, db *XDatabase) bool {
	//x.mux.Lock()
	//defer x.mux.Unlock()
	if x.dbs[ident] != nil {
		x.Logs.Error("AddDB ident not unique!")
		return false
	}
	x.dbs[ident] = db
	return true
} // end func AddDB

func (x *XDBS) GetDB(ident string, new bool) (db *XDatabase) {
	x.mux.RLock()
	if x.dbs[ident] != nil {
		db = x.dbs[ident]
		x.mux.RUnlock()
		//x.Logs.Debug("GetDB ident='%s' => db", ident)
		return
	}
	x.mux.RUnlock()

	if !new {
		x.Logs.Error("GetDB ident='%s' not found!", ident)
		return
	}

	x.mux.Lock()
	if x.dbs[ident] != nil {
		// anyone was faster creating the db
		db = x.dbs[ident]
		x.mux.Unlock()
		return
	}

	x.Logs.Error("GetDB ident='%s' => NewDICK", ident)
	db = NewDICK(x.Logs, x.SubDicks, x.HashMode)
	x.AddDB(ident, db)
	x.mux.Unlock()
	return
} // end func GetDB

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
