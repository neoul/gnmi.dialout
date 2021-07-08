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
	time.Sleep(20 * time.Second)
	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter command [show|control|close]:")
		cmd, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		cmd = strings.TrimSpace(cmd)
		if strings.Compare(cmd, "show") == 0 {
			data := []string{}
			fmt.Println("Peer Session Infomation:")
			for i, v := range server.GetSessionInfo(data) {
				fmt.Printf("[%d] %s\n", i+1, v)
			}
		} else if strings.Compare(cmd, "close") == 0 {
			server.Close()
			return
		} else {
			fmt.Print("Enter session num:")
			ses, err := reader.ReadString('\n')
			if err != nil {
				continue
			}
			ses = strings.TrimSpace(ses)
			sesi, err := strconv.Atoi(ses)
			if err != nil {
				continue
			}
			if sesi < 0 {
				continue
			}

			fmt.Print("Enter session command [stop|restart|<1-60>sec]:")
			ccmd, err := reader.ReadString('\n')
			if err != nil {
				continue
			}
			ccmd = strings.TrimSpace(ccmd)
			if strings.Compare(ccmd, "stop") == 0 {
				server.PauseSession(sesi)
			} else if strings.Compare(ccmd, "restart") == 0 {
				server.RestartSession(sesi)
			} else {
				//convert string to int64
				interval, err := strconv.ParseInt(ccmd, 10, 64)
				if err != nil {
					continue
				}
				server.IntervalPauseSession(sesi, interval*1000000000)
			}
		}
	}
}
