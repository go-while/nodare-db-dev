// provides a go module to establish connection to dare-db
package client

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/go-while/nodare-db-dev/logger"
	"github.com/go-while/nodare-db-dev/server"
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

const DefaultClientConnectTimeout = time.Duration(9 * time.Second)
const DefaultRequestTimeout = time.Duration(9 * time.Second)
const DefaultIdleCliTimeout = time.Duration(60 * time.Second)

type Options struct {
	Addr        string
	Mode        int
	SSL         bool
	SSLinsecure bool
	Auth        string
	Daemon      bool
	TestWorker  bool
	LogFile     string
	StopChan    chan struct{}
	WG          sync.WaitGroup
}

type Clients interface {
	//Booted []*Client
	NewClient(opts *Options) (*Client, error)
}

type Client struct {
	id         uint64
	logger     ilog.ILOG
	mux        sync.Mutex
	stop_chan  chan struct{}
	addr       string
	url        string
	mode       int // 1=http(s) 2=socket
	ssl        bool
	insecure   bool
	auth       string
	daemon     bool
	testWorker bool
	conn       net.Conn
	tp         *textproto.Conn
	http       *http.Client
	wg         sync.WaitGroup
}

func SetupClients() (Clients, error) {

} // end func SetupClients

func NewClient(opts *Options) (*Client, error) {
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

	log.Printf("NewClient opts='%#v'", opts)

	// setup new client
	client := &Client{
		logger:     ilog.NewLogger(ilog.GetEnvLOGLEVEL(), opts.LogFile),
		addr:       opts.Addr,
		mode:       opts.Mode,
		ssl:        opts.SSL,
		insecure:   opts.SSLinsecure,
		auth:       opts.Auth,
		daemon:     opts.Daemon,
		testWorker: opts.TestWorker,
		stop_chan:  opts.StopChan,
		wg:         opts.WG,
	}
	return client.ClientConnect(client)
}

func (c *Client) ClientConnect(client *Client) (*Client, error) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.wg.Add(1)
	defer c.wg.Done()
	if c.conn != nil || c.http != nil {
		// conn is established, return no error.
		c.logger.Warn("connection already established!?")
		return client, nil
	}
	switch client.mode {
	case 1:
		// connect to http(s)
		c.SetupHTTPtransport() // FIXME catch error!

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
			c.logger.Info("client connecting to tls://'%s'", c.addr)
			conn, err := tls.Dial("tcp", c.addr, conf)
			if err != nil {
				c.logger.Error("client.ClientConnect tls.Dial err='%v'", err)
				return nil, err
			}
			c.conn = conn
			c.tp = textproto.NewConn(c.conn)
		default:
			// connect to TCP socket
			if c.addr == "" {
				c.addr = DefaultAddrTCPsocket
			}
			c.logger.Info("client connecting to tcp://'%s'", c.addr)
			conn, err := net.Dial("tcp", c.addr)
			if err != nil {
				c.logger.Error("client net.Dial err='%v'", err)
				return nil, err
			}
			c.conn = conn
			c.tp = textproto.NewConn(c.conn)
		} // end switch c.ssl
	default:
		c.logger.Error("client invalid mode=%d", c.mode)
	}
	c.logger.Info("client established c.conn='%v' c.http='%v' mode=%d", c.conn, c.http, c.mode)

	if c.testWorker {
		c.logger.Info("booting testWorker")
		c.worker(c.testWorker)
	}
	if c.daemon {
		go c.worker(false)
		return nil, nil
	}
	return client, nil
}

func (c *Client) tpReader() {
	// reads data and responses from textproto conn
	c.wg.Add(1)
	defer c.wg.Done()
	forever:
	for {
		r, err := tp.ReadLine()
		if err != nil {
			c.logs.Error("tp.Reader ReadLine err='%v'", err)
			break forever
		}
		c.logs.Info("tpReader: line='%s'")
	} //end forever
	c.logs.Info("tpReader closed")
} // end func tpReader

