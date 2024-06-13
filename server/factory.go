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

func (f *Factory) GetWebServer(ndbServer WebMux, logger ilog.ILOG) (srv Server, vcfg VConfig, sub_dicks uint32) {
	f.mux.Lock()
	defer f.mux.Unlock()
	if f.getTLSEnabled() {
		srv, vcfg, sub_dicks = NewHttpsServer(ndbServer, logger)
		log.Printf("Factory TLS srv='%#v'", srv)
		//_ = NewSocketHandler(srv)
		//sockets.Start()
		return
	}
	srv, vcfg, sub_dicks = NewHttpServer(ndbServer, logger)
	lvlstr := vcfg.GetString("log.log_level")
	lvlint := ilog.GetLOGLEVEL(lvlstr)
	logger.SetLOGLEVEL(lvlint)
	log.Printf("Factory TCP srv='%#v' vcfg='%#v' sub_dicks=%d lvlstr='%s'=%d loglvl=%d", srv, vcfg, sub_dicks, lvlstr, lvlint, logger.GetLOGLEVEL())
	return
}

func (f *Factory) getTLSEnabled() bool {
	isTLSEnabled, err := strconv.ParseBool(os.Getenv("NDB_TLS_ENABLED"))
	if err != nil {
		isTLSEnabled = false
	}
	log.Printf("isTLSEnabled=%t", isTLSEnabled)
	return isTLSEnabled
}
