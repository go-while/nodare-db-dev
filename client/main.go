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
	"encoding/hex"
	"os"
	"sync"
	"time"
	crand "crypto/rand"
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
	startint int
	runtest  bool // runs a client internal test after connecting (not implemented)
	randomize  bool
	logfile  string
	keylen  int
	vallen  int
)

func main() {
	stop_chan := make(chan struct{}, 1)
	flag.BoolVar(&daemon, "daemon", false, "launch workers in background")
	flag.StringVar(&addr, "addr", "", "uri to non-default http(s) (addr:port)")
	flag.StringVar(&sock, "sock", "", "uri to non-default socket (addr:port)")
	flag.IntVar(&mode, "mode", 2, "mode=1=http(s) | mode=2=socket")
	flag.BoolVar(&ssl, "ssl", false, "use secure connection")
	flag.IntVar(&items, "items", 125000, "insert this many items per parallel worker")
	flag.IntVar(&rounds, "rounds", 8, "test do N rounds")
	flag.IntVar(&parallel, "parallel", 8, "limits parallel tests to N conns")
	flag.IntVar(&startint, "startint", 1, "start test at int value N\n  both server and test-client especially may eat up lots of memory\n  so we can add 1billion k:v in steps")
	flag.BoolVar(&runtest, "runtest", true, "runs the test after connecting")
	flag.BoolVar(&randomize, "random", true, "if true uses random key:vals and caputures these in maps\n  so we can check them later. eats loads of memory!")
	flag.IntVar(&keylen, "keylen", 16, "set length of key. used with -random=true")
	flag.IntVar(&vallen, "vallen", 16, "set length of val. used with -random=true")
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

	cliChan := make(chan *client.Client, parallel)
	parChan := make(chan struct{}, parallel)
	MapRetChan := make(chan map[string]string, rounds)
	SetDoneChan := make(chan struct{}, rounds)
	GetDoneChan := make(chan struct{}, rounds)
	RetIntChan := make(chan int, rounds)
	start := time.Now().Unix()
	var insert_end int64

	// launch insert tests
	for r := 1; r <= rounds; r++ {
		time.Sleep(1 * time.Millisecond) // mini delay to have them spawn in order
		go func(cliHandler *client.CliHandler, opts *client.Options, r int, rounds int, items int, parChan chan struct{}, cliChan chan *client.Client, MapRetChan chan map[string]string, startint int, randomize bool, SetDoneChan chan struct{}, keylen int, vallen int) {
			parChan <- struct{}{} // locks parallel
			logs.Info("parChan got lock: start insert test r=%d/%d", r, rounds)
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
			var set int
			for i := 1; i <= items; i++ {
				var key, val, resp string
				//var rkey, rval string
				switch randomize {
					case true:
						// use random key:val and pass K:v to capturemaps to test later
						DevUrandomString(keylen, &key)
						DevUrandomString(vallen, &val)
						//key = fmt.Sprintf("%s-r%d-%d", rkey, r, startint)
						//val = fmt.Sprintf("%s-r%d-%d", rval, r, startint)
					case false:
						// use upcounting / startint key:val
						// %010 leftpads startint and round with 10 zeroes, like 17 => 0000000017
						key = fmt.Sprintf("%d_Atestkey%010d-r-%010d", startint, startint, r)
						val = fmt.Sprintf("%d_aTestVal%010d-r-%010d",startint, startint, r)
				}

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
				if randomize {
					testmap[key] = val
				}
				startint++
				set++
			} // end for items
			if randomize {
				logs.Info("OK insert random test round=%d/%d set=%d items=%d testmap=%d", r, rounds, set, items, len(testmap))
			} else {
				logs.Info("OK insert upcunt test round=%d/%d set=%d items=%d", r, rounds, set, items)
			}
			cliChan <- netCli // return netCli
			if randomize {
				MapRetChan <- testmap
			}
			SetDoneChan <- struct{}{}
			<-parChan // returns lock parallel
			logs.Debug("<- insert test r=%d returned parChan", r)
		}(cliHandler, clientOpts, r, rounds, items, parChan, cliChan, MapRetChan, startint, randomize, SetDoneChan, keylen, vallen)
		//^^ go func
		// wait for setDone
	} // end insert worker

	// wait
	logs.Info("wait for SetDoneChan")
	setdone := 0
	for {
		<- SetDoneChan
		setdone++
		if setdone == rounds {
			logs.Info("OK setdone=%d rounds=%d", setdone, rounds)
			insert_end = time.Now().Unix()
			logs.Info("insert finished (random=%t): took %d sec! checking...", randomize, insert_end-start)
			break
		}
	}

	// start checks
	if !randomize {
		logs.Info("start check !randomize")

		for r := 1; r <= rounds; r++ {
			time.Sleep(1 * time.Millisecond) // mini delay to have them spawn in order
			go func(cliHandler *client.CliHandler, opts *client.Options, r int, rounds int, items int, parChan chan struct{}, cliChan chan *client.Client, MapRetChan chan map[string]string, startint int, randomize bool, SetDoneChan chan struct{}, GetDoneChan chan struct{}) {
				parChan <- struct{}{} // locks parallel
				var netCli *client.Client
				select {
					case netCli = <- cliChan:
						logs.Info("check test: got open netCli")
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
				var err error
				var checked int
				for i := 1; i <= items; i++ {
					var checkkey, checkval, retval, nfk string
					var found bool
					checkkey = fmt.Sprintf("%d_Atestkey%010d-r-%010d", startint, startint, r)
					checkval = fmt.Sprintf("%d_aTestVal%010d-r-%010d", startint, startint, r)
					switch netCli.Mode {
						case 1:
							// http mode
							err = netCli.HTTP_Get(checkkey, &retval, &found) // http Get Key: return val is passed as pointer!
						case 2:
							// sock mode
							// TODO! add test for GetMany
							err = netCli.SOCK_Get(checkkey, &retval, &nfk, &found) // socket Get key: return val is passed as pointer!
					}
					if err != nil {
						log.Fatalf("ERROR ?_Get k='%s' err='%v' mode=%d nfk='%s' found=%t", checkkey, err, netCli.Mode, nfk, found)
					}
					if !found || retval != checkval {
						log.Fatalf("ERROR verify checkkey='%s' checkval='%s' != retval='%s' nfk='%s' found=%t", checkkey, checkval, retval, nfk, found)
						os.Exit(1)
					}
					startint++
					checked++
				} // end for items
				cliChan <- netCli // return netCli
				GetDoneChan <- struct{}{}
				RetIntChan <- checked // returns checked amount to sum later
				<- parChan
				logs.Info("<- check test !random r=%d returned parChan", r)
			}(cliHandler, clientOpts, r, rounds, items, parChan, cliChan, MapRetChan, startint, randomize, SetDoneChan, GetDoneChan) // end go func
		} // end for rounds

		// wait
		logs.Info("wait for GetDoneChan !random")
		getdone := 0
		for {
			<- GetDoneChan
			getdone++
			logs.Info("wait GetDoneChan random=%t getdone=%d / rounds=%d", randomize, getdone, rounds)
			if getdone == rounds {
				logs.Info("OK getdone=%d rounds=%d", getdone, rounds)
				break
			}
		}
	} // end if !randomize

	if randomize {
		logs.Info("wait for insert test to return randomized maps to test K:V")
		var capturemaps []map[string]string
		for {
			testmap := <-MapRetChan
			capturemaps = append(capturemaps, testmap)
			logs.Info("Got a testmap have=%d want=%d", len(capturemaps), rounds)
			if len(capturemaps) == rounds {
				logs.Info("OK all testmaps returned, checking now...")
				break
			}
		} // end for wait capture testmaps

		// check all testmaps
		for i, testmap := range capturemaps {
			r := i+1
			go func(parChan chan struct{}, cliChan chan *client.Client, RetIntChan chan int, GetDoneChan chan struct{}, testmap map[string]string, r int) {
				parChan <- struct{}{} // locks
				var netCli *client.Client
				select {
					case netCli = <- cliChan:
						logs.Info("check test: got open netCli")
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
				var checked int
				var err error
				for k, v := range testmap {
					var val, nfk string
					var found bool
					switch netCli.Mode {
						case 1:
							// http mode
							err = netCli.HTTP_Get(k, &val, &found) // http Get Key: return val is passed as pointer!
						case 2:
							// sock mode
							// TODO! add test for GetMany
							err = netCli.SOCK_Get(k, &val, &nfk, &found) // socket Get key: return val is passed as pointer!
					}
					if err != nil {
						log.Fatalf("ERROR randomize ?_Get k='%s' err='%v' mode=%d nfk='%s' found=%t", k, err, netCli.Mode, nfk, found)
					}
					if !found || val != v {
						log.Fatalf("FAILED verify randomize k='%s' v='%s' != val='%s' nfk='%s' found=%t checked=%d", k, v, val, nfk, found, checked)
						os.Exit(1)
					}
					checked++
				} // end for testmap
				cliChan <- netCli // return netCli to other rounds
				RetIntChan <- checked // returns checked amount to sum later
				GetDoneChan <-struct{}{}
				<-parChan // returns lock
				logs.Info("<- check test random r=%d returned parChan", r)
			}(parChan, cliChan, RetIntChan, GetDoneChan, testmap, r)
		} // end for check test maps

		// wait
		logs.Info("wait for GetDoneChan random")
		getdone := 0
		for {
			<- GetDoneChan
			getdone++
			logs.Info("wait GetDoneChan random=%t getdone=%d / rounds=%d", randomize, getdone, rounds)
			if getdone == rounds {
				logs.Info("OK getdone=%d rounds=%d", getdone, rounds)
				break
			}
		}

	} // end if randomize

	time.Sleep(time.Second)

	// sums all checks
	checked := 0
	logs.Info("wait to return checked")
final:
	for {
		select {
		case aint := <-RetIntChan:
			checked += aint
			if checked == items*rounds {
				break final
			}
		}
	}
	test_end := time.Now().Unix()
	diff1 := insert_end-start
	diff2 := test_end-start
	if diff1 <= 0 || diff2 <= 0 {
		logs.Info("Check done!\n but calculating a benchmark value would result in division by zero error ....\n   just because it was tooo fast!\n    if you have not seen any other error:\n     everything went smoothly!")
	} else {
	logs.Info("Check done! Test Result:\n{\n parallel: %d\n total: %d\n checked: %d\n items/round: %d\n rounds: %d\n insert: %d sec (%d/sec)\n check: %d sec (%d/sec)\n total: %d sec\n}",
											parallel,
											items*rounds,
											checked,
											items,
											rounds,
											insert_end-start,
											int64(checked)/(insert_end-start),
											test_end-insert_end, int64(checked)/(test_end-insert_end),
											test_end-start)
	}
	logs.Info("infinite wait on stop_chan")
	<-stop_chan

} // end func main:client/main.go

func DevUrandomString(length int, retstr *string) {
	uselen := length / 2
	if uselen <= 0 {
		uselen = 1
	}
	b := make([]byte, uselen)
	_, _ := crand.Read(b)
	*retstr = hex.EncodeToString(b)
	//log.Printf("DevUrandomString read=%d n=%d err='%v' retstr='%s'=%d wanted=%d", len(b), n, err, *retstr, len(*retstr), length)
	// ignores any errors, will fail anyways later ;)
} // end func DevUrandomString