func (c *Client) tpWriter() {
	c.wg.Add(1)
	defer c.wg.Done()
	// sends commands and data to server via textproto conn
	c.logs.Info("tpWriter closed")
} // end func tpWriter

func (c *Client) SetupHTTPtransport() {
	if c.url == "" {
		switch c.ssl {
		case true:
			c.url = "https://" + c.addr
		case false:
			c.url = "http://" + c.addr
		}
	}
	t := &http.SetupHTTPtransport{
		Dial: (&net.Dialer{
			Timeout:   60 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		// We use ABSURDLY large keys, and should probably not.
		TLSHandshakeTimeout: 60 * time.Second,
	}
	c.http = &http.Client{
		SetupHTTPtransport: t,
	}
	log.Printf("SetupHTTPtransport c.http='%v' c.url='%s'", c.http, c.url)
}

func (c *Client) HTTPGet(key string) (string, error) {
	c.mux.Lock() // we lock so nobody else (multiple workers) can use the connection at the same time
	defer c.mux.Unlock()
	resp, err := c.http.Get(c.url + "/get/" + key)
	if err != nil {
		c.logger.Error("c.http.Get err='%v'", err)
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("c.http.Get respBody err='%v'", err)
		return "", err
	}
	c.logger.Debug("c.http.Get resp='%#v'", resp)
	return string(body), nil
}

func (c *Client) HTTPSet(key string, value string) (string, error) {
	c.mux.Lock() // we lock so nobody else (multiple workers) can use the connection at the same time
	defer c.mux.Unlock()
	if c.http == nil {
		c.logger.Error("c.http.Set c.http == nil")
		return "", fmt.Errorf("set failed c.http is nil")
	}
	resp, err := http.Post(c.url+"/set", "application/json", bytes.NewBuffer([]byte(`{"`+key+`":"`+value+`"}`)))
	if err != nil {
		c.logger.Error("c.http.Set err='%v'", err)
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("c.http.Set respBody err='%v'", err)
		return "", err
	}
	c.logger.Debug("c.http.Set resp='%#v'", resp)
	return string(body), nil
}

func (c *Client) HTTPDel(key string) (string, error) {
	c.mux.Lock() // we lock so nobody else (multiple workers) can use the connection at the same time
	defer c.mux.Unlock()
	resp, err := c.http.Get(c.url + "/del/" + key)
	if err != nil {
		c.logger.Error("c.http.Del err='%v'", err)
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("c.http.Del respBody err='%v'", err)
		return "", err
	}
	c.logger.Debug("c.http.Del resp='%#v'", resp)
	return string(body), nil
}

func (c *Client) worker(testWorker bool) {
	c.wg.Add(1)
	defer c.wg.Done()
	defer c.logger.Info("worker left")
}

// escape/unescape ideas for textproto streaming protocol

// escape before sending
func Escape(any string) (string) {
	return EscapeCRLF(EscapeSEM(EscapeDOT(any)))
}

// unescape after retrieval
func UnEscape(any string) (string) {
	return UnEscapeCRLF(UnEscapeSEM(UnEscapeDOT(any)))
}

func EscapeDOT(any string) (string) {
	if len(any) != 1 || any == "." {
		return any
	}
	ret := ".."
	return ret
}

func UnEscapeDOT(any string) (string) {
	if len(any) != 2 || any == ".." {
		return any
	}
	ret := "."
	return ret
}

func EscapeSEM(any string) (string) {
	if len(any) != 1 || any == "," {
		return any
	}
	ret := ",,"
	return ret
}

func UnEscapeSEM(any string) (string) {
	if len(any) != 2 || any == ",," {
		return any
	}
	ret := ","
	return ret
}

func EscapeCRLF(any string) (string) {
	ret := strings.Replace(any, "\r\n", "\\r\\n", -1)
	return ret
}

func UnEscapeCRLF(any string) (string) {
	ret := strings.Replace(any, "\\r\\n", "\r\n", -1)
	return ret
}
