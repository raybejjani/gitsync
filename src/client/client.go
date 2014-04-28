// This is an example websocket client from the docs at:
// http://godoc.org/code.google.com/p/go.net/websocket#example-Dial
package main

import (
	"flag"
	"fmt"
	"log"

	"code.google.com/p/go.net/websocket"
)

func main() {
	var (
		hostname = flag.String("host", "localhost", "Host to connect to")
		port     = flag.Int("port", 12345, "Port to connect to")
	)
	flag.Parse()

	origin := fmt.Sprintf("http://%s/", *hostname)
	url := fmt.Sprintf("ws://%s:%d/events", *hostname, *port)
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
