package main

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"fmt"
	log "github.com/ngmoco/timber"
	"gitsync"
	"html"
	"net/http"
	"sync"
)

// makeWebsocketName composes an identifier for a websocket client
func makeWebsocketName(ws *websocket.Conn) string {
	return fmt.Sprintf("[%p]%s", ws, ws.RemoteAddr())
}

// clientSet is a set of websocket clients. It allows us to distribute events to
// them and manage membership
type clientSet struct {
	sync.RWMutex                                            // lock the set
	clients      map[*websocket.Conn]chan gitsync.GitChange // set of websocket clients
}

// Add Client adds a client to the set, it does not check for prior membership
func (cs *clientSet) AddClient(ws *websocket.Conn, ch chan gitsync.GitChange) {
	cs.Lock()
	defer cs.Unlock()
	cs.clients[ws] = ch
}

// RemoveClient removes the client from the set
func (cs *clientSet) RemoveClient(ws *websocket.Conn) {
	cs.Lock()
	defer cs.Unlock()
	delete(cs.clients, ws)
}

// distributeEvent will sent a copy of the event to all clients
func (cs *clientSet) distributeEvent(event gitsync.GitChange) {
	cs.RLock()
	defer cs.RUnlock()
	for _, clientChannel := range cs.clients {
		clientChannel <- event
	}
}

// distribute will loop on incoming events and send them to all listening
// clients
func (cs *clientSet) distribute(events chan *gitsync.GitChange) {
	for event := range events {
		cs.distributeEvent(*event)
	}
}

// handleGitChangeWebClient will build and register a channel to revieve events
// on from a distributor in clientSet. It will exit on error but will consume
// one more event after a client disconnects before it does so.
func handleGitChangeWebClient(cs *clientSet, ws *websocket.Conn) {
	var events = make(chan gitsync.GitChange)

	log.Info("Begin handling %s", makeWebsocketName(ws))
	defer log.Info("End handling %s", makeWebsocketName(ws))

	cs.AddClient(ws, events)
	defer cs.RemoveClient(ws)

	for event := range events {
		if data, err := json.Marshal(event); err != nil {
			log.Info("%s: Cannot marshall event data %+v. %s", makeWebsocketName(ws), event, err)
		} else {
			if _, err := ws.Write(data); err != nil {
				ws.Close()
				log.Error("%s: Cannot write out event: %s", makeWebsocketName(ws), err)
				return
			} else {
				log.Info("%s: Wrote out data", makeWebsocketName(ws))
			}
		}
	}
}

// serveWeb starts a webserver that can serve a page and websocket events as
// they are seen.
// It is expected to be run only once and uses the http package global request
// router. It does NOT return.
func serveWeb(port uint16, events chan *gitsync.GitChange) {
	// the container for websocket clients, passed into every websocket handler
	// below
	var cs = clientSet{
		clients: make(map[*websocket.Conn]chan gitsync.GitChange)}

	// Static page handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
	})

	// Events endpoint
	// Note: we wrap the handler in a closure to pass in the clientSet
	http.Handle("/events", websocket.Handler(func(ws *websocket.Conn) {
		handleGitChangeWebClient(&cs, ws)
	}))

	log.Info("Attempting to spawn webserver on %d", port)
	go cs.distribute(events)
	if err := http.ListenAndServe(fmt.Sprintf(":%v", port), nil); err != nil {
		log.Error("Error listening on %d: %s", port, err)
	}
}
