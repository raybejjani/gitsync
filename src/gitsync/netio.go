package gitsync

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"strings"
)

var (
	Port             = 9999 // mDNS/Bonjour uses 5353
	IP4MulticastAddr = &net.UDPAddr{
		IP:   net.ParseIP("224.0.0.251"),
		Port: Port,
	}

	IP6MulticastAddr = &net.UDPAddr{
		IP:   net.ParseIP("ff02::fb"),
		Port: Port,
	}
)

func init() {
	gob.Register(GitChange{})
}

func establishConnPair(addr *net.UDPAddr) (recvConn, sendConn *net.UDPConn, err error) {
	if recvConn, err = net.ListenMulticastUDP("udp", nil, addr); err != nil {
		return
	}

	if sendConn, err = net.DialUDP("udp", nil, addr); err != nil {
		return
	}

	return
}

// NetIO shares GitChanges on toNet with the network via a multicast group. It
// will pass on GitChanges from the network via fromNet. It uniques the daemon
// instance by changing the .Name member to be name@<host IP>/<original .Name)
func NetIO(name string, fromNet, toNet chan GitChange) {
	var (
		err                error
		netName            string       // name prefix to identify ourselves with
		recvConn, sendConn *net.UDPConn // UDP connections to allow us to send and	receive change updates
	)

	log.Printf("Joining %v multicast(%t) group", IP4MulticastAddr, IP4MulticastAddr.IP.IsMulticast())
	if recvConn, sendConn, err = establishConnPair(IP4MulticastAddr); err != nil {
		log.Fatalf("Error joining listening: %s\n", IP4MulticastAddr, err)
		return
	}

	log.Printf("Successfully joined %v multicast(%t) group", IP4MulticastAddr, IP4MulticastAddr.IP.IsMulticast())
	defer recvConn.Close()
	defer sendConn.Close()
	netName = fmt.Sprintf("%s@%s", name, sendConn.LocalAddr().(*net.UDPAddr).IP)

	term := false
	defer func() { term = true }()
	rawFromNet := make(chan []byte, 128)
	go func() {
		for !term {
			b := make([]byte, 1024)

			if n, err := recvConn.Read(b); err != nil {
				log.Fatalf("Cannot read socket: %s", err)
				continue
			} else {
				rawFromNet <- b[:n]
			}
		}
	}()

	for {
		select {
		case req, ok := <-toNet:
			if !ok {
				return
			}

			req.Name = fmt.Sprintf("%s:%s", netName, req.Name)

			//log.Printf("Sending %+v", req)
			buf := &bytes.Buffer{}
			enc := gob.NewEncoder(buf)

			if err := enc.Encode(req); err != nil {
				log.Fatalf("%s", err)
			}

			//log.Printf("Sending %+v", buf.Bytes())
			if _, err := sendConn.Write(buf.Bytes()); err != nil {
				log.Fatalf("%s", err)
			}

		case resp := <-rawFromNet:
			var msg GitChange
			dec := gob.NewDecoder(bytes.NewReader(resp))

			if err := dec.Decode(&msg); err != nil {
				log.Fatalf("%s", err)
			} else {
				//log.Printf("received %+v", msg)
			}

			if !strings.HasPrefix(msg.Name, netName) {
				fromNet <- msg
			}
		}
	}
}
