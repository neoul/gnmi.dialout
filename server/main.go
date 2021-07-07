package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	dialout "gnmi.dialout"
)

var (
	address    = flag.String("address", ":57401", "ip:port address wait for gnmi dialout client")
	ca         = flag.String("ca-crt", "", "ca certificate file")
	crt        = flag.String("server-crt", "", "server certificate file")
	key        = flag.String("server-key", "", "server private key file")
	secure     = flag.Bool("secure", false, "disable tls (transport layer security")
	skipVerify = flag.Bool("skip-verify", false, "when skip-verify is true, server just verify client certificate if given")
	username   = flag.String("username", "", "master username for user authentication")
	password   = flag.String("password", "", "master password for user authentication")
	logfile    = flag.String("logfile", "gnmi.log", "server log file")
)

func main() {
	dialout.Print = log.Print
	dialout.Printf = log.Printf
	f, err := os.OpenFile(*logfile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err == nil {
		defer f.Close()
		log.SetOutput(f)
	}
	server, err := dialout.NewGNMIDialoutServer(
		*address, !(*secure), *skipVerify, *ca, *crt, *key, *username, *password)
	if err != nil {
		log.Fatal(err)
	}
	go runCmd(server)
	if err := server.Serve(); err != nil {
		log.Fatal(err)
	}
}

func runCmd(server *dialout.GNMIDialoutServer) {
	time.Sleep(20 * time.Second)

	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter command[0:stop, 1~60:interval, -1:restart, -2:close]:")
		cmd, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		cmd = strings.TrimSpace(cmd)
		icmd, err := strconv.ParseInt(cmd, 10, 64)
		if err != nil {
			return
		}

		if icmd == 0 {
			server.PauseSession(1)
		} else if icmd == -1 {
			server.RestartSession(1)
		} else if icmd == -2 {
			server.CloseSession(1)
			return
		} else {
			if icmd >= 1 && icmd <= 60 {
				server.IntervalSession(1, icmd)
			}
		}
	}
}
