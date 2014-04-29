// Package gitsync provides the tools to syncronise git repositories between
// peers.
package gitsync

import (
	log "github.com/ngmoco/timber"
	"strings"
)

type GitChange struct {
	User string // username that produced this change
	// PeerAddr is the address as "IP:port" of peer that generated this change. It
	// is the source IP:Port for their connection to the multicast group.
	// Note: This filled before a change is placed on the network and may be blank
	// for local only changes.
	PeerAddr      string
	RepoName      string // name of repo directory
	RefName       string // name of reference
	Prev, Current string // previous and current reference for branch
	RootCommit    string // ref hash of the first commit (assuming there is only one)
	CheckedOut    bool
}

func (change GitChange) FromRepo(repo Repo) bool {
	repoRoot, err := repo.RootCommit()
	if err != nil {
		log.Info("Error getting root commit")
	}
	return change.RootCommit == repoRoot
}

// GetPeerIP returns the IP of the peer that generated this change
func (change GitChange) GetPeerIP() string {
	return strings.Split(change.PeerAddr, ":")[0]
}

// GetPeerPort returns the source Port of the peer that generated this change
func (change GitChange) GetPeerPort() string {
	return strings.Split(change.PeerAddr, ":")[1]
}

// IsFromNetwork indicates if a change has come from or been on the network
func (change GitChange) IsFromNetwork() bool {
	return change.PeerAddr != ""
}
