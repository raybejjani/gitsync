gitsync (BETA)
=======

gitsync is a repository-syncronisation daemon, whose purpose is to
keep coders on the same project aware of each others' work without
requiring any pushes to remotes. Running on machines on the same local
network, on any given peer it will auto-fetch any branches modified on
the other peers.

For example, say Alice and Bob are working on repo 'foo' on their
separate machines. With gitsyncd running on both machines, everytime
Alice makes a local commit, Bob's machine will auto-fetch Alice's
modified branch into a local one named `gitsync-Alice-branch`.

Run with `gitsyncd /path/to/repo`.

You can open up a local webserver to see a live-updating page of your
coworker's changes by suppling a port number: `gitsyncd /path/to/repo
port`.

See extended options by running `gitsyncd -h`.
