// provides a go module to establish connection to dare-db
package client

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/go-while/nodare-db-dev/logger"
	"github.com/go-while/nodare-db-dev/server"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/textproto"
	"strings"
	"sync"
	"time"
)

const DefaultAddr = "localhost:2420"
const DefaultAddrSSL = "localhost:2420"

const DefaultAddrTCPsocket = "localhost:3420"
const DefaultAddrTLSsocket = "localhost:4420"

const DefaultCliConnectTimeout = time.Duration(9 * time.Second)
const DefaultRequestTimeout = time.Duration(9 * time.Second)
const DefaultIdleCliTimeout = time.Duration(60 * time.Second)

type Options struct {
	Addr        string
	Mode        int
	SSL         bool
	SSLinsecure bool
	Auth        string
	Daemon      bool
	RunTest  bool
	LogFile     string
	StopChan    chan struct{}
	WG          sync.WaitGroup
	Logs        ilog.ILOG
}

type CliHandler struct{
	id int
	slots int
	Clients []*Client
	mux sync.RWMutex
	logs ilog.ILOG
}

type Client struct {
	Mode       int // 1=http(s) 2=socket
	wg         sync.WaitGroup
	mux        sync.Mutex
	logs       ilog.ILOG
	stop_chan  chan struct{}
	id         int
	addr       string
	url        string
	ssl        bool
	insecure   bool
	auth       string
	daemon     bool
	runtest    bool
	http       *http.Client
	sock       net.Conn
	tp         *textproto.Conn
}

func NewCliHandler(logs ilog.ILOG) (cli *CliHandler) {
	logs.Debug("Client.NewCliHandler")
	return &CliHandler{
		logs: logs,
		// Clients [0] is not used: we count ids from 1!
		// beware of the off-by-one error
		// slice expands x2 when full
		Clients: make([]*Client, 2),
		slots: 1,
	}
} // end func NewCliHandler

func (cliH *CliHandler) NewCli(opts *Options) (*Client, error) {
	cliH.mux.Lock()
	defer cliH.mux.Unlock()
	switch opts.Addr {
		case "":
			// no addr:port supplied
			switch opts.Mode {
				case 1:
					// wants http(s)
					switch opts.SSL {
						case true:
							opts.Addr = DefaultAddrSSL
						default:
							opts.Addr = DefaultAddr
					}
				case 2:
					// wants socket
					switch opts.SSL {
						case true:
							opts.Addr = DefaultAddrTLSsocket
						default:
							opts.Addr = DefaultAddrTCPsocket
					} // end switch SSL
			} // end switch Mode
	} // end switch Addr

	cliH.logs.Info("NewCli opts='%#v'", opts)
	// setup new client
	client := &Client{
		Mode:       opts.Mode,
		addr:       opts.Addr,
		ssl:        opts.SSL,
		insecure:   opts.SSLinsecure,
		auth:       opts.Auth,
		daemon:     opts.Daemon,
		runtest:    opts.RunTest,
		stop_chan:  opts.StopChan,
		wg:         opts.WG,
		logs:       cliH.logs,
	}
	cliconn, err := client.CliConnect(client)
	if err != nil {
		cliH.logs.Error("client.CliConnect addr='%s' failed err='%v'", err)
		return nil, err
	}
	cliH.id++
	cliconn.id = cliH.id
	cliH.logs.Info("NewCli id=%d", cliH.id)
	cliH.expandSlice()
	cliH.Clients[cliH.id] = cliconn
	return cliconn, nil
} // end func NewCli

func (c *Client) CliConnect(client *Client) (*Client, error) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.wg.Add(1)
	defer c.wg.Done()
	if c.sock != nil || c.http != nil {
		// conn is established, return no error.
		c.logs.Warn("connection already established!?")
		return client, nil
	}
	switch client.Mode {
	case 1:
		// connect to http(s)
		c.Transport() // FIXME catch error!

	case 2:
		// connect to sockets
		switch c.ssl {
		case true:
			// connect to TLS socket
			if c.addr == "" {
				c.addr = DefaultAddrTLSsocket
			}
			conf := &tls.Config{
				InsecureSkipVerify: c.insecure,
				MinVersion:         tls.VersionTLS12,
				CurvePreferences: []tls.CurveID{
					tls.CurveP521,
					tls.CurveP384,
					tls.CurveP256},
				PreferServerCipherSuites: true,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				},
			}
			c.logs.Info("client connecting to tls://'%s'", c.addr)
			conn, err := tls.Dial("tcp", c.addr, conf)
			if err != nil {
				c.logs.Error("client.CliConnect tls.Dial err='%v'", err)
				return nil, err
			}
			c.sock = conn
			c.tp = textproto.NewConn(c.sock)
		default:
			// connect to TCP socket
			if c.addr == "" {
				c.addr = DefaultAddrTCPsocket
			}
			c.logs.Info("client connecting to tcp://'%s'", c.addr)
			conn, err := net.Dial("tcp", c.addr)
			if err != nil {
				c.logs.Error("client net.Dial err='%v'", err)
				return nil, err
			}
			c.sock = conn
			c.tp = textproto.NewConn(c.sock)
		} // end switch c.ssl
	default:
		c.logs.Error("client invalid mode=%d", c.Mode)
	}
	c.logs.Info("client established c.sock='%v' c.http='%v' mode=%d", c.sock, c.http, c.Mode)
	if c.tp != nil {
		_, _, err := c.tp.ReadCodeLine(200) // server.ACK welcome message
		if err != nil {
			c.logs.Error("c.tp.ReadCodeLine init err='%v'", err)
			return nil, err
		}
		//go c.tpReader()
		//go c.tpWriter()
	}

	if c.runtest {
		c.logs.Warn("booting internal runtest not implemented") // TODO!
		c.worker(c.runtest)
	}
	if c.daemon {
		go c.worker(c.runtest)
		return nil, nil
	}

	return client, nil
} // end func CliConnect

