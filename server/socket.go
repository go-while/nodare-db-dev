package server

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/go-while/nodare-db-dev/database"
	"github.com/go-while/nodare-db-dev/logger"
	"github.com/go-while/nodare-db-dev/utils"
	"log"
	"io"
	"net"
	"net/textproto"
	"os"
	"strings"
	"sync"
	"time"
)

type SOCKET struct {
	stop_chan      chan struct{}
	db             *database.XDatabase
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
	id             uint64

}

type CLI struct {
	id             uint64
	conn           net.Conn
	tp             *textproto.Conn
} // end CLI struct

var (
	DefaultACL map[string]bool // can be set before booting
)

func NewSocketHandler(cfg VConfig, logs ilog.ILOG, stop_chan chan struct{}, wg sync.WaitGroup, db *database.XDatabase) *SOCKET {
	sockets := &SOCKET{
		logs: logs,
		db: db,
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

func (sock *SOCKET) CloseSocket() {
	sock.wg.Add(1)
	defer sock.wg.Done()
	stopnotify := <-sock.stop_chan // waits for signal from main
	sock.socketlistener.Close()
	os.Remove(sock.socketPath)
	sock.logs.Debug("Socket closed")
	sock.stop_chan <- stopnotify // push back in to notify others
}

func (sock *SOCKET) Start(tcpListen string, tlsListen string, socketPath string, tlscrt string, tlskey string, tlsenabled bool) {
	// socket listener
	go func(socketPath string) {
		sock.wg.Add(1)
		defer sock.wg.Done()
		if socketPath == "" {
			return
		}
		sock.wg.Add(1)
		defer sock.wg.Done()
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			log.Fatalf("ERROR SOCKET err='%v'", err)
			return
		}
		sock.logs.Info("SOCKET Path: %s", socketPath)
		sock.socketPath = socketPath
		sock.socketlistener = listener
		go sock.CloseSocket()
		for {
			conn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					sock.logs.Info("Closing SOCKET")
					return
				}
				sock.logs.Warn("SOCKET err='%v'", err)
				continue
			}
			sock.mux.Lock()
			sock.id++
			sock.mux.Unlock()
			cli := &CLI{
				conn: conn,
				id: sock.id,
			}
			go sock.handleSocketConn(cli, "", true)
		}
	}(socketPath)

	// tcp listener
	go func(tcpListen string) {
		if tcpListen == "" {
			return
		}
		sock.wg.Add(1)
		defer sock.wg.Done()
		listener, err := net.Listen("tcp", tcpListen)
		if err != nil {
			log.Fatalf("ERROR SOCKET creating tcpListen err='%v'", err)
			return
		}
		sock.logs.Info("SOCKET TCP: %s", tcpListen)
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			raddr := getRemoteIP(conn)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					sock.logs.Info("Closing TCP SOCKET")
					return
				}
				sock.logs.Warn("ERROR SOCKET accepting tcp err='%v'", err)
				continue
			}
			if !sock.acl.checkACL(conn) {
				sock.logs.Info("TCP SOCKET !ACL: '%s'", raddr)
				conn.Close()
				continue
			}
			sock.logs.Info("TCP SOCKET newConn: '%s'", raddr)
			sock.mux.Lock()
			sock.id++
			sock.mux.Unlock()
			cli := &CLI{
				conn: conn,
				id: sock.id,
			}
			go sock.handleSocketConn(cli, raddr, false)
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
		sock.wg.Add(1)
		defer sock.wg.Done()
		listener_ssl, err := tls.Listen("tcp", tlsListen, ssl_conf)
		if err != nil {
			log.Fatalf("ERROR SOCKET tls.Listen err='%v'", err)
			return
		}
		defer listener_ssl.Close()
		sock.logs.Info("SOCKET TLS: %s", tlsListen)
		for {
			conn, err := listener_ssl.Accept()
			raddr := getRemoteIP(conn)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					sock.logs.Info("Closing TLS SOCKET")
					return
				}
				sock.logs.Warn("ERROR TLS SOCKET accepting tcp err='%v'", err)
				continue
			}
			if !sock.acl.checkACL(conn) {
				sock.logs.Info("SOCKET TLS !ACL: '%s'", raddr)
				conn.Close()
				continue
			}
			sock.logs.Info("SOCKET TLS newConn: '%s'", raddr)
			sock.mux.Lock()
			sock.id++
			sock.mux.Unlock()
			cli := &CLI{
				conn: conn,
				id: sock.id,
			}
			go sock.handleSocketConn(cli, raddr, false)
		}
	}(tlsListen, tlscrt, tlskey, tlsenabled)
} // end func startServer

