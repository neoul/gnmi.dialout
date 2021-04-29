package dialout

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	pb "github.com/neoul/gnmi.dialout/proto/dialout"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
)

var clientCount int

type GNMIDialOutClient struct {
	Clientid int
	pb.GNMIDialOutClient
	pb.GNMIDialOut_PublishClient
	Stop chan time.Duration // -1: start, 0: stop, 0>: stop interval

	conn      *grpc.ClientConn
	respchan  chan *gnmi.SubscribeResponse
	waitgroup *sync.WaitGroup
}

func (client *GNMIDialOutClient) String() string {
	return "client[" + strconv.Itoa(client.Clientid) + "]"
}

func (client *GNMIDialOutClient) Close() {
	close(client.respchan)
	client.respchan = nil
	client.conn.Close()
	client.waitgroup.Wait()
	client.waitgroup = nil
	Printf("gnmi.dialout.%v.closed", client)
}

func (client *GNMIDialOutClient) SendMessage(message []*gnmi.SubscribeResponse) error {
	if client == nil || client.respchan == nil {
		return fmt.Errorf("gnmi dial-out publish channel closed")
	}
	for i := range message {
		client.respchan <- message[i]
	}
	return nil
}

func recv(client *GNMIDialOutClient) {
	defer client.waitgroup.Done()
	for {
		publishResponse, err := client.Recv()
		if err != nil {
			if err == io.EOF {
				Printf("gnmi.dialout.%v.recv.closed", client)
				return
			}
			return
		}
		switch msg := publishResponse.GetRequest().(type) {
		case *pb.PublishResponse_Stop:
			client.Stop <- 0
		case *pb.PublishResponse_Restart:
			client.Stop <- -1
		case *pb.PublishResponse_StopInterval:
			client.Stop <- time.Duration(msg.StopInterval)
		}
	}
}

func send(client *GNMIDialOutClient) {
	defer client.waitgroup.Done()
	for {
		subscribeResponse, ok := <-client.respchan
		if !ok {
			Printf("gnmi.dialout.%v.send.shutdown", client)
			client.CloseSend()
			return
		}
		if err := client.Send(subscribeResponse); err != nil {
			Printf("gnmi.dialout.%v.send.err=%v", client, err)
			client.CloseSend()
			return
		}
		Printf("gnmi.dialout.%v.send.msg=%v", client, subscribeResponse)
	}
}

func NewGNMIDialOutClient(serverAddr string, tls bool, caFilePath string) (*GNMIDialOutClient, error) {
	clientCount++
	var opts []grpc.DialOption

	// if tls {
	// 	if caFilePath == "" {
	// 		caFilePath = data.Path("../../../../../github.com/neoul/gnmi.dialout/tls/client.crt")
	// 	}
	// 	creds, err := credentials.NewClientTLSFromFile(caFilePath, *serverHostOverride)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to create TLS crdentials %v", err)
	// 	}
	// 	opts = append(opts, grpc.WithTransportCredentials(creds))
	// } else {
	// 	opts = append(opts, grpc.WithInsecure())
	// }
	opts = append(opts, grpc.WithInsecure())
	opts = append(opts, grpc.WithBlock())
	// [FIXME] grpc.DialContext vs grpc.Dial
	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		err := fmt.Errorf("gnmi.dialout.client[%v].dial.err=%v", clientCount, err)
		Print(err)
		return nil, err
	}

	pbclient := pb.NewGNMIDialOutClient(conn)
	if pbclient == nil {
		err := fmt.Errorf("gnmi.dialout.client[%v].create.err", clientCount)
		Print(err)
		return nil, err
	}

	client := &GNMIDialOutClient{
		GNMIDialOutClient: pbclient,
		conn:              conn,
		respchan:          make(chan *gnmi.SubscribeResponse, 256),
		waitgroup:         new(sync.WaitGroup),
		Clientid:          clientCount,
	}

	// [FIXME] need to update context control
	stream, err := client.Publish(context.Background())
	if err != nil {
		err := fmt.Errorf("gnmi.dialout.%v.publish.err", client)
		Print(err)
		return nil, err
	}
	client.GNMIDialOut_PublishClient = stream

	// Receive publish messages from server
	client.waitgroup.Add(1)
	go recv(client)

	// Send publish messages to server
	client.waitgroup.Add(1)
	go send(client)
	Printf("gnmi.dialout.%v.created", client)
	return client, nil
}