func (c *Client) tpReader() {
	// reads data and responses from textproto conn
	c.wg.Add(1)
	defer c.wg.Done()
	forever:
	for {
		r, err := c.tp.ReadLine() // lines from server
		if err != nil {
			c.logs.Error("tp.Reader ReadLine err='%v'", err)
			break forever
		}
		c.logs.Info("tpReader: line='%s'=%d", r, len(r))
	} //end forever
	c.logs.Info("tpReader closed")
} // end func tpReader

func (c *Client) tpWriter() {
	c.wg.Add(1)
	defer c.wg.Done()
	// sends commands and data to server via textproto conn
	c.logs.Info("tpWriter closed")
} // end func tpWriter

func (c *Client) Transport() {
	if c.url == "" {
		switch c.ssl {
		case true:
			c.url = "https://" + c.addr
		case false:
			c.url = "http://" + c.addr
		}
	}
	t := &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   60 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		// We use ABSURDLY large keys, and should probably not.
		TLSHandshakeTimeout: 60 * time.Second,
	}
	c.http = &http.Client{
		Transport: t,
	}
	log.Printf("Transport c.http='%v' c.url='%s'", c.http, c.url)
}


func (c *Client) SOCK_Set(key string, val string, resp *string) (err error) {
	if c.tp == nil {
		err = fmt.Errorf("ERROR SOCK_Set c.tp nil")
		return
	}

	// 	SET|1\r\n
	//		AveryLooongKey11\r\n
	//		AveryLongValue\r\n
	//		\x17\r\n

	request := server.MagicS+"|1"+server.CRLF+key+server.CRLF+val+server.CRLF+server.ETB+server.CRLF
	c.logs.Debug("SOCK_Set k='%v' v='%v' request='%#v'", key, val, request)
	_, err = io.WriteString(c.sock, request)
	if err != nil {
		return
	}
	c.logs.Debug("SOCK_Set k='%v' v='%v' wait ReadLine", key, val)
	reply, err := c.tp.ReadLine()
	if err != nil {
		return
	}
	if len(reply) == 0 {
		err = fmt.Errorf("SOCK_Set empty reply")
		return
	}
	if string(reply[0]) != server.ACK {
		c.logs.Error("SOCK_Set !reply.ACK k='%v' v='%v' reply='%#v'", key, val, reply)
	}
	c.logs.Debug("SOCK_Set k='%v' v='%v' got ReadLine reply='%#v'", key, val, reply)
	*resp = reply
	return
} // end func SOCK_Set

func (c *Client) SOCK_Get(key string, resp *string, nfk *string) (found bool, err error) {
	if c.tp == nil {
		err = fmt.Errorf("ERROR SOCK_Get c.tp nil")
		return
	}
	c.logs.Debug("SOCK_Get key='%s'", key)

	//	GET|1\r\n
	// 		AveryLooongKey\r\n
	//		\x17\r\n

	request := server.MagicG+"|1"+server.CRLF+key+server.CRLF+server.ETB+server.CRLF
	_, err = io.WriteString(c.sock, request)
	if err != nil {
		return
	}

	reply, err := c.tp.ReadLine()
	if err != nil {
		return
	}
	log.Printf("SOCK_GET key='%s' reply='%#v'", key, reply)

	switch string(reply[0]) {
		case server.NUL:
		c.logs.Warn("SOCK_GET key='%s' NUL")
		// not found
		if nfk != nil && len(reply) > 1 {
			// extract the not-found-key (only with multiple requests)
			*nfk = string(reply[1:])
			return
		}
	}
	found, *resp = true, reply
	return
} // end func SOCK_Get

