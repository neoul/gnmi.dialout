package main

import (
	"flag"
	"log"

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
)

func main() {
	dialout.Print = log.Print
	dialout.Printf = log.Printf
	server, err := dialout.NewGNMIDialoutServer(
		*address, !(*secure), *skipVerify, *ca, *crt, *key, *username, *password)
	if err != nil {
		log.Fatal(err)
	}
	if err := server.Serve(); err != nil {
		log.Fatal(err)
	}
}
