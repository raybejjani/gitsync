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
	"time"
	"util"
)

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

func fetchChange(change gitsync.GitChange, dirName string) error {
	// We force a fetch from the change's source to a local branch
	// named gitsync-<remote username>-<remote branch name>
	localBranchName := fmt.Sprintf(
		"gitsync-%s-%s", change.User, change.RefName)
	fetchUrl := fmt.Sprintf(
		"git://%s/%s", change.HostIp, change.RepoName)
	cmd := exec.Command("git", "fetch", "-f", fetchUrl,
		fmt.Sprintf("%s:%s", change.RefName, localBranchName))
	cmd.Dir = dirName
	err := cmd.Run()
	return err
}

func ReceiveChanges(changes chan gitsync.GitChange, repo gitsync.Repo) {
	for {
		select {
		case change, ok := <-changes:
			if !ok {
				log.Debug("Exiting Loop")
				break
			}

			log.Info("saw %+v", change)
			fmt.Printf("saw %+v\n", change)
			if change.FromRepo(repo) {
				if err := fetchChange(change, repo.Path()); err != nil {
					log.Info("Error fetching change")
				} else {
					log.Info("fetched change")
				}
			}
		}
	}
}

func startGitDaemon(absolutePath string) error {
	daemonSentinel := path.Join(absolutePath, ".git",
		"git-daemon-export-ok")
	if _, err := os.Stat(daemonSentinel); os.IsNotExist(err) {
		_, err := os.Create(daemonSentinel)
		if err != nil {
			log.Fatalf("Unable to set up git daemon")
		}
	}
	cmd := exec.Command("git", "daemon", "--reuseaddr",
		fmt.Sprintf("--base-path=%s/..", absolutePath),
		absolutePath)
	err := cmd.Start()
	return err
}

func main() {
	// Start changes handler
	var (
		username  = flag.String("user", "", "Username to report when sending changes to the network")
		groupIP   = flag.String("ip", gitsync.IP4MulticastAddr.IP.String(), "Multicast IP to connect to")
		groupPort = flag.Int("port", gitsync.IP4MulticastAddr.Port, "Port to use for network IO")
		logLevel  = flag.String("loglevel", "info", "Lowest log level to emit. Can be one of debug, info, warning, error.")
		logSocket = flag.String("logsocket", "", "proto://address:port target to send logs to")
		logFile   = flag.String("logfile", "", "path to file to log to")
	)
	flag.Parse()

	if len(flag.Args()) == 0 {
		fatalf("No Git directory supplied")
	}

	if err := setupLogging(*logLevel, *logSocket, *logFile); err != nil {
		fatalf("Cannot setup logging: %s", err)
	}

	log.Info("Starting up")
	defer log.Info("Exiting")

	var (
		err       error
		dirName   = flag.Args()[0] // directories to watch
		netName   string           // name to report to network
		groupAddr *net.UDPAddr     // network address to connect to

		// channels to move change messages around
		remoteChanges   = make(chan gitsync.GitChange, 128)
		toRemoteChanges = make(chan gitsync.GitChange, 128)
	)
	dirName = util.AbsPath(dirName)

	// get the user's name
	if *username != "" {
		netName = *username
	} else if user, err := user.Current(); err == nil {
		netName = user.Username
	} else {
		fatalf("Cannot get username: %v", err)
	}

	if groupAddr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", *groupIP, *groupPort)); err != nil {
		fatalf("Cannot resolve address %v:%v: %v", *groupIP, *groupPort, err)
	}

	// start directory poller
	repo, err := gitsync.NewCliRepo(dirName)
	if err != nil {
		fatalf("Cannot open repo: %s", err)
	}

	if err = startGitDaemon(dirName); err != nil {
		log.Fatalf("Unable to start git daemon")
	}

	go gitsync.PollDirectory(log.Global, dirName, repo, toRemoteChanges, 1*time.Second)
	go gitsync.NetIO(log.Global, netName, groupAddr, remoteChanges, toRemoteChanges)
	go ReceiveChanges(remoteChanges, repo)

	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Kill)
	<-s
}
