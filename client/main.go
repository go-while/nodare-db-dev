/*
 * Test Client App
 *
 */
package main

import (
	"flag"
	"fmt"
	"github.com/go-while/nodare-db-dev/client/clilib"
	"github.com/go-while/nodare-db-dev/logger"
	"log"
	"os"
	"sync"
	"time"
)

var (
	wg       sync.WaitGroup
	daemon   bool
	addr     string
	sock     string
	mode     int // mode=1=http(s) || mode = 2 raw tcp (with tls)
	ssl      bool
	items    int
	rounds   int
	parallel int
	runtest  bool // runs a test after connecting
	logfile  string

)

func main() {
	stop_chan := make(chan struct{}, 1)
	flag.BoolVar(&daemon, "daemon", false, "launch workers in background")
	flag.StringVar(&addr, "addr", "", "uri to non-default http(s) (addr:port)")
	flag.StringVar(&sock, "sock", "", "uri to non-default socket (addr:port)")
	flag.IntVar(&mode, "mode", 1, "mode=1=http(s) | mode=2=socket")
	flag.BoolVar(&ssl, "ssl", false, "use secure connection")
	flag.IntVar(&items, "items", 100000, "insert this many items per parallel worker")
	flag.IntVar(&rounds, "rounds", 10, "test do N rounds")
	flag.IntVar(&parallel, "parallel", 8, "limits parallel tests to N conns")
	flag.BoolVar(&runtest, "runtest", true, "runs the test after connecting")
	flag.StringVar(&logfile, "logfile", "", "logfile for client")
	flag.Parse()

	logs := ilog.NewLogger(ilog.GetEnvLOGLEVEL(), logfile)
	cliHandler := client.NewCliHandler(logs)

	/*
	 * single connection
	 *
	 */

	/*
	netCli, err := client.NewCli(&client.Options{

	netCli, err := cliHandler.NewCli(&client.Options{
		SSL:        ssl,
		Addr:       addr,
		Mode:       mode,
		StopChan:   stop_chan,
		Daemon:     daemon,
		RunTest:    runtest,
		WG:         wg,
		Logs:       logs,
	})
	if netCli == nil || err != nil {
		log.Printf("ERROR netCli='%v' err='%v'", netCli, err)
		return
	}
	//log.Printf("netCli='%#v'", netCli)
	*/

	// defines opts once for all connections
	clientOpts := &client.Options{
		SSL:        ssl,
		Addr:       addr,
		Mode:       mode,
		StopChan:   stop_chan,
		Daemon:     daemon,
		RunTest:    runtest,
		WG:         wg,
		Logs:       logs,
	}

	parChan := make(chan struct{}, parallel)
	retchan := make(chan map[string]string, 1)
	cliChan := make(chan *client.Client, parallel)
	start := time.Now().Unix()

	logs.Debug("starting insert")
	// launch insert tests
	for r := 1; r <= rounds; r++ {
		time.Sleep(1 * time.Millisecond) // mini delay to have them spawn in order

		go func(cliHandler *client.CliHandler, opts *client.Options, r int, rounds int, items int, parChan chan struct{}, cliChan chan *client.Client, retchan chan map[string]string) {
			parChan <- struct{}{} // locks parallel
			var netCli *client.Client
			select {
				case netCli = <- cliChan:
					logs.Info("insert test: got open netCli")
					// pass
				default:
					// no conn in cliChan? establish new!
					newnetCli, err := cliHandler.NewCli(clientOpts)
					if newnetCli == nil || err != nil {
						logs.Error("ERROR netCli='%v' err='%v'", newnetCli, err)
						return
					}
					netCli = newnetCli
			}
			testmap := make(map[string]string)
			logs.Info("Launch insert test round=%d/%d", r, rounds)
			var err error
			var resp string
			for i := 1; i <= items; i++ {
				// %010 leftpads i and r with 10 zeroes, like 17 => 0000000017
				key := fmt.Sprintf("Atestkey%010d-r-%010d", i, r)
				val := fmt.Sprintf("aTestVal%010d-r-%010d", i, r)
				switch netCli.Mode {
					case 1:
						// http mode
						err = netCli.HTTP_Set(key, val, &resp)
					case 2:
						// sock mode
						// TODO! add test for SetMany
						err = netCli.SOCK_Set(key, val, &resp)
				}
				if err != nil {
					log.Fatalf("ERROR Set key='%s' => val='%s' err='%v' resp='%s' mode=%d", key, val, err, resp, mode)
				}
				testmap[key] = val
			}
			logs.Info("OK insert test round=%d/%d set=%d", r, rounds, len(testmap))
			cliChan <- netCli // return netCli
			retchan <- testmap
			<-parChan // returns lock parallel
		}(cliHandler, clientOpts, r, rounds, items, parChan, cliChan, retchan)
		//^^ go func
	} // end insert worker

	logs.Info("wait for insert test to return maps to test K:V")
	var capturemaps []map[string]string
forever:
	for {
		select {
		case testmap := <-retchan:
			capturemaps = append(capturemaps, testmap)
			logs.Info("Got a testmap have=%d want=%d", len(capturemaps), rounds)
		default:
			if len(capturemaps) == rounds {
				logs.Info("OK all testmaps returned, checking now...")
				break forever
			}
			time.Sleep(time.Millisecond * 100)
		}
	} // end for wait capture testmaps
	insert_end := time.Now().Unix()
	logs.Info("insert finished: took %d sec! checking...", insert_end-start)

	// check all testmaps
	retintChan := make(chan int, len(capturemaps))
	for _, testmap := range capturemaps {
		go func(parChan chan struct{}, cliChan chan *client.Client, retintChan chan int, testmap map[string]string) {
			parChan <- struct{}{} // locks
			netCli := <-cliChan // gets open netCli
			checked := 0
			var err error
			var val string
			var nfk string

			for k, v := range testmap {
				var found bool
				switch netCli.Mode {
					case 1:
						// http mode
						err = netCli.HTTP_Get(k, &val) // http Get Key: return val is passed as pointer!
					case 2:
						// sock mode
						// TODO! add test for GetMany
						found, err = netCli.SOCK_Get(k, &val, &nfk) // socket Get key: return val is passed as pointer!
				}
				if err != nil || !found {
					log.Fatalf("ERROR ?_Get k='%s' err='%v' mode=%d nfk='%s' found=%t", k, err, netCli.Mode, nfk, found)
				}
				if val != v {
					log.Fatalf("ERROR verify k='%s' v='%s' != val='%#v' nfk='%s'", k, v, val, nfk)
					os.Exit(1)
				}
				checked++
			}
			cliChan <- netCli // return netCli to other rounds
			retintChan <- checked // returns checked amount to sum later
			<-parChan // returns lock
		}(parChan, cliChan, retintChan, testmap)
	} // end for check test maps

	// sums all checks
	checked := 0
final:
	for {
		select {
		case aint := <-retintChan:
			checked += aint
			if checked == items*rounds {
				break final
			}
		}
	}
	test_end := time.Now().Unix()
	logs.Info("Check done! Result {\n test parallel=%d\n total=%d\n checked=%d\n items/round=%d\n rounds=%d\n insert took %d sec \n check took %d sec \n total %d sec\n }", parallel, items*rounds, checked, items, rounds, insert_end-start, test_end-insert_end, test_end-start)
	logs.Info("infinite wait on stop_chan")
	<-stop_chan

} // end func main:client/main.go
