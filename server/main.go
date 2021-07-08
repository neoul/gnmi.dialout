package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	dialout "github.com/neoul/gnmi.dialout"
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
	var sessionId int
	var interval int64
	var stopState map[int]bool = make(map[int]bool)

	fmt.Println("********************************************************")
	fmt.Println("[Command]")
	fmt.Println("Stop         : put the 'enter' key")
	fmt.Println("Restart      : put the 'enter' key again during 'stop'")
	fmt.Println("Interval Stop: put the 'enter' key with number")
	fmt.Println("Show         : put the 'show' command")
	fmt.Println("Close        : put the 'close' command")
	fmt.Println("********************************************************")
	fmt.Println("[Example]")
	fmt.Println("1.Stop session 1")
	fmt.Println("$ 1 (enter)")
	fmt.Println("2.Restart session 1")
	fmt.Println("$ 1 (enter)")
	fmt.Println("3.Stop session 1 for 10sec")
	fmt.Println("$ 1 10 (enter)")
	fmt.Println("4.Show peer session list")
	fmt.Println("$ show (enter)")
	fmt.Println("5.Close server")
	fmt.Println("$ close (enter)")
	fmt.Println("********************************************************")
	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter:")
		cmd, err := reader.ReadString('\n')
		if err != nil {
			server.Close()
			return
		}
		cmd = strings.TrimSpace(cmd)
		if strings.Compare(cmd, "close") == 0 {
			server.Close()
			stopState = make(map[int]bool)
			return
		} else if strings.Compare(cmd, "show") == 0 {
			data := []string{}
			for i, v := range server.GetSessionInfo(data) {
				fmt.Printf("[%d] %s [stop=%v]\n", i+1, v, stopState[sessionId])
			}
			continue
		}

		args := strings.Split(cmd, " ")
		sessionId, err = strconv.Atoi(args[0])
		if err != nil {
			fmt.Println("%% Please enter session id")
			continue
		}
		if len(args) > 1 {
			//interval stop
			interval, err = strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				fmt.Println("%% Please enter interval time")
				continue
			}
			server.IntervalPauseSession(sessionId, interval)
		} else {
			//stop & restart
			if _, ok := stopState[sessionId]; !ok {
				//new one
				server.PauseSession(sessionId)
				stopState[sessionId] = true
				continue
			}

			if stopState[sessionId] {
				//current is 'stop'
				server.RestartSession(sessionId)
				stopState[sessionId] = false
			} else {
				//current is 'run'
				server.PauseSession(sessionId)
				stopState[sessionId] = true
			}
		}
	}
}
