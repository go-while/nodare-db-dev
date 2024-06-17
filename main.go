package main

import (
	"flag"
	"github.com/go-while/go-cpu-mem-profiler"
	"github.com/go-while/nodare-db-dev/database"
	"github.com/go-while/nodare-db-dev/logger"
	"github.com/go-while/nodare-db-dev/server"
	"os"
	"os/signal"
	"sync"
	"syscall"
)


var (
	wg              sync.WaitGroup
	stop_chan = make(chan struct{}, 1)
	Prof            *prof.Profiler
	flag_configfile string
	flag_logfile    string
	flag_pprof      string
) // end var

func main() {
	// capture the flags: overwrites config file settings!
	flag.StringVar(&flag_configfile, "config", server.DEFAULT_CONFIG_FILE, "path to config file")
	flag.StringVar(&flag_logfile, "logfile", "", "path to ndb.log")
	flag.StringVar(&flag_pprof, "pprof", "", "PPROF WEB: [ (addr):port ]\n     LOCAL '127.0.0.1:1234' OR '[::1]:1234'\n     PUBLIC/WORLD ':1234' OR 'IP4:PORT' OR '[IP6]:PORT'")
	flag.Parse()

	// loading logger prints first line LOGLEVEL="XX" to console but will never showup in logfile!
	logs := ilog.NewLogger(ilog.GetEnvLOGLEVEL(), flag_logfile)
	cfg, sub_dicks := server.NewViperConf(flag_configfile, logs)

	db := database.NewDICK(logs, sub_dicks)
	srv := server.NewFactory().NewNDBServer(cfg, server.NewXNDBServer(db, logs), logs, stop_chan, wg, db)
	if flag_pprof != "" {
		Prof = prof.NewProf()
		server.Prof = Prof
		logs.Debug("Launching PprofWeb @ %s", flag_pprof)
		go Prof.PprofWeb(flag_pprof)
	}
	if logs.IfDebug() {
		logs.Debug("Mode 1: Loaded vcfg='%#v' host='%v'", cfg, cfg.GetString(server.VK_SERVER_HOST))
		logs.Debug("Mode 1: Booted DB sub_dicks=%d srv='%v'", sub_dicks, srv)
	}
	go srv.Start()

	// wait for os signal to exit and initiates shutdown procedure
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	stop_chan <- struct{}{} // force waiters to stop
	wg.Wait()
	logs.Info("Exit: %s", os.Args[0])
} // end func main
