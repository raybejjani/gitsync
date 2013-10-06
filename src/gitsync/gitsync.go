// Package gitsync provides the tools to syncronise git repositories between
// peers.
package gitsync

type GitChange struct {
	Name          string // name of this host
	RefName       string // name of reference
	Prev, Current string // previous and current reference for branch
	CheckedOut    bool
}
