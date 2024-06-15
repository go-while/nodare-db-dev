package main

import (
	"fmt"
	"flag"
	"github.com/go-while/nodare-db-dev/client/clilib"
	"log"
	"os"
	"sync"
	"time"
)

var (
	wg sync.WaitGroup
	addr string
	sock string
	mode int // mode=1=http(s) || mode = 2 raw tcp (with tls)
	ssl bool
	items int
	parallel int
	rounds int
	logfile string
)

func main() {
	stop_chan := make(chan struct{}, 1)
	testWorker := true // runs a test after connecting
	daemon := false

	flag.StringVar(&addr, "addr", "", "uri to non-default http(s) (addr:port)")
	flag.StringVar(&sock, "sock", "", "uri to non-default socket (addr:port)")
	flag.IntVar(&mode, "mode", 1, "mode=1=http(s) | mode=2=socket")
	flag.BoolVar(&ssl, "ssl", false, "use secure connection")
	flag.IntVar(&parallel, "workers", 4, "start N workers")
	flag.IntVar(&items, "items", 100000, "insert this many items per parallel worker")
	flag.IntVar(&rounds, "rounds", 10, "do N rounds:  distribute over workers")
	flag.StringVar(&logfile, "logfile", "", "logfile for client")
	flag.Parse()

	netcli, err := client.NewClient(&client.Options{
		SSL:        ssl,
		Addr:       addr,
		Mode:       mode,
		Stop:       stop_chan,
		Daemon:     daemon,
		TestWorker: testWorker,
	})
	if netcli == nil || err != nil {
		log.Printf("ERROR netcli='%v' err='%v'", netcli, err)
		return
	}
	//log.Printf("netcli='%#v'", netcli)

	parchan := make(chan struct{}, parallel)
	retchan := make(chan map[string]string, 1)
	log.Printf("starting insert")
	start := time.Now().Unix()

	// launch insert worker
	for r := 1; r <= rounds; r++ {
		time.Sleep(100 * time.Millisecond)
		go func(r int, items int, parchan chan struct{}, retchan chan map[string]string) {
			parchan <- struct{}{} // locks
			testmap := make(map[string]string)
			log.Printf("launch insert worker r=%d", r)
			for i := 1; i <= items; i++ {
				// %010 leftpads i and r with 10 zeroes, like 17 => 0000000017
				key := fmt.Sprintf("atestKey%010d-r-%010d", i, r)
				val := fmt.Sprintf("atestVal%010d-r-%010d", i, r)
				res, err := netcli.Set(key, val)
				if err != nil {
					log.Fatalf("ERROR set key='%s' => val='%s' err='%v' res='%v'", key, val, err, res)
				}
				testmap[key] = val
			}
			<-parchan // returns lock
			retchan <- testmap
			log.Printf("returned insert worker r=%d set=%d", r, len(testmap))
		}(r, items, parchan, retchan)
	}

	log.Printf("wait for insert worker to return maps to test K:V")
	var capturemaps []map[string]string
forever:
	for {
		select {
		case testmap := <-retchan:
			capturemaps = append(capturemaps, testmap)
			log.Printf("wait got a testmap have=%d want=%d", len(capturemaps), rounds)
		default:
			if len(capturemaps) == rounds {
				log.Printf("OK all testmaps returned, checking now...")
				break forever
			}
			time.Sleep(time.Millisecond * 100)
		}
	}
	insert_end := time.Now().Unix()
	log.Printf("insert finished: took %d sec! checking...", insert_end-start)

	// check all testmaps
	retint := make(chan int, len(capturemaps))
	for _, testmap := range capturemaps {
		go func(parchan chan struct{}, retint chan int, testmap map[string]string) {
			parchan <- struct{}{} // locks
			checked := 0
			for k, v := range testmap {
				val, err := netcli.Get(k) // check GET
				if err != nil {
					log.Fatalf("ERROR get k='%s' err='%v'", k, err)
				}
				if val != v {
					log.Fatalf("ERROR verify k='%s' v='%s' != val='%s'", k, v, val)
					os.Exit(1)
				}
				checked++
			}
			<-parchan // returns lock
			retint <- checked
		}(parchan, retint, testmap)
	}

	checked := 0
final:
	for {
		select {
		case aint := <-retint:
			checked += aint
			if checked == items*rounds {
				break final
			}
		}
	}
	test_end := time.Now().Unix()

	log.Printf("\n test parallel=%d total=%d/%d \n items/round=%d rounds=%d\n insert took %d sec \n check took %d sec \n total %d sec", parallel, checked, items*rounds, items, rounds, insert_end-start, test_end-insert_end, test_end-start)

	log.Printf("infinite wait on stop_chan")
	<-stop_chan
}
