package main

import (
	"errors"
	"flag"
	"fmt"
	log "github.com/ngmoco/timber"
	"gitsync"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path"
	"strings"
	"syscall"
	"time"
	"util"
)

// Configuration Variables
var (
	username  string // username used when sharing on network
	groupIP   string // Mulitcast IP for peer discovery & sharing
	groupPort int    // Mulitcast port for peer discovery & sharing
	logLevel  string // log level to emit. One of debug, info, warning, error.
	logSocket string // proto://address:port target to send logs to
	logFile   string // path to file to log to
	webPort   int    // Port for local webserver. Off(0) by default

	dirName   string       // Directory to watch
	groupAddr *net.UDPAddr // network address to connect to
)

// parseArgs handles CLI arguments and extracts their defaults. It sets the module
// globals above.
func parseArgs() (err error) {
	// setup the parser
	flag.StringVar(&username, "user", "", "Username to report when sending changes to the network")
	flag.StringVar(&groupIP, "ip", gitsync.IP4MulticastAddr.IP.String(), "Multicast IP to connect to")
	flag.IntVar(&groupPort, "port", gitsync.IP4MulticastAddr.Port, "Port to use for network IO")
	flag.StringVar(&logLevel, "loglevel", "info", "Lowest log level to emit. Can be one of debug, info, warning, error.")
	flag.StringVar(&logSocket, "logsocket", "", "proto://address:port target to send logs to")
	flag.StringVar(&logFile, "logfile", "", "path to file to log to")
	flag.IntVar(&webPort, "webport", 0, "Port for local webserver. Off by default")
	flag.Parse()

	// parse the directory to watch
	if len(flag.Args()) == 0 {
		return fmt.Errorf("No Git directory supplied")
	} else {
		dirName = util.AbsPath(flag.Args()[0])
	}

	// init logging
	if err = setupLogging(logLevel, logSocket, logFile); err != nil {
		return fmt.Errorf("Cannot setup logging: %s", err)
	}

	// get the user's name, default to the running user if not passed in
	if username == "" {
		if user, err := user.Current(); err == nil {
			username = user.Username
		} else {
			return fmt.Errorf("Cannot get username: %v", err)
		}
	}

	// parse p2p IP:port
	if groupAddr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", groupIP, groupPort)); err != nil {
		return fmt.Errorf("Cannot resolve address %v:%v: %v", groupIP, groupPort, err)
	}

	return nil
}

// unixSyslog discovers where the syslog daemon is running on the local machine
// using a Unix domain socket.
// NOTE: Stolen from golang log/syslog/syslog_unix.go
func unixSyslog() (network, addr string, err error) {
	logTypes := []string{"unixgram", "unix"}
	logPaths := []string{"/dev/log", "/var/run/syslog"}
	for _, network := range logTypes {
		for _, path := range logPaths {
			conn, err := net.Dial(network, path)
			if err != nil {
				continue
			} else {
				defer conn.Close()
				return conn.RemoteAddr().Network(), conn.RemoteAddr().String(), nil
			}
		}
	}
	return "", "", errors.New("Unix syslog delivery error")
}

// setupLogging initialises logging per the parameters:
// logLevel: lowest level to log
// logSocket: socket to log to (defaults to the system syslog)
// logFile: file to log to
func setupLogging(logLevel, logSocket, logFile string) (err error) {
	var (
		network string      // the network type to pass Dial
		address string      // the address to Dial to
		level   = log.DEBUG // the lowest level to log
	)

	if logLevel != "" {
		level = log.Level(0)
		for idx, str := range log.LongLevelStrings {
			if strings.EqualFold(str, logLevel) {
				level = log.Level(idx)
			}
		}
		if level == log.Level(0) {
			return fmt.Errorf("Cannot parse log level %s", logLevel)
		}
	}

	if logSocket == "" {
		if network, address, err = unixSyslog(); err != nil {
			return
		}
	} else {
		parts := strings.Split(logSocket, "://")
		if len(parts) < 2 {
			return fmt.Errorf("Cannot parse log socket %s", logSocket)
		}
		network, address = parts[0], parts[1]
	}

	if network != "" && address != "" {
		var writer log.LogWriter
		if writer, err = log.NewSocketWriter(network, address); err != nil {
			return err
		}
		// add console output for logs
		log.AddLogger(log.ConfigLogger{
			LogWriter: writer,
			Level:     level,
			Formatter: log.NewSyslogFormatter("[%L] %s %M"),
		})
	}

	if logFile != "" {
		var writer log.LogWriter
		if writer, err = log.NewFileWriter(logFile); err != nil {
			return err
		}
		log.AddLogger(log.ConfigLogger{
			LogWriter: writer,
			Level:     level,
			Formatter: log.NewPatFormatter("[%D %T][%L] %s %M"),
		})
	}

	return
}

// fatalf logs a fatal error and exits
func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	log.Fatalf(format, args...)
}

