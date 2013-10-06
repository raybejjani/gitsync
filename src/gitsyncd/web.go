package main

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"fmt"
	log "github.com/ngmoco/timber"
	"gitsync"
	"html"
	"net/http"
)

// handleGitChangeWebClient
func handleGitChangeWebClient(events chan *gitsync.GitChange, ws *websocket.Conn) {
	log.Info("Saw %+v", ws)
	ws.Write([]byte("FOOBAR"))
	for event := range events {
		if data, err := json.Marshal(event); err != nil {
			log.Info("Cannot marshall event data %+v. %s", event, err)
		} else {
			ws.Write(data)
		}
	}
}

// serveWeb starts a webserver that can serve a page and websocket events as
// they are seen.
// It is expected to be run only once and uses the http package global request
// router. It does NOT return.
func serveWeb(port uint16, events chan *gitsync.GitChange) {
	// Static page handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
	})

	// Events endpoint
	http.Handle("/events", websocket.Handler(func(ws *websocket.Conn) {
		handleGitChangeWebClient(events, ws)
	}))

	log.Info("Attempting to spawn webserver on %d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%v", port), nil); err != nil {
		log.Error("Error listening on %d: %s", port, err)
	}
}