func (sock *SOCKET) handleSocketConn(cli *CLI, raddr string, socket bool) {
	defer cli.conn.Close()
	cli.tp = textproto.NewConn(cli.conn)
	if !socket {
		// send welcome banner to incoming tcp connection
		err := cli.tp.PrintfLine("200 X") // server.ACK
		if err != nil {
			return
		}
	}
	// counter
	//var add, tmpadd uint64
	var set, tmpset uint64
	var get, tmpget uint64
	var del, tmpdel uint64

	// session flags
	var mode = no_mode
	var state int8 = -1
	var numBy int
	var cmd string
	var key string
	var keys []string
	var vals map[string]*string
	var sentbytes int
	var recvbytes int
	var overwrite bool

readlines:
	for {
		line, err := cli.tp.ReadLine()
		if err != nil {
			sock.logs.Info("Error [cli=%d] handleConn err='%v'", cli.id, err)
			break readlines
		}
		recvbytes += len(line)+2 // does not account line endings delimiter "\r\n" ! +2 does!

		// clients sends: CMD|num_of_lines\r\n
		// followed by multiple lines with BEL byte \x07 as delim of k:v pairs
		// with a single line containing a ETB \x17 when done:
		// ...data\r\n\x17\r\n or CR LF ETB CR LF after last byte of data!
		// any values (or keys?!) aka lines containing \r\n only
		// must be escaped by client before sending and unescape after retrieval!
		//
		// server replies on order of sending

		// 	ADD|3\r\n
		// 		AveryLooongKey1111111NameforThisLIST\r\n
		//		aValue01forThisList\r\n
		//		aValue02forThisList\r\n
		//		aValue03forThisList\r\n
		//		\x17\r\n

		// SET needs "$OV" as overwrite flag
		// overwrite true = const Acknowledge "ACK" or \x05
		// overwrite false = const Negative Ack "NAK" or \x15

		// 	SET|1|$OF\r\n
		//		AveryLooongKey11\r\n
		//		AveryLongValue\r\n
		//		\x17\r\n

		// 	SET|3|$OF\r\n
		// 		AveryLooongKey11\r\n
		// 		AveryLongValue\r\n
		// 		\x07\r\n
		// 		AveryLooongKey22\r\n
		// 		AveryLooooongValue\r\n
		// 		\x07\r\n
		// 		AveryLooongKey33\r\n
		// 		AveryLonoooooongValue\r\n
		// 		\x17\r\n

		//	GET|1\r\n
		// 		AveryLooongKey\r\n
		//		\x17\r\n

		//	GET|3\r\n
		// 		AveryLooongKey\r\n
		// 		AnotherLooooooooongKey\r\n
		//		NeedMOaaaarKeysKey\r\n
		//		\x17\r\n

		//	DEL|1\r\n
		// 		AveryLooongKey\r\n
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
			sock.logs.Debug("SOCKET [cli=%d] modeADD line='%#v'", cli.id, line)
			// TODO process multiple Add lines here.

		case modeSET:
			sock.logs.Debug("SOCKET [cli=%d] modeSET line='%#v'", cli.id, line)
			// process multiple Set lines here.

			// receive first line with key at state 0
			// receive second line with value at state 1
			// if client sends \x07 (BEL) flip state to 0 and continue reading lines
			// if client sends \x17 (ETB): process request, clear keys/vals and set mode=no_mode
			// finally send reply to client

			switch state {
			case 0: // modeSET state 0 reads key
				if len(line) > KEY_LIMIT {
					cli.tp.PrintfLine(EOM)
					break readlines
				}
				key = line
				state++ // modeSET state is 1 now
				continue readlines

			case 1: // state 1 reads val
				if len(line) > VAL_LIMIT {
					cli.tp.PrintfLine(EOT)
					break readlines
				}
				// got a k,v pair!
				//v = line
				numBy-- // decrease counter
				tmpset++ // increase tmp counter, amount we have to set

				keys = append(keys, key)
				vals[key] = &line

				sock.logs.Debug("SOCKET [cli=%d] modeSET state1 recv k='%s' v='%s' keys=%d vals=%d", cli.id, key, line, len(keys), len(vals))
				key = ""
				state++ // modeSET state is 2 now
				continue readlines

			case 2: // modeSET state 2 reads ETB or BEL
				if len(line) != 1 {
					cli.tp.PrintfLine(CAN)
					break readlines
				}

				switch line {
				case ETB:
					sock.logs.Debug("SOCKET [cli=%d] modeSET state2 got ETB", cli.id)
					// client finished streaming
					// set key:val pairs
					setloopkeys:
					for _, akey := range keys {
						val := vals[akey]
						ok := sock.db.Set(akey, *val, overwrite) // default always overwrites
						if !ok {
							sock.logs.Error("SOCKET [cli=%d] modeSET state2 set !ok", cli.id)
							// reply error
							n, ioerr := io.WriteString(cli.conn, NUL+key)
							if ioerr != nil {
								// could not send reply, peer disconnected?
								sock.logs.Error("SOCKET [cli=%d] modeSET state2 reply !ok ioerr='%v'", cli.id, ioerr)
								break readlines
							}
							sentbytes += n
							continue setloopkeys
						}
						tmpset--
						set++
						sock.logs.Debug("SOCKET [cli=%d] modeSET state2 ETB Set k='%s' v='%s'", cli.id, akey, *val)
					} // end for keys

					// reply single ACK
					sock.logs.Debug("SOCKET [cli=%d] state2 reply ACK", cli.id)
					n, ioerr := io.WriteString(cli.conn, ACK+CRLF)
					if ioerr != nil {
						sock.logs.Error("SOCKET [cli=%d] modeSET state2 reply ioerr='%v'", cli.id, ioerr)
						break readlines
					}
					sentbytes += n
					keys, vals = nil, nil
					mode = no_mode // state reverts when client sends next command
					continue readlines

				case BEL:
					sock.logs.Debug("SOCKET [cli=%d] modeSET state2 got BEL", cli.id)
					// client continues sending k,v pairs
					continue readlines
				}
			}

		case modeGET:
			sock.logs.Debug("SOCKET [cli=%d] modeGET line='%#v'", cli.id, line)
			// process multiple Get lines here.

			// receive first line with key at state 0
			// if client sends \x07 (BEL) flip state to 0 and continue reading key lines
			// if client sends \x17 (ETB): process request, clear keys and set mode=no_mode
			// finally send reply to client

			switch state {

			case 0: // modeGET state 0 reads key
				if len(line) > KEY_LIMIT {
					cli.tp.PrintfLine(EOM)
					break readlines
				}
				numBy-- // decrease counter
				tmpget++ // increase tmp counter, amount we have to get
				keys = append(keys, line)
				state++ // modeGET state is 1 now
				continue readlines

			case 1: // modeGET state 1 reads ETB or BEL
				if len(line) != 1 {
					cli.tp.PrintfLine(CAN)
					break readlines
				}
				switch line {
				case ETB:
					lenk := len(keys)
					getloopkeys:
					for _, akey := range keys {
						var val string
						found := sock.db.Get(akey, &val)
						if !found {
							sock.logs.Error("SOCKET [cli=%d] modeGET state1 val nil", cli.id)
							// reply error
							retstr := NUL
							if lenk > 1 {
								retstr = retstr+key
							}
							n, ioerr := io.WriteString(cli.conn, retstr+CRLF)
							if ioerr != nil {
								// could not send reply, peer disconnected?
								sock.logs.Error("SOCKET [cli=%d] modeGET state1 replyERR ioerr='%v'", cli.id, ioerr)
								break readlines
							}
							sentbytes += n
							continue getloopkeys
						}
						n, ioerr := io.WriteString(cli.conn, val+CRLF)
						if ioerr != nil {
							// could not send reply, peer disconnected?
							sock.logs.Error("SOCKET [cli=%d] modeGET state1 replyACK ioerr='%v'", cli.id, ioerr)
							break readlines
						}
						tmpget--
						get++
						sentbytes += n
						sock.logs.Debug("SOCKET [cli=%d] modeGET state1 ETB Got k='%s' ?=> val='%s'", cli.id, akey, val)
					} // end for keys
					mode = no_mode
					keys = nil

				case BEL:
					state-- // reset state to read more keys
				} // end switch line
			} // end switch state

		case modeDEL:
			sock.logs.Debug("SOCKET [cli=%d] modeDEL line='%#v'", cli.id, line)

			// receive first line with key at state 0
			// if client sends \x07 (BEL) flip state to 0 and continue reading key lines
			// if client sends \x17 (ETB): process request, clear keys and set mode=no_mode
			// finally send reply to client

			switch state {
			// process multiple Del lines here.
			case 0: // state 0 reads key
				if len(line) > KEY_LIMIT {
					cli.tp.PrintfLine(EOM)
					break readlines
				}
				numBy-- // decrease counter
				tmpdel++ // increase tmp counter, amount we have to del
				keys = append(keys, line)
				state++ // modeDEL state is 1 now
				continue readlines

			case 1: // modeDEL state 1 reads ETB or BEL
				if len(line) != 1 {
					cli.tp.PrintfLine(CAN)
					break readlines
				}
				switch line {
				case ETB:
					delloopkeys:
					for _, akey := range keys {
						ok := sock.db.Del(akey)
						if !ok {
							sock.logs.Error("SOCKET [cli=%d] modeDEL state1 !ok", cli.id)
							// reply with error
							n, ioerr := io.WriteString(cli.conn, NUL+key+CRLF)
							if ioerr != nil {
								// could not send reply, peer disconnected?
								sock.logs.Error("SOCKET [cli=%d] modeDEL state1 replyERR ioerr='%v'", cli.id, ioerr)
								break readlines
							}
							sentbytes += n
							continue delloopkeys
						}
						n, ioerr := io.WriteString(cli.conn, ACK+CRLF)
						if ioerr != nil {
							// could not send reply, peer disconnected?
							sock.logs.Error("SOCKET [cli=%d] modeDEL state1 replyACK ioerr='%v'", cli.id, ioerr)
							break readlines
						}
						tmpdel--
						del++
						sentbytes += n
						sock.logs.Debug("SOCKET [cli=%d] modeDEL state1 ETB k='%s'", cli.id, akey)
					} // end for keys
				case BEL:
					state-- // reset state to read more keys
				} // end switch line
				continue readlines
			} // end switch state

		case no_mode:
			// ENTER STATE MACHINE
			keys = nil
			if vals == nil {
				vals = make(map[string]*string, 8)
			}
			// 1st arg is command
			// 2nd arg is number of keys client wants to set/get/del
			// 3rt arg is $OV overwrite flag
			// len min: X|1  || 2nd char is not '|'
			if len(line) < 3 || line[1] != '|' {
				// invalid format
				cli.tp.PrintfLine(CAN)
				break readlines
			}
			state = -1
			// no mode is set: find command and set mode to accept reading of multiple lines
			split := strings.Split(line, "|")
			if len(split) < 2 {
				sock.logs.Error("SOCKET FORMAT ERROR N00 len(split)=%d split='%#v' line='%#v' ??", len(split), split, line)
				cli.tp.PrintfLine(CAN)
				break readlines
			}
			cmd = string(split[0])
			//add, tmpadd = 0, 0
			set, tmpset = 0, 0
			del, tmpdel = 0, 0
			get, tmpget = 0, 0
			switch cmd {

			/*
			case MagicA: // ADD
				mode = modeADD

			case MagicL: // LIST
				mode = modeLIST
			*/

			case MagicS: // SET key => value
				numBy = utils.Str2int(split[1])
				if numBy == 0 {
					// abnormal: str2num failed parsing
					// or client send really a 0
					break readlines
				}
				//ovflag := string(split[2])
				if len(split) < 3 || len(split[2]) != 1 {
					sock.logs.Error("SOCKET FORMAT ERROR N01 SET len(split)=%d split='%#v' line='%#v' ??", len(split), split, line)
					cli.tp.PrintfLine(CAN)
					break readlines
				}
				switch split[2] {
					case ACK:
						overwrite = true
					case NAK:
						overwrite = false
					default:
						cli.tp.PrintfLine(CAN)
						break readlines
				}
				mode = modeSET
				state++ // should be 0 now
				continue readlines

			case MagicG: // GET key returns value or NUL
				numBy = utils.Str2int(split[1])
				if numBy == 0 {
					// abnormal: str2num failed parsing
					// or client send really a 0
					break readlines
				}
				mode = modeGET
				state++ // should be 0 now
				continue readlines

			case MagicD: // DEL key
				numBy = utils.Str2int(split[1])
				if numBy == 0 {
					// abnormal: str2num failed parsing
					// or client send really a 0
					break readlines
				}
				mode = modeDEL
				state++ // should be 0 now
				continue readlines

			case Magic1:
				// CAPTURE MEMORY PROFILE
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
					sock.logs.Info("Lock MemProfile run=(%d sec) wait=(%d sec)", runi, waiti)
					sock.mem.Lock()
					sock.logs.Info("StartMemProfile run=(%d sec) wait=(%d sec)", runi, waiti)
					run := time.Duration(runi) * time.Second
					wait := time.Duration(waiti) * time.Second
					Prof.StartMemProfile(run, wait)
					sock.mem.Unlock()
				}(runi, waiti)
				cli.tp.PrintfLine("200 StartMemProfile run=%d wait=%d", runi, waiti)

			case Magic2:
				 // CAPTURE CPU PROFILE
				if !socket {
					break readlines
				}
				sock.cpu.Lock()
				if sock.CPUfile != nil {
					Prof.StopCPUProfile()
					cli.tp.PrintfLine("200 StopCPUProfile")
					sock.CPUfile = nil
				} else {
					CPUfile, err := Prof.StartCPUProfile()
					if err != nil || CPUfile == nil {
						sock.logs.Info("ERROR SOCKET StartCPUProfile err='%v'", err)
						cli.tp.PrintfLine("400 ERR StartCPUProfile")
					} else {
						sock.CPUfile = CPUfile
						cli.tp.PrintfLine("200 StartCPUProfile")
					}
				}
				sock.cpu.Unlock()

			case MagicZ:
				// quit
				break readlines

			default:
				// unknown cmd
				break readlines

			} // end switch cmd
		} // end switch mode
		continue readlines
	} // end for readlines

	sock.logs.Info("SOCKET [cli=%d] LEFT conn rx=%d tx=%d", cli.id, recvbytes, sentbytes)
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
