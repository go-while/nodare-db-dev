package database

import (
	"github.com/go-while/nodare-db-dev/logger"
	"time"
)

type XDatabase struct {
	XDICK *XDICK
	BootT int64
	HashMode int
}

func NewDICK(logs ilog.ILOG, sub_dicks int, hashmode int) *XDatabase {
	xdick := NewXDICK(logs, sub_dicks,hashmode)
	db := &XDatabase{
		XDICK: xdick,
		BootT: time.Now().Unix(),
		HashMode: hashmode, // must not change on runtime
	}
	return db
}

func (db *XDatabase) Get(key string, val *string) bool {
	return db.XDICK.Get(key, val)
}

func (db *XDatabase) Set(key string, value string, overwrite bool) bool {
	return db.XDICK.Set(key, value, overwrite)
}

func (db *XDatabase) Del(key string) bool {
	return db.XDICK.Del(key)
}
