package gitsync

import (
	"bytes"
	"fmt"
	log "github.com/ngmoco/timber"
	"os/exec"
	"path"
	"regexp"
)

// match: '* master 98ce09c goo'
var branchLineRE = regexp.MustCompile(`([* ]) ((?:[a-zA-Z0-9/\-_]+)|(?:[(]no branch[)]))[ ]+([0-9a-f]{40}) .*`)

// Repo represents a git repository. It provides the basic interrogation
// abilities.
type Repo interface {
	fmt.Stringer

	Name() string
	Path() string
	Branches() (branches []*GitChange, err error)
	RootCommit() (rootCommit string, err error)
}

// cliReader is a Repo that shells out to the git CLI to interrogate the git
// repo
type cliReader struct {
	repoPath string // path containing repo, e.g. /foo/bar/moo
	repoName string // directory of repo, e.g. moo
}

func NewCliRepo(repoAbsPath string) (repo *cliReader, err error) {
	return &cliReader{
		repoPath: repoAbsPath,
		repoName: path.Base(repoAbsPath)}, nil
}

func (repo *cliReader) String() string {
	return repo.repoPath
}

func (repo *cliReader) Path() string {
	return repo.repoPath
}

func (repo *cliReader) Name() string {
	return repo.repoName
}

func (repo *cliReader) RootCommit() (rootCommit string, err error) {
	cmd := exec.Command("git", "rev-list", "--max-parents=0", "HEAD")
	cmd.Dir = repo.repoPath
	output, err := cmd.Output()
	if bytes.Count(output, []byte{'\n'}) > 1 {
		log.Critical("More than one root commit present")
	}
	rootCommit = string(output)
	return
}

// Branches reads all branches in a git repo
func (repo *cliReader) Branches() (branches []*GitChange, err error) {
	cmd := exec.Command("git", "branch", "-v", "--abbrev=40")
	cmd.Dir = repo.repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	rootCommit, err := repo.RootCommit()
	if err != nil {
		log.Critical("Unable to get root commit")
	}
	for _, m := range branchLineRE.FindAllStringSubmatch(string(out), -1) {
		branches = append(branches, &GitChange{
			RefName:    m[2],
			Current:    m[3],
			CheckedOut: m[1] == "*",
			RootCommit: rootCommit,
			RepoName:   repo.Name(),
		})
	}

	return
}
