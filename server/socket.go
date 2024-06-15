package server

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/go-while/nodare-db-dev/logger"
	"github.com/go-while/nodare-db-dev/utils"
	"log"
	"net"
	"net/textproto"
	"os"
	"strings"
	"sync"
	"time"
	//"os/signal"
	//"syscall"
)

type SOCKET struct {
	stop_chan      chan struct{}
	wg             sync.WaitGroup
	mux            sync.Mutex
	cpu            sync.Mutex
	mem            sync.Mutex
	CPUfile        *os.File
	logs           ilog.ILOG
	socket         *os.File
	socketPath     string
	socketlistener net.Listener
	tcplistener    net.Listener
	tlslistener    net.Listener
	acl            *AccessControlList
}

var (
	DefaultACL map[string]bool // can be set before booting
)

func NewSocketHandler(cfg VConfig, logs ilog.ILOG, stop_chan chan struct{}, wg sync.WaitGroup) *SOCKET {
	sockets := &SOCKET{
		logs: logs,
	}
	logs.Debug("NewSocketHandler cfg='%#v'", cfg)
	sockets.stop_chan = stop_chan
	sockets.wg = wg
	host := cfg.GetString(VK_SERVER_HOST)
	tcpport := cfg.GetString(VK_SERVER_SOCKET_PORT_TCP)
	tlsport := cfg.GetString(VK_SERVER_SOCKET_PORT_TLS)
	socketPath := cfg.GetString(VK_SERVER_SOCKET_PATH)
	tcpListen := host + ":" + tcpport
	tlsListen := host + ":" + tlsport
	tlscrt := cfg.GetString(VK_SEC_TLS_PUBCERT)
	tlskey := cfg.GetString(VK_SEC_TLS_PRIVKEY)
	tlsenabled := cfg.GetBool(VK_SEC_TLS_ENABLED)

	// setup acl
	sockets.acl = NewACL()
	iplist := cfg.GetString(VK_SERVER_SOCKET_ACL)
	if iplist != "" {
		ips := strings.Split(iplist, ",")
		for _, ip := range ips {
			sockets.acl.SetACL(ip, true)
		}
	}
	sockets.Start(tcpListen, tlsListen, socketPath, tlscrt, tlskey, tlsenabled)
	time.Sleep(time.Second / 100)
	return sockets
}

func (c *SOCKET) CloseSocket() {
	c.wg.Add(1)
	defer c.wg.Done()
	stopnotify := <-c.stop_chan // waits for signal from main
	c.socketlistener.Close()
	os.Remove(c.socketPath)
	c.logs.Debug("Socket closed")
	c.stop_chan <- stopnotify // push back in to notify others
}

func (c *SOCKET) Start(tcpListen string, tlsListen string, socketPath string, tlscrt string, tlskey string, tlsenabled bool) {
	// socket listener
	go func(socketPath string) {
		c.wg.Add(1)
		defer c.wg.Done()
		if socketPath == "" {
			return
		}
		c.wg.Add(1)
		defer c.wg.Done()
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			log.Fatalf("ERROR SOCKET err='%v'", err)
			return
		}
		c.logs.Info("SOCKET Path: %s", socketPath)
		c.socketPath = socketPath
		c.socketlistener = listener
		go c.CloseSocket()
		for {
			conn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					c.logs.Info("Closing SOCKET")
					return
				}
				c.logs.Warn("SOCKET err='%v'", err)
				continue
			}
			go c.handleSocketConn(conn, "", true)
		}
	}(socketPath)

	// tcp listener
	go func(tcpListen string) {
		if tcpListen == "" {
			return
		}
		c.wg.Add(1)
		defer c.wg.Done()
		listener, err := net.Listen("tcp", tcpListen)
		if err != nil {
			log.Fatalf("ERROR SOCKET creating tcpListen err='%v'", err)
			return
		}
		c.logs.Info("SOCKET TCP: %s", tcpListen)
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			raddr := getRemoteIP(conn)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					c.logs.Info("Closing TCP SOCKET")
					return
				}
				c.logs.Warn("ERROR SOCKET accepting tcp err='%v'", err)
				continue
			}
			if !c.acl.checkACL(conn) {
				log.Printf("TCP SOCKET !ACL: '%s'", raddr)
				conn.Close()
				continue
			}
			log.Printf("TCP SOCKET newConn: '%s'", raddr)
			go c.handleSocketConn(conn, raddr, false)
		}
	}(tcpListen)

	// tls listener
	go func(tlsListen string, tlscrt string, tlskey string, tlsenabled bool) {
		if tlsListen == "" || !tlsenabled {
			return
		}
		certs, err := tls.LoadX509KeyPair(tlscrt, tlskey)
		if err != nil {
			log.Fatalf("ERROR tls.LoadX509KeyPair err='%v'", err)
		}
		ssl_conf := &tls.Config{
			Certificates: []tls.Certificate{certs},
			//MinVersion: tls.VersionTLS12,
			//MaxVersion: tls.VersionTLS13,
		}
		c.wg.Add(1)
		defer c.wg.Done()
		listener_ssl, err := tls.Listen("tcp", tlsListen, ssl_conf)
		if err != nil {
			log.Fatalf("ERROR SOCKET tls.Listen err='%v'", err)
			return
		}
		defer listener_ssl.Close()
		c.logs.Info("SOCKET TLS: %s", tlsListen)
		for {
			conn, err := listener_ssl.Accept()
			raddr := getRemoteIP(conn)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					c.logs.Info("Closing TLS SOCKET")
					return
				}
				c.logs.Warn("ERROR TLS SOCKET accepting tcp err='%v'", err)
				continue
			}
			if !c.acl.checkACL(conn) {
				log.Printf("SOCKET TLS !ACL: '%s'", raddr)
				conn.Close()
				continue
			}
			log.Printf("SOCKET TLS newConn: '%s'", raddr)
			go c.handleSocketConn(conn, raddr, false)
		}
	}(tlsListen, tlscrt, tlskey, tlsenabled)
} // end func startServer

