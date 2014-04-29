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
func NetIO(l log.Logger, repo Repo, addr *net.UDPAddr, fromNet, toNet chan GitChange) {
	var (
		err                error
		recvConn, sendConn *net.UDPConn // UDP connections to allow us to send and receive change updates
	)

	l.Info("Joining %v multicast(%t) group", addr, addr.IP.IsMulticast())
	if recvConn, sendConn, err = establishConnPair(addr); err != nil {
		l.Critical("Error joining listening: %s\n", addr, err)
		return
	}

	l.Info("Successfully joined %v multicast(%t) group", addr, addr.IP.IsMulticast())
	defer recvConn.Close()
	defer sendConn.Close()
	hostIp := sendConn.LocalAddr().(*net.UDPAddr).IP.String()

	term := false
	defer func() { term = true }()
	rawFromNet := make(chan []byte, 128)
	go func() {
		for !term {
			b := make([]byte, 1024)

			if n, err := recvConn.Read(b); err != nil {
				l.Critical("Cannot read socket: %s", err)
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

			req.User = repo.User()
			req.HostIp = hostIp

			l.Info("Sending %+v", req)
			buf := &bytes.Buffer{}
			enc := gob.NewEncoder(buf)

			if err := enc.Encode(req); err != nil {
				l.Critical("%s", err)
				continue
			}

			l.Fine("Sending %+v", buf.Bytes())
			if _, err := sendConn.Write(buf.Bytes()); err != nil {
				l.Critical("%s", err)
				continue
			}

		case resp := <-rawFromNet:
			var change GitChange
			dec := gob.NewDecoder(bytes.NewReader(resp))

			if err := dec.Decode(&change); err != nil {
				l.Critical("%s", err)
				continue
			} else {
				l.Debug("received %+v", change)
			}

			if rootCommit, err := repo.RootCommit(); err != nil {
				log.Critical("Error getting root commit")
			} else {
				if (repo.User() != change.User) && (rootCommit == change.RootCommit) {
					fromNet <- change
				}
			}
		}
	}
}
