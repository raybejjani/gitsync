package main

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"fmt"
	log "github.com/ngmoco/timber"
	"gitsync"
	"gitsync/changebus"
	"net/http"
)

// handleGitChangeWebClient will distribute events to a single connected
// websocket client. It will exit on error or when the client disconnects.
// Note: Client disconnect detection is delayed by one GitChange event.
func handleGitChangeWebClient(events chan gitsync.GitChange, ws *websocket.Conn) {
	clientPrettyName := fmt.Sprintf("[%p]%s", ws, ws.RemoteAddr())

	log.Info("Begin handling %s", clientPrettyName)
	defer log.Info("End handling %s", clientPrettyName)

	for event := range events {
		if data, err := json.Marshal(event); err != nil {
			log.Info("%s: Cannot marshall event data %+v. %s", clientPrettyName, event, err)
		} else {
			if _, err := ws.Write(data); err != nil {
				ws.Close()
				log.Error("%s: Cannot write out event: %s", clientPrettyName, err)
				return
			} else {
				log.Info("%s: Wrote out data", clientPrettyName)
			}
		}
	}
}

// setupEventAPI registers endpoints for event distribution.
func setupEventAPI(mux *http.ServeMux, eventBus changebus.ChangeBus) error {
	// Events endpoint
	// Note: we close over a listener channel on eventBus and curry
	// handleGitChangeWebClient into a websocket.Handler
	mux.Handle("/events", websocket.Handler(func(ws *websocket.Conn) {
		handleGitChangeWebClient(eventBus.GetNewListener(), ws)
	}))
	return nil
}
