package main

import (
	"gitsync"
	"log"
	"os"
	"os/signal"
	"os/user"
	"time"
)

func FanIn(inChannels ...chan gitsync.GitChange) (target chan gitsync.GitChange) {
	target = make(chan gitsync.GitChange)

	for _, c := range inChannels {
		go func(in chan gitsync.GitChange) {
			for {
				newVal, stillOpen := <-in
				if !stillOpen {
					return
				}

				target <- newVal
			}
		}(c)
	}

	return
}

func FanOut(source chan gitsync.GitChange, outChannels ...chan gitsync.GitChange) {
	go func() {
		for {
			newVal, stillOpen := <-source
			if !stillOpen {
				return
			}

			for _, out := range outChannels {
				out <- newVal
			}
		}
	}()
}

func Clone(source chan gitsync.GitChange) (duplicate chan gitsync.GitChange) {
	duplicate = make(chan gitsync.GitChange)
	FanOut(source, duplicate)
	return
}

func RecieveChanges(changes chan gitsync.GitChange) {
	for {
		select {
		case change, ok := <-changes:
			if !ok {
				log.Printf("Exiting Loop")
				break
			}

			log.Printf("saw %+v", change)
		}
	}
}

func main() {
	log.Printf("Starting")

	// Start changes handler
	var (
		dirName = os.Args[1] // directory to watch
		netName string       // name to report to network

		localChanges    = make(chan gitsync.GitChange, 128)
		localChangesDup = make(chan gitsync.GitChange, 128)
		remoteChanges   = make(chan gitsync.GitChange, 128)
		toRemoteChanges = make(chan gitsync.GitChange, 128)
	)

	// get the user's name
	if user, err := user.Current(); err != nil {
		log.Fatalf("Cannot get username: %v", err)
	} else {
		netName = user.Username
	}

	// start directory poller
	repo, err := gitsync.NewCliRepo(dirName)
	if err != nil {
		log.Fatalf("Cannot open repo: %s", err)
		os.Exit(1)
	}
	go gitsync.PollDirectory(dirName, repo, localChanges, 1*time.Second)

	// start network listener
	FanOut(localChanges, localChangesDup, toRemoteChanges)
	go gitsync.NetIO(netName, remoteChanges, toRemoteChanges)

	changes := FanIn(localChangesDup, remoteChanges)
	go RecieveChanges(changes)

	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Kill)
	<-s

	log.Printf("Exiting")
}