// RecieveChanges takes local and remote changes and:
// 1- local changes: sends them on the websocket
// 2- remote changes: fetches them if they are for a repo we are watching
// It also starts the local webserver
func ReceiveChanges(changes chan gitsync.GitChange, webPort uint16, repo gitsync.Repo) {
	log.Info("webport %d", webPort)
	var webEvents = make(chan *gitsync.GitChange, 128)
	if webPort != 0 {
		go serveWeb(webPort, webEvents)
	}

	for {
		select {
		case change, ok := <-changes:
			if !ok {
				log.Debug("Exiting Loop")
				break
			}

			log.Info("saw %+v", change)
			if change.FromRepo(repo) {
				if err := fetchChange(change, repo.Path()); err != nil {
					log.Info("Error fetching change")
				} else {
					log.Info("fetched change")
				}
			}

			if webPort != 0 {
				select {
				case webEvents <- &change:
				default:
					log.Info("Dropped event %+v from websocket")
				}
			}
		}
	}
}

// manageGitRepo polls a git repository for changes and propagates them on the
// changes channel. It also spawns a gitdaemon to allow remote peers to fetch
// local git data.
// Note: It currently spawns a goroutine to do the polling.
func manageLocalGitRepo(repo gitsync.Repo, pollPeriod time.Duration, changes chan gitsync.GitChange) (err error) {
	// begin sharing the repo
	if err = shareGitRepo(repo); err != nil {
		return fmt.Errorf("Unable to start git daemon: %s", err.Error())
	}

	go gitsync.PollRepoForChanges(log.Global, repo, changes, pollPeriod)

	return
}

// shareGitRepo spawns a gitdaemon instance for this repository. This allows
// remote clients to connect and get fetch data.
func shareGitRepo(repo gitsync.Repo) error {
	daemonSentinel := path.Join(repo.Path(), ".git",
		"git-daemon-export-ok")
	if _, err := os.Stat(daemonSentinel); os.IsNotExist(err) {
		_, err := os.Create(daemonSentinel)
		if err != nil {
			log.Fatalf("Unable to set up git daemon")
		}
	}
	cmd := exec.Command("git", "daemon", "--reuseaddr",
		fmt.Sprintf("--base-path=%s/..", repo.Path()),
		repo.Path())
	err := cmd.Start()
	return err
}

// fetchChange runs git to retrieve commits from a remotely running gitdaemon.
func fetchChange(change gitsync.GitChange, dirName string) error {
	// We force a fetch from the change's source to a local branch
	// named gitsync-<remote username>-<remote branch name>
	localBranchName := fmt.Sprintf("gitsync-%s-%s", change.User, change.RefName)
	fetchUrl := fmt.Sprintf("git://%s/%s", change.HostIp, change.RepoName)
	cmd := exec.Command("git", "fetch", "-f", fetchUrl,
		fmt.Sprintf("%s:%s", change.RefName, localBranchName))
	cmd.Dir = dirName
	err := cmd.Run()
	return err
}

// cleanup deletes all local branches beginning with 'gitsync-'
func cleanup(dirName string) {
	getBranches := exec.Command("git", "branch")
	getGitsyncBranches := exec.Command("grep", "gitsync-")
	deleteGitsyncBranches := exec.Command("xargs", "git", "branch", "-D")
	getBranches.Dir = dirName
	getGitsyncBranches.Dir = dirName
	deleteGitsyncBranches.Dir = dirName
	getGitsyncBranches.Stdin, _ = getBranches.StdoutPipe()
	deleteGitsyncBranches.Stdin, _ = getGitsyncBranches.StdoutPipe()
	err := deleteGitsyncBranches.Start()
	err = getGitsyncBranches.Start()
	err = getBranches.Run()
	err = getGitsyncBranches.Wait()
	err = deleteGitsyncBranches.Wait()

	if err != nil {
		log.Info("Could not delete gitsync branches ", err)
	}
}

// exitOnSignal waits and returns on any of the passed in signals
func exitOnSignal(signals ...os.Signal) {
	s := make(chan os.Signal, 1)
	signal.Notify(s, signals...)
	<-s
}

func main() {
	if err := parseArgs(); err != nil {
		fatalf(err.Error())
	}

	log.Info("Starting up")
	defer log.Info("Exiting")

	var (
		err error

		// channels to move change messages around
		remoteChanges   = make(chan gitsync.GitChange, 128)
		toRemoteChanges = make(chan gitsync.GitChange, 128)
	)

	// begin observing the repo
	repo, err := gitsync.NewCliRepo(username, dirName)
	if err != nil {
		fatalf("Cannot open repo: %s", err)
	}

	if err = manageLocalGitRepo(repo, 1*time.Second, toRemoteChanges); err != nil {
		fatalf("Cannot manage repo: %s", err)
	}

	go gitsync.NetIO(log.Global, repo, groupAddr, remoteChanges, toRemoteChanges)
	go ReceiveChanges(remoteChanges, uint16(webPort), repo)

	defer cleanup(dirName)

	// wait until we are told to shut down
	exitOnSignal(os.Kill, os.Interrupt, syscall.SIGUSR1)
}