func (c *SOCKET) handleSocketConn(conn net.Conn, raddr string, socket bool) {
	defer conn.Close()
	tp := textproto.NewConn(conn)
	if !socket {
		// send welcome banner to incoming tcp connection
		err := tp.PrintfLine("200 NDB")
		if err != nil {
			return
		}
	}
	//var set, tmpset uint64

	/*
		var add, tmpadd uint64

		var get, tmpget uint64
		var del, tmpdel uint64
	*/
	var mode = no_mode
	var state int8 = -1
	var numBy int
	var k, v string
	var reply []string
	var cmd string
readlines:
	for {
		line, err := tp.ReadLine()
		if err != nil {
			log.Printf("Error handleConn err='%v'", err)
			break readlines
		}
		// clients sends: CMD|num_of_lines\r\n
		// followed by multiple lines with BEL byte \x07 as delim of k:v pairs
		// with a single line containing a ETB \x17 when done:
		// ...data\r\n\x17\r\n or CR LF ETB CR LF after last byte of data!
		// any values (or keys?!) aka lines containing \r\n only
		// must be escaped by client before sending and unescape after retrieval!
		//
		// server replies on order of sending

		// 	ADD|4\r\n
		// 		AveryLooongKey1111111NameforThisLIST\r\n
		//		aValue01forThisList\r\n
		//		aValue02forThisList\r\n
		//		aValue03forThisList\r\n
		//		\x17\r\n

		// 	SET|6\r\n
		// 		AveryLooongKey11\r\n
		// 		AveryLongValue\r\n
		// 		\x07\r\n
		// 		AveryLooongKey22\r\n
		// 		AveryLooooongValue\r\n
		// 		\x07\r\n
		// 		AveryLooongKey33\r\n
		// 		AveryLonoooooongValue\r\n
		// 		\x17\r\n

		//	GET|3\r\n
		// 		AveryLooongKey\r\n
		// 		AnotherLooooooooongKey\r\n
		//		NeedMOaaaarKeysKey\r\n
		//		\x17\r\n

		//	DEL|5\r\n
		// 		AveryLooongKey\r\n
		// 		AnotherLooooooooongKey\r\n
		//		NeedMOaaaarKeysKey\r\n
		//		HavenomoaarKeys\r\n
		//		OneKeyMoarPlease\r\n
		//		\x17\r\n

		switch mode {
		case modeADD:
			// TODO process multiple Add lines here.

		case modeSET:
			// TODO process multiple Set lines here.
			log.Printf("SOCKET SET: line='%s'", line)
			// receive first line with key at state 0
			// receive second line with value at state 1
			// if client sends \x07 (BEL) flip state to 0 and continue reading lines
			// if client sends \x17 (ETB) set state=-1 AND mode=no_mode
			// and send reply to client

			switch state {
			case 0: // state 0 reads key
				if len(line) > KEY_LIMIT {
					tp.PrintfLine(CAN)
					break readlines
				}
				k = line
				state++ // state is 1 now
				continue readlines

			case 1: // state 1 reads val
				if len(line) > VAL_LIMIT {
					tp.PrintfLine(CAN)
					break readlines
				}
				v = line
				numBy--

				// TODO!
				// got a k,v pair!

				// process Set request to db

				// store reply in slice and reply in one batch
				// or flush frequently to cli
				// or pass answer to cli now
				// ????

				log.Printf("SOCKET recv k='%s' v='%s' rep=%d", k, v, len(reply))
				state++ // state is 2 now
				continue readlines

			case 2: // state 2 reads ETB or BEL
				if len(line) != 1 {
					tp.PrintfLine(CAN)
					break readlines
				}
				switch line {
				case ETB:
					// client finished streaming command: SET
					mode = no_mode
					state = -2 // command done
					// state reverts when client sends next command
					continue readlines
				case BEL:
					// client continues sending k,v pairs
					continue readlines
				}
			}

		case modeGET:
			// TODO process multiple Get lines here.
			log.Printf("SOCKET GET: '%s'", line)

		case modeDEL:
			// TODO process multiple Del lines here.
			log.Printf("SOCKET DEL: '%s'", line)

		case no_mode:
			// 1st arg is command
			// 2nd arg is number of bytes client wants to send
			// 		or run,wait for MemProfile
			// len min: X|1  || 2nd is not '|' || line tooooooooooooooooo long
			if len(line) < 3 || line[1] != '|' {
				// invalid format
				tp.PrintfLine(CAN)
				break readlines
			}
			state = -1
			// no mode is set: find command and set mode to accept reading of multiple lines
			split := strings.Split(line, "|")[0:2]
			if len(split) < 2 {
				tp.PrintfLine(CAN)
				break readlines
			}
			cmd = string(split[0])
			switch cmd {

			//case magicA: // ADD
			//	mode = modeADD

			//case magicL: // LIST
			//	mode = modeLIST

			case magicS: // SET key => value
				numBy = utils.Str2int(split[1])
				if numBy == 0 {
					// abnormal: str2num failed parsing
					// or client send really a 0
					break readlines
				}
				mode = modeSET
				state++ // should be 0 now
				continue readlines

			case magicG: // GET key returns value or NUL
				numBy = utils.Str2int(split[1])
				if numBy == 0 {
					// abnormal: str2num failed parsing
					// or client send really a 0
					break readlines
				}
				mode = modeGET
				state++ // should be 0 now
				continue readlines

			case magicD: // DEL key
				numBy = utils.Str2int(split[1])
				if numBy == 0 {
					// abnormal: str2num failed parsing
					// or client send really a 0
					break readlines
				}
				mode = modeDEL
				state++ // should be 0 now
				continue readlines

			case magic1:
				// 		M|  <--- use default: run capture 30 sec instantly
				// 		M|60,30  <--- runs 60 secs but waits 30 sec before
				//
				// start mem profiling
				// cant stop or run twice
				// further calls lockin and run when running one finishes
				// allows some kind of queue for mem profiles
				if !socket {
					break readlines
				}
				// default
				runi := 30
				waiti := 0
				var args []string
				if strings.Contains(split[1], ",") {
					args = strings.Split(split[1], ",")[0:2]
					run := utils.Str2int(args[0])
					wait := utils.Str2int(args[1])
					if run > 0 && wait >= 0 {
						runi = run
						waiti = wait
					}
				}
				go func(runi int, waiti int) {
					log.Printf("Lock MemProfile run=(%d sec) wait=(%d sec)", runi, waiti)
					c.mem.Lock()
					log.Printf("StartMemProfile run=(%d sec) wait=(%d sec)", runi, waiti)
					run := time.Duration(runi) * time.Second
					wait := time.Duration(waiti) * time.Second
					Prof.StartMemProfile(run, wait)
					c.mem.Unlock()
				}(runi, waiti)
				tp.PrintfLine("200 StartMemProfile run=%d wait=%d", runi, waiti)

			case magic2:
				// start/stop cpu profiling
				if !socket {
					break readlines
				}
				c.cpu.Lock()
				if c.CPUfile != nil {
					Prof.StopCPUProfile()
					tp.PrintfLine("200 StopCPUProfile")
					c.CPUfile = nil
				} else {
					CPUfile, err := Prof.StartCPUProfile()
					if err != nil || CPUfile == nil {
						log.Printf("ERROR SOCKET StartCPUProfile err='%v'", err)
						tp.PrintfLine("400 ERR StartCPUProfile")
					} else {
						c.CPUfile = CPUfile
						tp.PrintfLine("200 StartCPUProfile")
					}
				}
				c.cpu.Unlock()

			case magicZ:
				// quit
				break readlines

			default:
				// unknown cmd
				break readlines

			} // end switch cmd
		} // end switch mode
		continue readlines
	} // end for readlines
	log.Printf("handleConn LEFT: %#v", conn)
} // end func handleConn

