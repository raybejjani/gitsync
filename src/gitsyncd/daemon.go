package main

import (
	"flag"
	"fmt"
	"gitsync"
	"log"
	"net"
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
	if len(flag.Args()) == 0 {
		log.Fatalf("No Git directory supplied")
	}

	log.Printf("Starting")

	// Start changes handler
	var (
		username  = flag.String("user", "", "Username to report when sending changes to the network")
		groupIP   = flag.String("ip", gitsync.IP4MulticastAddr.IP.String(), "Multicast IP to connect to")
		groupPort = flag.Int("port", gitsync.IP4MulticastAddr.Port, "Port to use for network IO")
	)
	flag.Parse()

	var (
		err       error
		dirName   = flag.Args()[0] // directories to watch
		netName   string           // name to report to network
		groupAddr *net.UDPAddr     // network address to connect to

		// channels to move change messages around
		localChanges    = make(chan gitsync.GitChange, 128)
		localChangesDup = make(chan gitsync.GitChange, 128)
		remoteChanges   = make(chan gitsync.GitChange, 128)
		toRemoteChanges = make(chan gitsync.GitChange, 128)
	)

	// get the user's name
	if *username != "" {
		netName = *username
	} else if user, err := user.Current(); err == nil {
		netName = user.Username
	} else {
		log.Fatalf("Cannot get username: %v", err)
	}

	if groupAddr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", *groupIP, *groupPort)); err != nil {
		log.Fatalf("Cannot resolve address %s: %v", err)
		os.Exit(1)
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
	go gitsync.NetIO(netName, groupAddr, remoteChanges, toRemoteChanges)

	changes := FanIn(localChangesDup, remoteChanges)
	go RecieveChanges(changes)

	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Kill)
	<-s

	log.Printf("Exiting")
}
