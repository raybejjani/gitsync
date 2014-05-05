package main

import (
	"fmt"
	log "github.com/ngmoco/timber"
	"gitsync"
	"gitsync/changebus"
	"gitsyncd/webcontent"
	"net/http"
)

// setupStaticServing registers an endpoint to handle static assets that are
// part of the binary
func setupStaticServing(mux *http.ServeMux) error {
	// Handle any static files (JS/CSS files)
	if handler, err := webcontent.NewMapHandler(webcontent.Paths); err != nil {
		return err
	} else {
		http.Handle("/", handler)
	}
	return nil
}

// startWebServer will serve content on port. It returns a channel that
// publishes GitChanges to connected websocket clients
func startWebServer(port uint16) chan gitsync.GitChange {
	// A pubsub bus to share changes seen by the webserver with all connected clients
	var clientBus = changebus.New(8, nil)

	// Setup endpoints
	if err := setupStaticServing(http.DefaultServeMux); err != nil {
		log.Error("Cannot setup static file serving: %s", err)
	}
	if err := setupEventAPI(http.DefaultServeMux, clientBus); err != nil {
		log.Error("Cannot setup events serving: %s", err)
	}

	// run the server
	go func() {
		log.Info("Attempting to spawn webserver on %d", port)
		if err := http.ListenAndServe(fmt.Sprintf(":%v", port), nil); err != nil {
			log.Error("Error listening on %d: %s", port, err)
		}
	}()

	return clientBus.GetPublishChannel()
}
