// This is an example websocket client from the docs at:
// http://godoc.org/code.google.com/p/go.net/websocket#example-Dial
package main

import (
	"fmt"
	"log"

	"code.google.com/p/go.net/websocket"
)

func main() {
	origin := "http://localhost/"
	url := "ws://localhost:12345/events"
	ws, err := websocket.Dial(url, "", origin)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := ws.Write([]byte("hello, world!\n")); err != nil {
		log.Fatal(err)
	}

	for {
		var msg = make([]byte, 512)
		var n int
		if n, err = ws.Read(msg); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Received: %s.\n", msg[:n])
	}
}
