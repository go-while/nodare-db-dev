package server

import (
	"encoding/json"
	"github.com/go-while/nodare-db-dev/database"
	"github.com/go-while/nodare-db-dev/logger"
	"github.com/gorilla/mux"
	"net/http"
)

const KEY_PARAM = "key"

type WebMux interface {
	CreateMux() *mux.Router
	HandlerGetValByKey(w http.ResponseWriter, r *http.Request)
	HandlerSet(w http.ResponseWriter, r *http.Request)
	HandlerDel(w http.ResponseWriter, r *http.Request)
}

type XNDBServer struct {
	db   *database.XDatabase
	logs ilog.ILOG
}

func NewXNDBServer(db *database.XDatabase, logs ilog.ILOG) *XNDBServer {
	return &XNDBServer{
		db:   db,
		logs: logs,
	}
}

func (srv *XNDBServer) CreateMux() *mux.Router {
	r := mux.NewRouter()
	//r.HandleFunc("/jkv/{"+KEY_PARAM+"}", srv.HandlerGetJsonBlobByKey)
	//r.HandleFunc("/jnv/{"+KEY_PARAM+"}", srv.HandlerGetJsonValByKey)
	//r.HandleFunc("/zip/{"+KEY_PARAM+"}", srv.HandlerCompress)
	r.HandleFunc("/get/{"+KEY_PARAM+"}", srv.HandlerGetValByKey)
	r.HandleFunc("/del/{"+KEY_PARAM+"}", srv.HandlerDel)
	r.HandleFunc("/set", srv.HandlerSet)
	return r
}

func (srv *XNDBServer) HandlerGetValByKey(w http.ResponseWriter, r *http.Request) {
	nilheader(w)

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		srv.logs.Warn("server /get/ method not allowed ")
		return
	}

	vars := mux.Vars(r)
	key := vars[KEY_PARAM]

	if key == "" {
		w.WriteHeader(http.StatusNotAcceptable) // 406
		return
	}

	val := srv.db.Get(key)
	if val == nil {
		srv.logs.Info("not found key='%s'", key)
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
	w.Write([]byte(val.(string)))
}

func (srv *XNDBServer) HandlerSet(w http.ResponseWriter, r *http.Request) {
	nilheader(w)

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var data map[string]interface{}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable) // 406
		return
	}

	for key, value := range data {
		err = srv.db.Set(key, value)
		if err != nil {
			srv.logs.Warn("HandlerSet err='%v'", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)

}

func (srv *XNDBServer) HandlerDel(w http.ResponseWriter, r *http.Request) {
	nilheader(w)
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	key := vars[KEY_PARAM]
	if key == "" {
		w.WriteHeader(http.StatusNotAcceptable) // 406
		return
	}

	err := srv.db.Del(key)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

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
