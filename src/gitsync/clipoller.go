package gitsync

import (
	"fmt"
	"os"
	"time"
)

/* PollDirectory will poll a git repo.
 * It will look for changes to branches and tags including creation and
 * deletion.
 */
func PollDirectory(repo Repo, changes chan GitChange, period time.Duration) {
	fmt.Printf("Watching %s\n", repo)
	defer func() { fmt.Printf("Stopped watching %s\n", repo) }()

	prev := make(map[string]*GitChange) // last seen ref status

	for {
		next := make(map[string]*GitChange) // currently seen refs

		branches, err := repo.Branches()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot get branch list for %s: %s\n", repo, err)
		}

		for _, branch := range branches {
			old, present := prev[branch.RefName]

			next[branch.RefName] = branch

			if present {
				branch.Current = old.Current
				delete(prev, branch.RefName)
			}

			switch {
			case !present, old.Current != branch.Current, old.CheckedOut != branch.CheckedOut:
				changes <- *branch
			}
		}

		// report remaining branches in prev as deleted
		for _, old := range prev {
			old.Prev = old.Current
			old.Current = ""
			old.CheckedOut = false

			changes <- *old
		}

		prev = next

		// run cmd every period
		time.Sleep(period)
	}
}
