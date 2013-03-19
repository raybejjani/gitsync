package main

import (
	"fmt"
	"gitsync"
	"os"
	"os/signal"
	"time"
)

func main() {
	fmt.Printf("Starting\n")

	changes := make(chan gitsync.GitChange, 128)

	go gitsync.RecieveChanges("fubar", changes)

	repo, err := gitsync.NewCliRepo(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open repo: %s", err)
		os.Exit(1)
	}
	go gitsync.PollDirectory(repo, changes, 1*time.Second)

	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Kill)
	<-s
	close(changes)

	fmt.Printf("Exiting\n")
}
