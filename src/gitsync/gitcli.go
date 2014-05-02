package gitsync

import (
	"bytes"
	"fmt"
	log "github.com/ngmoco/timber"
	"os"
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
	User() string
	Branches() (branches []*GitChange, err error)
	RootCommit() (rootCommit string, err error)
	// FetchRemoteChange forces a fetch from change's source to a local branch
	// named gitsync-<remote username>-<remote branch name>
	FetchRemoteChange(change GitChange) (err error)

	// Share spawns a gitdaemon instance for this repository. This allows remote
	// clients to connect and get fetch data.
	Share() (err error)

	// Cleanup removes gitsync specific artifacts (braches, remotes etc.) from a
	// repo.
	Cleanup() (err error)
}

// cliReader is a Repo that shells out to the git CLI to interrogate the git
// repo
type cliReader struct {
	repoPath string // path containing repo, e.g. /foo/bar/moo
	repoName string // directory of repo, e.g. moo
	userName string
}

func NewCliRepo(userName string, repoAbsPath string) (repo *cliReader, err error) {
	return &cliReader{
		repoPath: repoAbsPath,
		repoName: path.Base(repoAbsPath),
		userName: userName}, nil
}

func (repo *cliReader) String() string {
	return repo.repoPath
}

func (repo *cliReader) Path() string {
	return repo.repoPath
}

func (repo *cliReader) User() string {
	return repo.userName
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
	rootCommit = string(bytes.TrimSpace(output))
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
			User:       repo.userName,
		})
	}

	return
}

// FetchRemoteChange forces a fetch from change's source to a local branch
// named gitsync-<remote username>-<remote branch name>
func (repo *cliReader) FetchRemoteChange(change GitChange) (err error) {
	// We force a fetch from the change's source to a local branch
	// named gitsync-<remote username>-<remote branch name>
	localBranchName := fmt.Sprintf("gitsync-%s-%s", change.User, change.RefName)
	fetchUrl := fmt.Sprintf("git://%s/%s", change.GetPeerIP(), change.RepoName)
	cmd := exec.Command("git", "fetch", "-f", fetchUrl,
		fmt.Sprintf("%s:%s", change.RefName, localBranchName))
	cmd.Dir = repo.Path()
	return cmd.Run()
}

func (repo *cliReader) Share() (err error) {
	// add in the flag to tell git-daemon to share this repo. It is a file in
	// the .git directory
	daemonSentinel := path.Join(repo.Path(), ".git",
		"git-daemon-export-ok")
	if _, err := os.Stat(daemonSentinel); os.IsNotExist(err) {
		_, err := os.Create(daemonSentinel)
		if err != nil {
			log.Fatalf("Unable to set up git daemon")
		}
	}

	// run the daemon
	cmd := exec.Command("git", "daemon", "--reuseaddr",
		fmt.Sprintf("--base-path=%s/..", repo.Path()),
		repo.Path())
	return cmd.Start()
}

// Cleanup deletes all local branches beginning with 'gitsync-'
func (repo *cliReader) Cleanup() (err error) {
	getBranches := exec.Command("git", "branch")
	getGitsyncBranches := exec.Command("grep", "gitsync-")
	deleteGitsyncBranches := exec.Command("xargs", "git", "branch", "-D")
	getBranches.Dir = repo.Path()
	getGitsyncBranches.Dir = repo.Path()
	deleteGitsyncBranches.Dir = repo.Path()
	getGitsyncBranches.Stdin, _ = getBranches.StdoutPipe()
	deleteGitsyncBranches.Stdin, _ = getGitsyncBranches.StdoutPipe()

	// run all delete commands and return early on error
	deleters := [...]func() error{deleteGitsyncBranches.Start,
		getGitsyncBranches.Start,
		getBranches.Run,
		getGitsyncBranches.Wait,
		deleteGitsyncBranches.Wait}
	for _, deleteFunc := range deleters {
		err = deleteFunc()
		if err != nil {
			return err
		}
	}
	return
}
