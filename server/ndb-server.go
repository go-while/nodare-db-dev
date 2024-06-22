package server

import (
	"encoding/json"
	"github.com/go-while/nodare-db-dev/database"
	"github.com/go-while/nodare-db-dev/logger"
	"github.com/gorilla/mux"
	"net"
	"net/http"
	"strings"
)

type WebMux interface {
	CreateMux(cfg VConfig) *mux.Router
	HandlerGetValByKey(w http.ResponseWriter, r *http.Request)
	HandlerSet(w http.ResponseWriter, r *http.Request)
	HandlerDel(w http.ResponseWriter, r *http.Request)
}

type XNDBServer struct {
	dbs  *database.XDBS
	logs ilog.ILOG
	acl  *AccessControlList
}

func NewXNDBServer(dbs *database.XDBS, logs ilog.ILOG) *XNDBServer {
	return &XNDBServer{
		dbs:   dbs,
		logs: logs,
		acl: NewACL(),
	}
}

func (srv *XNDBServer) CreateMux(cfg VConfig) *mux.Router {
	r := mux.NewRouter()
	//r.HandleFunc("/jkv/{"+KEY_PARAM+"}", srv.HandlerGetJsonBlobByKey)
	//r.HandleFunc("/jnv/{"+KEY_PARAM+"}", srv.HandlerGetJsonValByKey)
	//r.HandleFunc("/zip/{"+KEY_PARAM+"}", srv.HandlerCompress)
	r.HandleFunc("/get/{"+KEY_PARAM+"}", srv.HandlerGetValByKey)
	r.HandleFunc("/get/{"+KEY_PARAM+"}/{"+DB_PARAM+"}", srv.HandlerGetValByKey)
	r.HandleFunc("/del/{"+KEY_PARAM+"}", srv.HandlerDel)
	r.HandleFunc("/del/{"+KEY_PARAM+"}/{"+DB_PARAM+"}", srv.HandlerDel)
	r.HandleFunc("/set", srv.HandlerSet)
	r.HandleFunc("/set/{"+DB_PARAM+"}", srv.HandlerSet)

	iplist := cfg.GetString(VK_SERVER_WEB_ACL)
	if iplist != "" {
		ips := strings.Split(iplist, ",")
		for _, ip := range ips {
			srv.logs.Info("WEB ACL allow IP: %s", ip)
			srv.acl.SetACL(ip, true)
		}
	}
	return r
}

func (srv *XNDBServer) HandlerGetValByKey(w http.ResponseWriter, r *http.Request) {
	nilheader(w)
	if !srv.acl.checkACL_IP(GetIpAddress(r)) {
		w.WriteHeader(http.StatusForbidden) // 403
		srv.logs.Warn("web /get Forbidden by ACL ip='%s'", GetIpAddress(r))
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		srv.logs.Warn("web /get method not allowed: ip='%s'", GetIpAddress(r))
		return
	}

	vars := mux.Vars(r)
	key := vars[KEY_PARAM]
	if key == "" {
		w.WriteHeader(http.StatusNotAcceptable) // 406
		return
	}

	xdb := DEFAULT_DB
	if vars[DB_PARAM] != "" {
		xdb = vars[DB_PARAM]
	}
	db := srv.dbs.GetDB(xdb, false)
	if db == nil {
		srv.logs.Error("HandlerGetValByKey DB NIL ident='%s' key='%s'", xdb, key)
		w.WriteHeader(http.StatusGone) // 410
		return
	}

	var val string
	found := db.Get(key, &val)
	if !found {
		srv.logs.Debug("HandlerGetValByKey not found key='%s'", key)
		w.WriteHeader(http.StatusGone) // 410
		return
	}

	// response as json with KEY:VAL ??
	/*
		response, err := json.Marshal(map[string]interface{}{key: val})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(response)
	*/

	// response as json with VAL only ?
	/*
		response, err := json.Marshal([]interface{}{val})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(response)
	*/

	// response as raw plain text with VAL only
	//w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(val))
	//srv.logs.Debug("web /get db='%s' key='%s' val='%s' ip='%s'", xdb, key, val, GetIpAddress(r))
} // end func HandlerGetValByKey

func (srv *XNDBServer) HandlerSet(w http.ResponseWriter, r *http.Request) {
	nilheader(w)

	if !srv.acl.checkACL_IP(GetIpAddress(r)) {
		w.WriteHeader(http.StatusForbidden) // 403
		srv.logs.Warn("web /set Forbidden by ACL ip='%s'", GetIpAddress(r))
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		srv.logs.Warn("web /set method not allowed: ip='%s'", GetIpAddress(r))
		return
	}


	vars := mux.Vars(r)
	xdb := DEFAULT_DB
	if vars[DB_PARAM] != "" {
		xdb = vars[DB_PARAM]
	}
	db := srv.dbs.GetDB(xdb, true)
	if db == nil {
		srv.logs.Error("HandlerSet DB NIL ident='%s'", xdb)
		w.WriteHeader(http.StatusGone) // 410
		return
	}

	// FIXME DECODE JSON
	var data map[string]string
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable) // 406
		return
	}

	for key, value := range data {
		ok := db.Set(key, value, true) // default always overwrites
		if !ok {
			srv.logs.Warn("HandlerSet err='%v'", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusCreated)
	//srv.logs.Debug("web /set db='%s' data='%#v' ip='%s'", xdb, data, GetIpAddress(r))
} // end func HandlerSet

func (srv *XNDBServer) HandlerDel(w http.ResponseWriter, r *http.Request) {
	nilheader(w)

	if !srv.acl.checkACL_IP(GetIpAddress(r)) {
		w.WriteHeader(http.StatusForbidden) // 403
		srv.logs.Warn("web /del Forbidden by ACL ip='%s'", GetIpAddress(r))
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		srv.logs.Warn("web /del method not allowed: ip='%s'", GetIpAddress(r))
		return
	}

	vars := mux.Vars(r)
	key := vars[KEY_PARAM]
	if key == "" {
		w.WriteHeader(http.StatusNotAcceptable) // 406
		return
	}
	//srv.logs.Debug("HandlerDel key='%s'", key)

	xdb := DEFAULT_DB
	if vars[DB_PARAM] != "" {
		xdb = vars[DB_PARAM]
	}
	db := srv.dbs.GetDB(xdb, false)
	if db == nil {
		srv.logs.Error("HandlerDel DB NIL ident='%s' key='%s'", xdb, key)
		w.WriteHeader(http.StatusGone) // 410
		return
	}

	ok := db.Del(key)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	//srv.logs.Debug("web /del db='%s' key='%s' ip='%s'", xdb, key, GetIpAddress(r))
} // end func HandlerDel

func (srv *XNDBServer) SetLogLvl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	key := r.PathValue(KEY_PARAM)
	if key == "" {
		w.WriteHeader(http.StatusNotAcceptable) // 406
		return
	}
	// TODO test
	srv.logs.SetLOGLEVEL(ilog.GetLOGLEVEL(key))
	return
}

func nilheader(w http.ResponseWriter) {
	w.Header()["Date"] = nil
	w.Header()["Content-Type"] = nil
	w.Header()["Content-Length"] = nil
	w.Header()["X-Content-Type-Options"] = nil
	w.Header()["Transfer-Encoding"] = nil
}

func GetIpAddress(r *http.Request) (host string) {
	host, _, _ = net.SplitHostPort(r.RemoteAddr)
	return
}

func GetXforwarded(r *http.Request) (xf string) {
	xf = r.Header.Get("X-Forwarded-For")
	return
}
