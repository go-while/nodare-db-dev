package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-while/nodare-db-dev/logger"
	"net/http"
	"sync"
	"time"
)

type Server interface {
	Start()
	Stop()
} // end Server interface

type HttpServer struct {
	ndbServer  WebMux
	httpServer *http.Server
	cfg        VConfig
	logs       ilog.ILOG
	stop_chan  chan struct{}
	wg         sync.WaitGroup
	acl        *AccessControlList
} // end struct HttpServer

type HttpsServer struct {
	ndbServer   WebMux
	httpsServer *http.Server
	cfg         VConfig
	logs        ilog.ILOG
	stop_chan   chan struct{}
	wg          sync.WaitGroup
	acl         *AccessControlList
} // end struct HttpsServer

func NewHttpServer(cfg VConfig, ndbServer WebMux, logs ilog.ILOG, stop_chan chan struct{}, wg sync.WaitGroup) (srv *HttpServer) {
	srv = &HttpServer{
		ndbServer: ndbServer,
		logs:      logs,
		cfg:       cfg,
		stop_chan: stop_chan,
		wg:        wg,
		acl:       NewACL(),
	}
	return
} // end func NewHttpServer

func NewHttpsServer(cfg VConfig, ndbServer WebMux, logs ilog.ILOG, stop_chan chan struct{}, wg sync.WaitGroup) (srv *HttpsServer) {
	srv = &HttpsServer{
		ndbServer: ndbServer,
		logs:      logs,
		cfg:       cfg,
		stop_chan: stop_chan,
		wg:        wg,
		acl:       NewACL(),
	}
	return
} // end func NewHttpsServer

func (server *HttpServer) Start() {

	server.httpServer = &http.Server{
		ReadTimeout:  time.Duration(RTO) * time.Second,
		WriteTimeout: time.Duration(WTO) * time.Second,
		IdleTimeout:  time.Duration(ITO) * time.Second,
		Addr:         fmt.Sprintf("%s:%s", server.cfg.GetString(VK_SERVER_HOST), server.cfg.GetString(VK_SERVER_PORT_TCP)),
		Handler:      server.ndbServer.CreateMux(server.cfg),
	}
	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		server.logs.Info("HTTP @ '%s:%s'", server.cfg.GetString(VK_SERVER_HOST), server.cfg.GetString(VK_SERVER_PORT_TCP))
		if err := server.httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			server.logs.Fatal("HTTP server error: %v", err)
		}
		server.logs.Info("HttpServer: closing")
	}()

	server.wg.Add(1)
	defer server.wg.Done()
	stopnotify := <-server.stop_chan // waits for signal from main
	server.Stop()
	server.logs.Info("HttpServer: closed")
	server.stop_chan <- stopnotify // push back in to notify others
} // end func httpServer interface Start()

func (server *HttpServer) Stop() {
	server.logs.Info("HttpServer: stopping")
	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()
	if err := server.httpServer.Shutdown(shutdownCtx); err != nil {
		server.logs.Fatal("HttpServer: shutdown error %v", err)
	}
	server.logs.Info("HttpServer: stopped")
} // end func httpServer interface Stop()

func (server *HttpsServer) Start() {
	server.httpsServer = &http.Server{
		ReadTimeout:  time.Duration(RTO) * time.Second,
		WriteTimeout: time.Duration(WTO) * time.Second,
		IdleTimeout:  time.Duration(ITO) * time.Second,
		Addr:         fmt.Sprintf("%s:%s", server.cfg.GetString(VK_SERVER_HOST), server.cfg.GetString(VK_SERVER_PORT_TCP)),
		Handler:      server.ndbServer.CreateMux(server.cfg),
	}
	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		server.logs.Info("HTTPS @ '%s:%s'", server.cfg.GetString(VK_SERVER_HOST), server.cfg.GetString(VK_SERVER_PORT_TCP))
		//server.logs.Debug("HttpsServer: PUB_CERT='%s' PRIV_KEY='%s'", server.cfg.GetString(VK_SEC_TLS_PUBCERT), server.cfg.GetString(VK_SEC_TLS_PRIVKEY))
		if err := server.httpsServer.ListenAndServeTLS(server.cfg.GetString(VK_SEC_TLS_PUBCERT), server.cfg.GetString(VK_SEC_TLS_PRIVKEY)); !errors.Is(err, http.ErrServerClosed) {
			server.logs.Fatal("HttpsServer: error %v", err)
		}
		server.logs.Info("HttpsServer: closing")
	}()

	server.wg.Add(1)
	defer server.wg.Done()
	stopnotify := <-server.stop_chan // waits for signal from main
	server.Stop()
	server.logs.Info("HttpsServer: closed")
	server.stop_chan <- stopnotify // push back in to notify others
} // end func httpsServer interface Start()

func (server *HttpsServer) Stop() {
	server.logs.Info("HttpsServer: stopping")
	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()
	if err := server.httpsServer.Shutdown(shutdownCtx); err != nil {
		server.logs.Fatal("HttpsServer: shutdown error %v", err)
	}
	server.logs.Info("HttpsServer: stopped")
} // end func httpsServer interface Stop()


