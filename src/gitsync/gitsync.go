/* Package gitsync provides the tools to syncronise git repositories between
 * peers.
 */
package gitsync

import (
	"fmt"
)

type GitChange struct {
	RefName       string // name of reference
	Prev, Current string // previous and current reference for branch
	CheckedOut    bool
}

func RecieveChanges(name string, changes chan GitChange) {
	for {
		select {
		case change, ok := <-changes:
			if !ok {
				fmt.Printf("%s: Exiting Loop\n", name)
				break
			}

			fmt.Printf("%s: saw %+v\n", name, change)
		}
	}
}