func (c *Client) SOCK_Del(key string, resp *string, nfk *string) (err error) {
	if c.tp == nil {
		err = fmt.Errorf("ERROR SOCK_Get c.tp nil")
		return
	}

	//	DEL|1\r\n
	// 	AveryLooongKey\r\n
	//	\x17\r\n

	request := server.MagicD+"|1"+server.CRLF+key+server.CRLF+server.ETB+server.CRLF
	_, err = io.WriteString(c.sock, request)
	if err != nil {
		return
	}

	reply, err := c.tp.ReadLine()
	if err != nil {
		return
	}

	switch string(reply[0]) {
		case server.NUL:
		// not found
		if nfk != nil && len(reply) > 1 {
			// extract the not-found-key (only with multiple requests)
			*nfk = string(reply[1:])
			return
		}
	}

	*resp = reply
	return
} // end func SOCK_Del

func (c *Client) SOCK_GetMany(keys *[]string) (err error) {
	if c.tp == nil {
		err = fmt.Errorf("ERROR SOCK_GetMany c.tp nil")
		return
	}
	return
} // end func SOCK_GetMany

func Construct_SOCK_GetMany(key []string) {

} // end func Construct_SOCK_GetMany

func (c *Client) HTTP_Get(key string, val *string) (error) {
	c.mux.Lock() // we lock so nobody else (multiple workers) can use the connection at the same time
	defer c.mux.Unlock()
	resp, err := c.http.Get(c.url + "/get/" + key)
	if err != nil {
		c.logs.Error("c.http.Get err='%v'", err)
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.logs.Error("c.http.Get respBody err='%v'", err)
		return err
	}
	c.logs.Debug("c.http.Get resp='%#v'", resp)
	*val = string(body)
	return nil
} // end func HTTP_Get

func (c *Client) HTTP_Set(key string, value string, resp *string) (err error) {
	c.mux.Lock() // we lock so nobody else (multiple workers) can use the connection at the same time
	defer c.mux.Unlock()
	if c.http == nil {
		err = fmt.Errorf("c.http.Set failed c.http nil")
		c.logs.Error("%s",err)
		return
	}
	rresp, rerr := http.Post(c.url+"/set", "application/json", bytes.NewBuffer([]byte(`{"`+key+`":"`+value+`"}`)))
	if rerr != nil {
		c.logs.Error("c.http.Post Set err='%v'", rerr)
		err = rerr
		return
	}

	defer rresp.Body.Close()
	body, err := ioutil.ReadAll(rresp.Body)
	if err != nil {
		c.logs.Error("c.http.Set rrespBody err='%v'", err)
		return
	}
	c.logs.Debug("c.http.Set rresp='%#v'", rresp)
	*resp = string(body)
	return
} // end func HTTP_Set

func (c *Client) HTTP_Del(key string, resp *string) (err error) {
	c.mux.Lock() // we lock so nobody else (multiple workers) can use the connection at the same time
	defer c.mux.Unlock()
	rresp, err := c.http.Get(c.url + "/del/" + key)
	if err != nil {
		c.logs.Error("c.http.Del err='%v'", err)
		return
	}
	defer rresp.Body.Close()
	body, err := ioutil.ReadAll(rresp.Body)
	if err != nil {
		c.logs.Error("c.http.Del respBody err='%v'", err)
		return
	}
	*resp = string(body)
	c.logs.Debug("c.http.Del resp='%#v'", resp)
	return
} // end func HTTP_Del

func (c *Client) worker(runtest bool) {
	c.wg.Add(1)
	defer c.wg.Done()
	defer c.logs.Info("worker left runtest=%t", runtest)
} // end func worker

func (cliH *CliHandler) expandSlice() {
	//cliH.logs.Debug("cliH expandSlice? maxclients=%d slots=%d cliH.id=%d", cap(cliH.Clients)-1, cliH.slots, cliH.id)
	if cliH.slots > cliH.id { // beware of the off-by-one error
		//cliH.logs.Debug("cliH not expandSlice")
		return
	}
	newslots := cliH.slots*2
	new := make([]*Client, newslots)

	for i, cli := range cliH.Clients {
		if cli == nil {
			continue
		}
		new[i] = cli // copy value to new slice
		cliH.Clients[i] = nil // nil value in old slice
	} // end for range cliH.Clients

	cliH.Clients = nil // nil the slice content
	cliH.Clients = new // set new slice
	cliH.slots = newslots
	cliH.logs.Info("cliH expandSlice! slots=%d cap=%d id=%d newSlice=%d", cliH.slots, cap(cliH.Clients), cliH.id, len(new))
} // end func expandSlice

/*
 * escape/unescape ideas for textproto streaming protocol
 *
 */

// escape before sending
func Escape(any string) (string) {
	return EscapeCR(EscapeLF(any))
}

// unescape after retrieval
func UnEscape(any string) (string) {
	return UnEscapeCR(UnEscapeLF(any))
}

func EscapeCR(any string) (string) {
	ret := strings.Replace(any, "\r", "\\r", -1)
	return ret
}

func UnEscapeCR(any string) (string) {
	ret := strings.Replace(any, "\\r", "\r", -1)
	return ret
}

func EscapeLF(any string) (string) {
	ret := strings.Replace(any, "\n", "\\n", -1)
	return ret
}

func UnEscapeLF(any string) (string) {
	ret := strings.Replace(any, "\\n", "\n", -1)
	return ret
}
