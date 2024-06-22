package server

import (
	"github.com/go-while/go-cpu-mem-profiler"
	"github.com/go-while/nodare-db-dev/logger"
)

var Prof *prof.Profiler

const (

	DEFAULT_SUB_DICKS = 10

	DEFAULT_PW_LEN = 32 // admin/username:password
	DEFAULT_SUPERADMIN = "superadmin"

	DEFAULT_CONFIG_FILE = "config.toml"
	DATA_DIR = "dat"
	CONFIG_DIR = "cfg"

	DEFAULT_LOGS_FILE = "ndb.log"
	DEFAULT_LOGLEVEL_STR = "INFO"
	DEFAULT_LOGLEVEL_INT = ilog.INFO

	DEFAULT_SERVER_ADDR = "[::1]"
	DEFAULT_SERVER_TCP_PORT = "2420"
	DEFAULT_SERVER_UDP_PORT = "2240"
	DEFAULT_SERVER_SOCKET_PATH = "/tmp/ndb.socket"
	DEFAULT_SERVER_SOCKET_TCP_PORT = "3420"
	DEFAULT_SERVER_SOCKET_TLS_PORT = "4420"

	DEFAULT_TLS_PRIVKEY = "privkey.pem"
	DEFAULT_TLS_PUBCERT = "fullchain.pem"

	// WEB ROUTER DATABASE FLAGS
	KEY_PARAM = "key"
	DB_PARAM = "db"
	DEFAULT_DB = "0"

	// readline flags
	no_mode = 0x00
	modeADD = 0x11
	modeGET = 0x22
	modeSET = 0x33
	modeDEL = 0x44
	CaseAdded = 0x69
	CaseDupes = 0xB8
	CaseDeleted = 0x00
	CasePass = 0xFF
	FlagSearch = 0x42

	// client proto flags
	Magic1 = "1" // mem-prof
	Magic2 = "2" // cpu-prof
	MagicA = "A" // add
	MagicD = "D" // del
	MagicG = "G" // get
	MagicL = "L" // list
	MagicS = "S" // set
	MagicZ = "Z" // quit

	// socket proto flags
	KEY_LIMIT = 1024 * 1024 * 1024 // respond: CAN
	VAL_LIMIT = 1024 * 1024 * 1024 // respond: CAN
	//EmptyStr = ""
	CR = "\r"
	LF = "\n"
	CRLF = CR + LF
	DOT = "."
	COM = ","
	SEM = ";"

	// ASCII control characters
	// [hex: 0 - 1F] // [DEC character code 0-31]
	NUL = string(0x00) // Null character 		// 0
	SOH = string(0x01) // Start of Heading 	// 1
	STX = string(0x02) // Start of Text 		// 2
	ETX = string(0x03) // End of Text 		// 3
	EOT = string(0x04) // End of Transmission // 4
	ENQ = string(0x05) // Enquiry 			// 5
	ACK = string(0x06) // Acknowledge 		// 6
	BEL = string(0x07) // Bell, Alert 		// 7
	NAK = string(0x15) // Negative Ack		// 21
	SYN = string(0x16) // Synchronous Idle	// 22
	ETB = string(0x17) // End of Trans. Block // 23
	CAN = string(0x18) // Cancel 				// 24
	EOM = string(0x19) // End of medium 		// 25
	SUB = string(0x20) // Substitute  		// 26
	ESC = string(0x1B) // Escape 				// 27

	// VIPER CONFIG DEFAULTS

	V_DEFAULT_SUB_DICKS = "10"
	V_DEFAULT_TLS_ENABLED = false
	V_DEFAULT_NET_WEBSRV_READ_TIMEOUT = 5
	V_DEFAULT_NET_WEBSRV_WRITE_TIMEOUT = 10
	V_DEFAULT_NET_WEBSRV_IDLE_TIMEOUT = 120
	V_DEFAULT_SERVER_SOCKET_ACL = "127.0.0.1,::1"
	V_DEFAULT_SERVER_WEB_ACL = "127.0.0.1,::1"

	// VIPER CONFIG KEYS
	VK_ACCESS_SUPERADMIN_USER = "server.superadmin_user"
	VK_ACCESS_SUPERADMIN_PASS = "server.superadmin_pass"

	VK_LOG_LOGLEVEL = "log.loglevel"
	VK_LOG_LOGFILE = "log.logfile"

	VK_SETTINGS_BASE_DIR = "settings.base_dir"
	VK_SETTINGS_DATA_DIR = "settings.data_dir"
	VK_SETTINGS_SETTINGS_DIR = "settings.settings_dir"
	VK_SETTINGS_SUB_DICKS = "settings.sub_dicks"

	VK_SEC_TLS_ENABLED = "security.tls_enabled"
	VK_SEC_TLS_PRIVKEY = "security.tls_priv_key"
	VK_SEC_TLS_PUBCERT = "security.tls_pub_cert"

	VK_NET_WEBSRV_READ_TIMEOUT = "network.websrv_read_timeout"
	VK_NET_WEBSRV_WRITE_TIMEOUT = "network.websrv_write_timeout"
	VK_NET_WEBSRV_IDLE_TIMEOUT = "network.websrv_idle_timeout"

	VK_SERVER_HOST = "server.bindip"
	VK_SERVER_PORT_TCP = "server.port"
	VK_SERVER_PORT_UDP = "server.port_udp"
	VK_SERVER_SOCKET_PATH = "server.socket_path"
	VK_SERVER_SOCKET_PORT_TCP = "server.socket_tcpport"
	VK_SERVER_SOCKET_PORT_TLS = "server.socket_tlsport"
	VK_SERVER_SOCKET_ACL = "server.socket_acl"
	VK_SERVER_WEB_ACL = "server.web_acl"


	// ENV KEYS
	ENVK_LOGLEVEL = "LOGLEVEL"
	ENVK_LOGSFILE = "LOGSFILE"

	ENVK_NDB_BASE_DIR = "NDB_BASE_DIR"
	ENVK_NDB_DATA_DIR = "NDB_DATA_DIR"
	ENVK_NDB_CONFIG_DIR = "NDB_CONFIG_DIR"
	ENVK_NDB_SUB_DICKS = "NDB_SUB_DICKS"

	ENVK_NDB_TLS_ENABLED = "NDB_TLS_ENABLED"
	ENVK_NDB_TLS_PRIVKEY = "NDB_TLS_PRIVKEY"
	ENVK_NDB_TLS_PUBCERT = "NDB_TLS_PUBCERT"

	ENVK_NDB_WEBSRV_READ_TIMEOUT = "NDB_WEBSRV_READ_TIMEOUT"
	ENVK_NDB_WEBSRV_WRITE_TIMEOUT = "NDB_WEBSRV_WRITE_TIMEOUT"
	ENVK_NDB_WEBSRV_IDLE_TIMEOUT = "NDB_WEBSRV_IDLE_TIMEOUT"

	ENVK_NDB_HOST = "NDB_HOST"
	ENVK_NDB_PORT = "NDB_PORT"
	ENVK_NDB_SERVER_UDP_PORT = "NDB_SERVER_UDP_PORT"
	ENVK_NDB_SERVER_SOCKET_PATH = "NDB_SERVER_SOCKET_PATH"
	ENVK_NDB_SERVER_SOCKET_TCP_PORT = "NDB_SERVER_SOCKET_TCP_PORT"
	ENVK_NDB_SERVER_SOCKET_TLS_PORT = "NDB_SERVER_SOCKET_TLS_PORT"
	ENVK_NDB_SERVER_SOCKET_ACL = "NDB_SERVER_SOCKET_ACL"
	ENVK_NDB_SERVER_WEB_ACL = "NDB_SERVER_WEB_ACL"
) // end const

