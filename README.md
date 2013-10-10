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

Installing
----------
There is a .pkg installer available for OS X Intel
[here](https://github.com/raybejjani/gitsync/releases/tag/0.5). For
other platforms, download the source and see the 'Compiling' section
below.

Running
-------
Run with `gitsyncd /path/to/repo`.

You can open up a local webserver to see a live-updating page of your
coworkers' changes by supplying a port number: `gitsyncd /path/to/repo
<port>`. Then go to `http://localhost:<port>` (it's very rudimentary
for now).

See extended options by running `gitsyncd -h`.

Compiling
-------
Run `make`. You need to to have the [Go runtime](http://golang.org)
installed.
