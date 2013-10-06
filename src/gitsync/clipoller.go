package gitsync

import (
	log "github.com/ngmoco/timber"
	"time"
)

// PollDirectory will poll a git repo.
// It will look for changes to branches and tags including creation and
// deletion.
func PollDirectory(l log.Logger, name string, repo Repo, changes chan GitChange, period time.Duration) {
	l.Info("Watching %s as %s\n", repo, name)
	defer l.Info("Stopped watching %s as %s\n", repo, name)

	prev := make(map[string]*GitChange) // last seen ref status

	// Every poll period, get the list of branches.
	// For those seen before, fill in previous and currect SHA in change. Remove
	// from prev set.
	// For those that are new, fill in data.
	// For remaining entries in prev set, these are deleted. Send them with
	// current as empty.
	for firstAttempt := true; ; firstAttempt = false {
		var (
			next     = make(map[string]*GitChange) // currently seen refs, becomes prev set
			branches []*GitChange                  // working set of branches
			err      error
		)

		// run cmd every period, except on the first try
		if !firstAttempt {
			time.Sleep(period)
		}

		if branches, err = repo.Branches(); err != nil {
			l.Critical("Cannot get branch list for %s: %s", repo, err)
			continue
		}
		for _, branch := range branches {
			var (
				old, seenBefore  = prev[branch.RefName]
				existsAndChanged = seenBefore && (old.Current != branch.Current || old.CheckedOut != branch.CheckedOut)
			)
			branch.Name = name // always assign a name
			next[branch.RefName] = branch
			if existsAndChanged {
				branch.Prev = old.Current
			}

			// share changes and new branches
			if !seenBefore || existsAndChanged {
				changes <- *branch
			}

			// Cleanup any branch we have seen before, and handled above
			if seenBefore {
				delete(prev, branch.RefName)
			}
		}

		// report remaining branches in prev as deleted
		// Note: Use the prev set object since we have no current one to play with
		for _, old := range prev {
			old.Prev = old.Current
			old.Current = ""
			old.CheckedOut = false

			changes <- *old
		}

		prev = next
	}
}
