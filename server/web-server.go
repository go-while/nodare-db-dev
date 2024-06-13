package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-while/nodare-db-dev/logger"
	//"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Server interface {
	Start()
	Stop()
}

type HttpServer struct {
	ndbServer  WebMux
	httpServer *http.Server
	VCFG       VConfig
	sigChan    chan os.Signal
	logs       ilog.ILOG
}

type HttpsServer struct {
	ndbServer   WebMux
	httpsServer *http.Server
	VCFG        VConfig
	sigChan     chan os.Signal
	logs        ilog.ILOG
}

func NewHttpServer(conf string, ndbServer WebMux, logs ilog.ILOG) (srv *HttpServer, cfg VConfig, sub_dicks uint32) {
	cfg, sub_dicks = NewViperConf(conf, logs)
	//log.Printf("NewHttpServer cfg='%#v' ViperConfig='%#v'", cfg, cfg.ViperConfig.)
	srv = &HttpServer{
		sigChan:   make(chan os.Signal, 1),
		ndbServer: ndbServer,
		logs:    logs,
		VCFG:      cfg,
	}
	return
}

func NewHttpsServer(conf string, ndbServer WebMux, logs ilog.ILOG) (srv *HttpsServer, cfg VConfig, sub_dicks uint32) {
	cfg, sub_dicks = NewViperConf(conf, logs)
	srv = &HttpsServer{
		sigChan:   make(chan os.Signal, 1),
		ndbServer: ndbServer,
		logs:    logs,
		VCFG:      cfg,
	}
	return
}

func (server *HttpServer) Start() {

	if server.VCFG.IsSet(VK_LOG_LOGFILE) {
		server.logs.LogStart(server.VCFG.GetString(VK_LOG_LOGFILE))
	}

	server.httpServer = &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		Addr:         fmt.Sprintf("%s:%s", server.VCFG.GetString(VK_SERVER_HOST), server.VCFG.GetString(VK_SERVER_PORT_TCP)),
		Handler:      server.ndbServer.CreateMux(),
	}

	go func() {
		server.logs.Info("HttpServer @ '%s:%s'", server.VCFG.GetString(VK_SERVER_HOST), server.VCFG.GetString(VK_SERVER_PORT_TCP))
		if err := server.httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			server.logs.Fatal("HTTP server error: %v", err)
		}
		server.logs.Info("HttpServer: closing")
		server.logs.LogClose()
	}()

	signal.Notify(server.sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-server.sigChan
}

func (server *HttpServer) Stop() {
	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	if err := server.httpServer.Shutdown(shutdownCtx); err != nil {
		server.logs.Fatal("HttpServer: shutdown error %v", err)
	}

	server.logs.Info("HttpServer shutdown complete")
	server.httpServer = nil

	server.logs.LogClose()
}

func (server *HttpsServer) Start() {
	server.httpsServer = &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		Addr:         fmt.Sprintf("%s:%s", server.VCFG.GetString(VK_SERVER_HOST), server.VCFG.GetString(VK_SERVER_PORT_TCP)),
		Handler:      server.ndbServer.CreateMux(),
	}

	go func() {
		server.logs.Info("HttpsServer @ '%s:%s'", server.VCFG.GetString(VK_SERVER_HOST), server.VCFG.GetString(VK_SERVER_PORT_TCP))
		server.logs.Debug("HttpsServer: PUB_CERT='%s' PRIV_KEY='%s'", server.VCFG.GetString(VK_SEC_TLS_PUBCERT), server.VCFG.GetString(VK_SEC_TLS_PRIVKEY))

		if err := server.httpsServer.ListenAndServeTLS(server.VCFG.GetString(VK_SEC_TLS_PUBCERT), server.VCFG.GetString(VK_SEC_TLS_PRIVKEY)); !errors.Is(err, http.ErrServerClosed) {
			server.logs.Fatal("HttpsServer: error %v", err)
		}
		server.logs.Debug("HttpsServer: closing")
		server.logs.LogClose()
	}()

	signal.Notify(server.sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-server.sigChan
}

func (server *HttpsServer) Stop() {
	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	if err := server.httpsServer.Shutdown(shutdownCtx); err != nil {
		server.logs.Fatal("HttpsServer: shutdown error %v", err)
	}

	server.logs.Info("HttpsServer: shutdown complete")
	server.httpsServer = nil
	server.logs.LogClose()
}
