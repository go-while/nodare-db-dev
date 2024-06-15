package main

import (
	"flag"
	"github.com/go-while/go-cpu-mem-profiler"
	"github.com/go-while/nodare-db-dev/database"
	"github.com/go-while/nodare-db-dev/logger"
	"github.com/go-while/nodare-db-dev/server"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

const MODE = 1

var (
	wg              sync.WaitGroup
	flag_configfile string
	flag_logfile    string
	flag_hashmode   int

	Prof *prof.Profiler
	// setHASHER sets prefered hash algo
	// [ HASH_siphash | HASH_FNV32A | HASH_FNV64A ]
	// TODO config value HASHER
	setHASHER = database.HASH_FNV64A
	stop_chan chan struct{}
)

func main() {
	stop_chan = make(chan struct{}, 1)
	Prof = prof.NewProf()
	server.Prof = Prof

	// capture the flags: overwrites config file settings!
	flag.StringVar(&flag_configfile, "config", server.DEFAULT_CONFIG_FILE, "path to config.toml")
	flag.IntVar(&flag_hashmode, "hashmode", database.HASH_FNV64A, "sets hashmode:\n sipHash = 1\n FNV32A = 2\n FNV64A = 3\n")
	flag.StringVar(&flag_logfile, "logfile", "", "path to ndb.log")
	flag.Parse()

	database.HASHER = flag_hashmode
	// this first line prints LOGLEVEL="XX" to console but will never showup in logfile!
	logs := ilog.NewLogger(ilog.GetEnvLOGLEVEL(), flag_logfile)
	cfg, sub_dicks := server.NewViperConf(flag_configfile, logs)

	switch MODE {
	case 0:
		// spaceholder

	case 1:
		db := database.NewDICK(logs, sub_dicks)
		if database.HASHER == database.HASH_siphash {
			db.XDICK.GenerateSALT()
		}
		srv := server.NewFactory().NewNDBServer(cfg, server.NewXNDBServer(db, logs), logs, stop_chan, wg)

		if logs.IfDebug() {
			logs.Debug("Mode 1: Loaded vcfg='%#v' host='%v'", cfg, cfg.GetString(server.VK_SERVER_HOST))
			logs.Debug("Mode 1: Booted DB sub_dicks=%d srv='%v'", sub_dicks, srv)
			logs.Debug("launching PprofWeb @ :1234")
			go Prof.PprofWeb(":1234")
		}
		go srv.Start()
	default:
		log.Fatalf("Invalid MODE=%d", MODE)
	}
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	stop_chan <- struct{}{} // force waiters to stop
	wg.Wait()
	logs.Info("Quit: %s", os.Args[0])
} // end func main
