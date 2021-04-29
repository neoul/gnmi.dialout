package main

import (
	"log"
	"sync"

	"github.com/robot303/gnmi.dialout/client"
	"github.com/robot303/gnmi.dialout/server"
)

func main() {
	wg := new(sync.WaitGroup)
	wg.Add(2)

	go func() {
		defer wg.Done()
		listenAddr := "localhost:8088"
		tlsEnable := false
		tlsFilePath := ""
		keyFilePath := ""
		err := server.NewDialOutServer(listenAddr, tlsEnable, tlsFilePath, keyFilePath)
		if err != nil {
			log.Printf("failed to create new dial-out client: %v", err)
			return
		}
	}()

	go func() {
		defer wg.Done()
		serverAddr := "localhost:8088"
		tlsEnable := false
		tlsFilePath := ""
		client, err := client.NewDialOutClient(serverAddr, tlsEnable, tlsFilePath)
		if err != nil {
			log.Printf("failed to create new dial-out client: %v", err)
			return
		}

		client.NewPublish()
	}()

	wg.Wait()
}
