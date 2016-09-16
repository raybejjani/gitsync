// Package gitsync provides the tools to syncronise git repositories between
// peers.
package gitsync

import (
	log "github.com/ngmoco/timber"
)

type GitChange struct {
	User          string // username at host
	HostIp        string // IP address of host
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
