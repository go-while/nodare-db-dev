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
	"time"
)


var (
	wg              sync.WaitGroup
	stop_chan = make(chan struct{}, 1)
	Prof            *prof.Profiler
	flag_configfile string
	flag_logfile    string
	flag_pprofweb   string
	flag_profcpu    bool
	flag_hashmode   int
	flag_sysmode    int
	flag_loglevel   string
) // end var

func main() {
	var CPUfile *os.File
	// capture the flags: overwrites config file settings!
	flag.StringVar(&flag_configfile, "config", server.DEFAULT_CONFIG_FILE, "path to config file")
	flag.StringVar(&flag_logfile, "logfile", "", "path to ndb.log")
	flag.StringVar(&flag_pprofweb, "pprofweb", "", "PPROF WEB: [ (addr):port ]\n     LOCAL '127.0.0.1:1234' OR '[::1]:1234'\n     PUBLIC/WORLD ':1234' OR 'IP4:PORT' OR '[IP6]:PORT'")
	flag.BoolVar(&flag_profcpu, "profcpu", false, "boot with CPU profiling")
	flag.IntVar(&flag_hashmode, "hashmode", 4, "SysMode 1) [ 1=PCAS | 2=CRC32 | 3=FNV1A ]\nSysMode 2) [ 1=sipHash | 2=FNV32A | 3=FNV64A | 4=XXHASH | 5=PCAS ]\n")
	flag.IntVar(&flag_sysmode, "sysmode", 2, "[ 1=MAP | 2=SLI ]")
	flag.StringVar(&flag_loglevel, "loglevel", "", "[ INFO | DEBUG ]")
	flag.Parse()

	loglevel := ilog.GetLOGLEVEL(flag_loglevel)
	if loglevel == -1 {
		loglevel = ilog.GetEnvLOGLEVEL()
	}
	// loading logger prints first line LOGLEVEL="XX" to console but will never showup in logfile!
	logs := ilog.NewLogger(loglevel, flag_logfile)

	switch flag_sysmode {
		case database.MAPMODE:
			database.SYSMODE = database.MAPMODE
		case database.SLIMODE:
			database.SYSMODE = database.SLIMODE
		default:
			logs.Fatal("invalid sysmode")
	}

	cfg, sub_dicks := server.NewViperConf(flag_configfile, logs)


	db := database.NewDICK(logs, sub_dicks, flag_hashmode)
	srv := server.NewFactory().NewNDBServer(cfg, server.NewXNDBServer(db, logs), logs, stop_chan, wg, db)
	if flag_pprofweb != "" || flag_profcpu {
		Prof = prof.NewProf()
		server.Prof = Prof
		if flag_pprofweb != "" {
			logs.Debug("Launching PprofWeb @ %s", flag_pprofweb)
			go Prof.PprofWeb(flag_pprofweb)
		}
		if flag_profcpu {
			logs.Info("Starting CPU prof")
			CPUfile, _ = Prof.StartCPUProfile()
			if CPUfile == nil {
				logs.Fatal("Could not start CPU prof / write file")
			}
		}
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
	if CPUfile != nil {
		logs.Info("Stop CPU prof")
		Prof.StopCPUProfile()
		CPUfile.Close()
	}
	stop_chan <- struct{}{} // force waiters to stop
	wg.Wait()
	time.Sleep(time.Second)
	logs.Info("Exit: %s", os.Args[0])
} // end func main
