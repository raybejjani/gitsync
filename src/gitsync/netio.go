package gitsync

import (
	"bytes"
	"encoding/gob"
	log "github.com/ngmoco/timber"
	"net"
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

// init registers GitChange with gob to ensure we can send it on the network.
func init() {
	gob.Register(GitChange{})
}

// establishConnPair builds a send/receive connection pair to work with the
// multicast network at address addr
func establishConnPair(addr *net.UDPAddr) (recvConn, sendConn *net.UDPConn, err error) {
	if recvConn, err = net.ListenMulticastUDP("udp", nil, addr); err != nil {
		return
	}

	if sendConn, err = net.DialUDP("udp", nil, addr); err != nil {
		return
	}

	return
}

// networkToChannel loops, reading packets of the network and deserialising them
// then places them onto out
func networkToChannel(l log.Logger, conn *net.UDPConn, out chan GitChange) {
	for {
		var (
			rawPacket = make([]byte, 1024)
			reader    = bytes.NewReader(rawPacket)
			dec       = gob.NewDecoder(reader)
		)

		if _, err := conn.Read(rawPacket); err != nil {
			l.Critical("Cannot read socket: %s", err)
			return
		}

		var change GitChange

		if err := dec.Decode(&change); err != nil {
			l.Critical("%s", err)
			continue
		}

		l.Debug("received %+v", change)
		out <- change
	}
}

// sendChangeToNetwork encodes and sends a GitChange to the conn (assumed to be
// connected).
func sendChangeToNetwork(l log.Logger, conn *net.UDPConn, change *GitChange) (bytesWritten int, err error) {
	l.Info("Sending %+v", change)
	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)

	if err = enc.Encode(change); err != nil {
		return
	}

	l.Fine("Sending %+v", buf.Bytes())
	if bytesWritten, err = conn.Write(buf.Bytes()); err != nil {
		return
	}

	return
}

// NetIO shares GitChanges on toNet with the network via a multicast group. It
// will pass on GitChanges from the network via fromNet. It uniques the daemon
// instance by changing the .Name member to be name@<host IP>/<original .Name)
func NetIO(l log.Logger, repo Repo, addr *net.UDPAddr, fromNet, toNet chan GitChange) {
	var (
		err                error
		recvConn, sendConn *net.UDPConn // UDP connections to allow us to send and receive change updates
		peerAddr           string       // the address we will put into network-bound changes
		// decodeFromNet is an intermediate channel for packets from the network. We
		// do some checks on them before we pass them onto toNet
		decodedFromNet = make(chan GitChange, cap(fromNet))
	)

	l.Info("Joining %v multicast(%t) group", addr, addr.IP.IsMulticast())
	if recvConn, sendConn, err = establishConnPair(addr); err != nil {
		l.Critical("Error joining listening: %s\n", addr, err)
		return
	}

	l.Info("Successfully joined %v multicast(%t) group", addr, addr.IP.IsMulticast())
	defer recvConn.Close()
	defer sendConn.Close()
	peerAddr = sendConn.LocalAddr().(*net.UDPAddr).String()

	go networkToChannel(l, recvConn, decodedFromNet)

	for {
		select {
		case change, ok := <-toNet:
			// exit when this channel is closed
			if !ok {
				return
			}

			// Drop requests that have a PeerAddr since that means they were on the
			// network already (and shared by us locally).
			if change.IsFromNetwork() {
				continue
			}

			// Add fields that matter for the network
			change.PeerAddr = peerAddr

			if _, err := sendChangeToNetwork(l, sendConn, &change); err != nil {
				l.Critical("%s", err)
				continue
			}

		case change := <-decodedFromNet:
			if rootCommit, err := repo.RootCommit(); err != nil {
				log.Critical("Error getting root commit")
			} else {
				if (peerAddr != change.PeerAddr) && (rootCommit == change.RootCommit) {
					fromNet <- change
				}
			}
		}
	}
}
