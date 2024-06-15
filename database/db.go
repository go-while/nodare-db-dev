package database

import (
	"github.com/go-while/nodare-db-dev/logger"
	"time"
)

type XDatabase struct {
	XDICK *XDICK
	BootT int64
}

func NewDICK(logs ilog.ILOG, sub_dicks uint32) *XDatabase {
	xdick := NewXDICK(logs, sub_dicks)
	db := &XDatabase{
		XDICK: xdick,
		BootT: time.Now().Unix(),
	}
	return db
}

func (db *XDatabase) Get(key string) interface{} {
	return db.XDICK.Get(key)
}

func (db *XDatabase) Set(key string, value interface{}) error {
	return db.XDICK.Set(key, value)
}

func (db *XDatabase) Del(key string) error {
	return db.XDICK.Del(key)
}
