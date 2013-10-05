package gitsync

import (
	"fmt"
	"os/exec"
	"regexp"
)

// match: '* master 98ce09c goo'
var branchLineRE = regexp.MustCompile(`([* ]) ((?:[a-zA-Z0-9/\-_]+)|(?:[(]no branch[)]))[ ]+([0-9a-f]{40}) .*`)

// Repo represents a git repository. It provides the basic interrogation
// abilities.
type Repo interface {
	fmt.Stringer

	Branches() (branches []*GitChange, err error)
}

// cliReader is a Repo that shells out to the git CLI to interrogate the git
// repo
type cliReader struct {
	path string // directory containing repo
}

func NewCliRepo(path string) (repo *cliReader, err error) {
	return &cliReader{path: path}, nil
}

func (repo *cliReader) String() string {
	return repo.path
}

// Branches reads all branches in a git repo
func (repo *cliReader) Branches() (branches []*GitChange, err error) {
	cmd := exec.Command("git", "branch", "-av", "--abbrev=40")
	cmd.Dir = repo.path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	for _, m := range branchLineRE.FindAllStringSubmatch(string(out), -1) {
		branches = append(branches, &GitChange{
			RefName:    m[2],
			Current:    m[3],
			CheckedOut: m[1] == "*",
		})
	}

	return
}
