package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

var servers []string
var serverLock *sync.Mutex
var waitingServers []string
var waitingServerLock *sync.Mutex

var robinManager int
var robinLocker *sync.Mutex

// StreamServer : Struct to associate a stream to a server
type StreamServer struct {
	URL      string
	cameraID string
}

var streamServers []*StreamServer

func manageServers() {
	client := new(http.Client)

	for {
		timer := time.After(5 * time.Second)

		healthyServers := make([]string, 0)

		serverLock.Lock()
		waitingServerLock.Lock()
		for _, server := range append(servers, waitingServers...) {
			response, err := client.Get(fmt.Sprintf("http://%s/", server))

			if err != nil {
				handle("Error trying to get health of server!", err)
			}

			if response.StatusCode != 200 {
				logger.Println("Found server not healthy!")
				continue
			}

			healthyServers = append(healthyServers, server)
		}

		updatedStreamServers := make([]*StreamServer, 0)

		for _, streamServer := range streamServers {
			found := false
			for _, healthyServer := range healthyServers {
				if streamServer.URL == healthyServer {
					found = true
					break
				}
			}

			if found {
				updatedStreamServers = append(updatedStreamServers, streamServer)
			}
		}

		streamServers = updatedStreamServers

		servers = healthyServers

		waitingServers = []string{}
		serverLock.Unlock()
		waitingServerLock.Unlock()

		<-timer
	}
}

// IngestServer : Server to handle requests sent from deploy servers ingesting
type IngestServer struct{}

// OutgestServer : Server to handle requests to stream data to client
type OutgestServer struct{}

// OtherServer : Server to handle all other requests that are not streaming
type OtherServer struct{}

func (b *IngestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	cid := r.URL.EscapedPath()[:strings.LastIndex(r.URL.EscapedPath(), "/")]

	refs := make([]struct {
		count int
		URL   string
	}, 0)

	for _, streamServer := range streamServers {
		if streamServer.cameraID == cid {
			http.Redirect(w, r, fmt.Sprintf("https://%s%s", streamServer.URL, r.RequestURI), http.StatusTemporaryRedirect)
			return
		}

		sentry := false
		for _, ref := range refs {
			if ref.URL == streamServer.URL {
				ref.count++
				sentry = true
				break
			}
		}

		if sentry {
			continue
		}

		refs = append(refs, struct {
			count int
			URL   string
		}{0, streamServer.URL})
	}

	lowestIndex := 0
	lowestCount := 0

	for i, ref := range refs {
		if ref.count > lowestCount {
			lowestCount = ref.count
			lowestIndex = i
		}
	}

	streamServers = append(streamServers, &StreamServer{refs[lowestIndex].URL, cid})

	http.Redirect(w, r, fmt.Sprintf("https://%s%s", refs[lowestIndex].URL, r.RequestURI), http.StatusTemporaryRedirect)
}

func (s *OutgestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sid := strings.Split(r.URL.Path, "/")[0]
	session := strings.Split(r.URL.Path, "/")[1]

	role, err := checkSession(sid, session)

	if err != nil {
		logger.Printf("Error checking sessions! %s\n", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Error checking session!"))
		return
	}

	switch role {
	case "S", "T":
		rows, err := db.Query(`SELECT cameras.id FROM sessions
    INNER JOIN people ON sessions.uname=people.uname
    INNER JOIN roster ON people.id=roster.pid
	INNER JOIN classes ON roster.cid=classes.id
	INNER JOIN cameras ON cameras.room=classes.room
    INNER JOIN periods ON classes.period=periods.code
    WHERE periods.stime<unix_timestamp()+60 AND periods.etime>unix_timestamp() AND sessions.id=?;`, session)

		if err != nil {
			logger.Printf("Error querying sessions to stream! %s\n", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Error querying session to redirect stream!"))
			return
		}

		defer rows.Close()

		if !rows.Next() {
			logger.Printf("No current period found for stream!")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"status": false, "err": "No session for stream found"}`))
			return
		}

		var (
			cid string
		)

		err = rows.Scan(&cid)

		if err != nil {
			logger.Printf("Error trying to scan database for stream server! %s\n", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error trying to scan database for data!"))
			return
		}

		for _, streamServer := range streamServers {
			if streamServer.cameraID == cid {
				http.Redirect(w, r, fmt.Sprintf("https://%s%s", streamServer.URL, r.RequestURI), http.StatusTemporaryRedirect)
				return
			}
		}

		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"status": false, "err": "No server available to process your request!"}`)
	case "A":
		filename := strings.Split(r.URL.Path, "/")

		var cid string

		err := db.QueryRow(`SELECT cameras.id FROM classes
		INNER JOIN cameras ON classes.room=cameras.room WHERE cameras.room=?;`, filename[len(filename)-2]).Scan(&cid)

		if err != nil {
			fhandle(w, "Could not select camera id for given room!", err)
			return
		}

		for _, streamServer := range streamServers {
			if streamServer.cameraID == cid {
				http.Redirect(w, r, fmt.Sprintf("https://%s%s", streamServer.URL, r.RequestURI), http.StatusTemporaryRedirect)
				return
			}
		}

		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"status": false, "err": "No server available to process your request!"}`)
	}
}

func (o *OtherServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	robinLocker.Lock()

	robinManager++
	if robinManager >= len(servers) {
		robinManager = 0
	}

	if len(servers) == 0 {
		robinLocker.Unlock()
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "No servers to handle your request!")
		return
	}

	http.Redirect(w, r, fmt.Sprintf("https://%s%s", servers[robinManager], r.RequestURI), http.StatusTemporaryRedirect)

	robinLocker.Unlock()
}
