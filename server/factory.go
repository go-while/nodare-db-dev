package server

import (
	"github.com/go-while/nodare-db-dev/logger"
	"log"
	"os"
	"strconv"
	"sync"
)

type Factory struct {
	mux sync.Mutex
}

func NewFactory() *Factory {
	return &Factory{}
}

func (f *Factory) NewNDBServer(conf string, ndbServer WebMux, logs ilog.ILOG) (srv Server, vcfg VConfig, sub_dicks uint32) {
	f.mux.Lock()
	defer f.mux.Unlock()


	if f.getEnvTLSEnabled() {
		srv, vcfg, sub_dicks = NewHttpsServer(conf, ndbServer, logs)
		log.Printf("Factory TLS srv='%#v'", srv)
		//_ = NewSocketHandler(srv)
		//sockets.Start()
		return
	}


	srv, vcfg, sub_dicks = NewHttpServer(conf, ndbServer, logs)
	lvlstr := vcfg.GetString(VK_LOG_LOGLEVEL)
	lvlint := ilog.GetLOGLEVEL(lvlstr)
	logs.SetLOGLEVEL(lvlint)
	log.Printf("Factory TCP srv='%#v' vcfg='%#v' sub_dicks=%d lvlstr='%s'=%d loglvl=%d", srv, vcfg, sub_dicks, lvlstr, lvlint, logs.GetLOGLEVEL())
	return
}


func (f *Factory) getEnvTLSEnabled() bool {
	isTLSEnabled, _ := strconv.ParseBool(os.Getenv("NDB_TLS_ENABLED"))
	if isTLSEnabled {
		return true
	}
	return false
}
