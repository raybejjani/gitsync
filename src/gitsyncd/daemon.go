package main

import (
	"errors"
	"flag"
	"fmt"
	log "github.com/ngmoco/timber"
	"gitsync"
	"gitsync/changebus"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
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

// fetchRemoteOnChange will run a git fetch on remotes that have changed in repo.
// This happens in response to a GitChange seen on changes.
func fetchRemoteOnChange(changes chan gitsync.GitChange, repo gitsync.Repo) {
	log.Debug("Starting fetchRemoteOnChange on %s", repo.Name())
	defer log.Debug("Stopping fetchRemoteOnChange on %s", repo.Name())
	for {
		select {
		case change, ok := <-changes:
			if !ok {
				log.Debug("Exiting Loop")
				return
			}

			log.Debug("saw %+v", change)
			if change.FromRepo(repo) {
				log.Info("Fetching remote changes for %s", repo.Name())
				if err := repo.FetchRemoteChange(change); err != nil {
					log.Warn("Error fetching change")
				} else {
					log.Debug("fetched change")
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
	if err = repo.Share(); err != nil {
		return fmt.Errorf("Unable to start git daemon: %s", err.Error())
	}

	go gitsync.PollRepoForChanges(log.Global, repo, changes, pollPeriod)

	return
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
		bus = changebus.New(8, nil)
	)

	// begin observing the repo
	repo, err := gitsync.NewCliRepo(username, dirName)
	if err != nil {
		fatalf("Cannot open repo: %s", err)
	}

	if err = manageLocalGitRepo(repo, 1*time.Second, bus.GetPublishChannel()); err != nil {
		fatalf("Cannot manage repo: %s", err)
	}
	go fetchRemoteOnChange(bus.GetNewListener(), repo)

	go gitsync.NetIO(log.Global, repo, groupAddr, bus.GetPublishChannel(), bus.GetNewListener())
	if webPort != 0 {
		go serveChangesWeb(uint16(webPort), bus.GetNewListener())
	}

	defer func() {
		if err := repo.Cleanup(); err != nil {
			log.Warn("Could not delete gitsync branches ", err)
		}
	}()

	// wait until we are told to shut down
	exitOnSignal(os.Kill, os.Interrupt, syscall.SIGUSR1)
}
