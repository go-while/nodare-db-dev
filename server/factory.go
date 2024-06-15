package server

import (
	"github.com/go-while/nodare-db-dev/logger"
	"sync"
	"time"
)

type Factory struct {
	mux sync.Mutex
}

func NewFactory() *Factory {
	return &Factory{}
}

func (f *Factory) NewNDBServer(cfg VConfig, ndbServer WebMux, logs ilog.ILOG, stop_chan chan struct{}, wg sync.WaitGroup) (srv Server) {
	f.mux.Lock()
	defer f.mux.Unlock()

	tls_enabled := cfg.GetBool(VK_SEC_TLS_ENABLED)
	logfile := cfg.GetString(VK_LOG_LOGFILE)
	logs.LogStart(logfile)
	logs.Info("factory: viper cfg loaded tls_enabled=%t logfile='%s'", tls_enabled, logfile)

	NewSocketHandler(cfg, logs, stop_chan, wg)
	time.Sleep(time.Second / 10)

	switch tls_enabled {
	case false:
		// TCP WEB SERVER
		srv = NewHttpServer(cfg, ndbServer, logs, stop_chan, wg)
		logs.Debug("Factory TCP WEB\n srv='%#v'\n^EOL\n\n cfg='%#v'\n^EOL loglevel=%d\n\n", srv, cfg, logs.GetLOGLEVEL())
	case true:
		// TLS WEB SERVER
		srv = NewHttpsServer(cfg, ndbServer, logs, stop_chan, wg)
		logs.Debug("Factory TLS WEB\n  srv='%#v'\n^EOL\n\n cfg='%#v'\n^EOL loglevel=%d\n\n", cfg, srv, logs.GetLOGLEVEL())
	}

	return
} // end func NewNDBServer
