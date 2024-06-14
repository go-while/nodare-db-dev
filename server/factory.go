package server

import (
	"github.com/go-while/nodare-db-dev/logger"
	"os"
	//"log"
	"strconv"
	"sync"
)

type Factory struct {
	mux sync.Mutex
}

func NewFactory() *Factory {
	return &Factory{}
}

func (f *Factory) NewNDBServer(conf string, ndbServer WebMux, logs ilog.ILOG) (srv Server, cfg VConfig, sub_dicks uint32) {
	f.mux.Lock()
	defer f.mux.Unlock()

	cfg, sub_dicks = NewViperConf(conf, logs)

	bootsocket := false
	if bootsocket {
		//_ = NewSocketHandler(srv)
		//sockets.Start()
	}
	tls_enabled := cfg.GetBool(VK_SEC_TLS_ENABLED)
	logfile := cfg.GetString(VK_LOG_LOGFILE)
	logs.LogStart(logfile)
	logs.Info("factory: viper cfg loaded tls_enabled=%t logfile='%s'", tls_enabled, logfile)
	switch tls_enabled {
	case false:
		// TCP WEB SERVER
		srv = NewHttpServer(cfg, ndbServer, logs)
		logs.Debug("Factory TCP WEB\n srv='%#v'\n^EOL\n\n cfg='%#v'\n^EOL sub_dicks=%d loglevel=%d\n\n", srv, cfg, sub_dicks, logs.GetLOGLEVEL())
	case true:
		// TLS WEB SERVER
		srv = NewHttpsServer(cfg, ndbServer, logs)
		logs.Debug("Factory TLS WEB\n  srv='%#v'\n^EOL\n\n cfg='%#v'\n^EOL sub_dicks=%d loglevel=%d\n\n", cfg, srv, sub_dicks, logs.GetLOGLEVEL())
	}
	return
} // end func NewNDBServer

func (f *Factory) getEnvTLSEnabled() bool {
	isTLSEnabled, _ := strconv.ParseBool(os.Getenv("NDB_TLS_ENABLED"))
	if isTLSEnabled {
		return true
	}
	return false
}