func getRemoteIP(conn net.Conn) string {
	remoteAddr := conn.RemoteAddr()
	if tcpAddr, ok := remoteAddr.(*net.TCPAddr); ok {
		return fmt.Sprintf("%s", tcpAddr.IP)
	}
	return "x"
}

func (a *AccessControlList) checkACL(conn net.Conn) bool {
	return a.IsAllowed(getRemoteIP(conn))
}

type AccessControlList struct {
	mux sync.RWMutex
	acl map[string]bool
}

func NewACL() *AccessControlList {
	acl := &AccessControlList{}
	acl.SetupACL()
	return acl
}

func (a *AccessControlList) SetupACL() {
	a.mux.Lock()
	defer a.mux.Unlock()
	if a.acl != nil {
		return
	}
	if DefaultACL != nil {
		a.acl = DefaultACL
		return
	}
	a.acl = make(map[string]bool)
}

func (a *AccessControlList) IsAllowed(ip string) bool {
	a.mux.RLock()
	retval := a.acl[ip]
	a.mux.RUnlock()
	return retval
}

func (a *AccessControlList) SetACL(ip string, val bool) {
	a.mux.Lock()
	defer a.mux.Unlock()
	if !val { // unset
		delete(a.acl, ip)
		return
	}
	a.acl[ip] = val
}
