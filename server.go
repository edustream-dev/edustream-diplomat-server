package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)

var db *sql.DB
var logger *log.Logger

func main() {
	file, err := os.OpenFile(fmt.Sprintf("logfile-%s.txt", time.Now().Format(time.RFC3339)), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0755)
	if err != nil {
		log.Fatalf("Error trying to open file for logging! %s\n", err.Error())
	}

	logger = log.New(file, "", log.Ldate|log.Ltime)

	load()

	go manageServers()

	r := mux.NewRouter()
	r.HandleFunc("/announce/", announce)
	r.PathPrefix("/ingest/").Handler(&IngestServer{})
	r.PathPrefix("/stream/").Handler(&OutgestServer{})
	r.PathPrefix("/").Handler(&OtherServer{})
	http.ListenAndServeTLS(":443", "public.crt", "private.key", r)
}

func load() {
	loadDatabase()

	servers = make([]string, 0)
	waitingServers = make([]string, 0)
	streamServers = make([]*StreamServer, 0)

	serverLock = new(sync.Mutex)
	waitingServerLock = new(sync.Mutex)

	robinManager = 0
	robinLocker = &sync.Mutex{}
}

func announce(w http.ResponseWriter, r *http.Request) {
	password, err := ioutil.ReadAll(r.Body)

	query := r.URL.Query()

	if query["url"] == nil {
		fhandle(w, "Incorrect parameters given!", nil)
		return
	}

	if err != nil {
		fhandle(w, "Could not read body from announce request!", err)
		return
	}

	if string(password) != "edustream-diplomat-server" {
		fhandle(w, "Incorrect password to announce to diplomat server!", nil)
		return
	}

	serverLock.Lock()
	waitingServers = append(waitingServers, query["url"][0])
	serverLock.Unlock()
}
