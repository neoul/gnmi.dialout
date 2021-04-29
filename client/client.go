package client

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/grpc/grpc-go/examples/data"
	"github.com/openconfig/gnmi/proto/gnmi"
	pb "github.com/robot303/gnmi.dialout/proto/dialout"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	serverHostOverride = flag.String("server_host_overrid", "", "The server name used to verify the hostname returned by the TLS handshake")
	ctxType            = flag.String("context_type", "cancel", "Context Type")
	ctxDuration        = flag.Uint("context_duratoin", 0, "Context duration when context type is WithTimeout or WithDeadline")
)

type dialOutClient struct {
	client     *pb.GNMIDialOutClient
	dialoutses *dialOutSession
	connection *grpc.ClientConn
	ctx        context.Context
	ctxcancel  context.CancelFunc
}

type dialOutSession struct {
	respchan  chan *gnmi.SubscribeResponse
	shutdown  chan struct{}
	errchan   chan error
	waitgroup *sync.WaitGroup
}

type contextType string

const (
	withCancel   = contextType("cancel")
	withDeadline = contextType("deadline")
	withTimeout  = contextType("timeout")
)

func recvPublishConf(stream pb.GNMIDialOut_PublishClient, dialOutSes *dialOutSession) {
	defer dialOutSes.waitgroup.Done()
	for {
		select {
		case <-dialOutSes.shutdown:
			log.Printf("receive shutdown command")
			return
		default:
			in, err := stream.Recv()
			if err == io.EOF {
				log.Printf("receive stream shutdown")
				dialOutSes.errchan <- err
				return
			}
			if err != nil {
				log.Printf("failed to receive:: %v", err)
				dialOutSes.errchan <- err
				return
			}

			log.Printf("Got message:: %v", in.String())
			//run Publish config
		}
	}
}

func sendSubscribeResp(stream pb.GNMIDialOut_PublishClient, dialOutSes *dialOutSession) {
	defer dialOutSes.waitgroup.Done()
	for {
		select {
		case <-dialOutSes.shutdown:
			stream.CloseSend()
			return
		case resp, ok := <-dialOutSes.respchan:
			if ok {
				if err := stream.Send(resp); err != nil {
					log.Printf("failed to send:: %v", err)
					dialOutSes.errchan <- err
					stream.CloseSend()
					return
				}
			}
		}
	}
}

func startDialOutSession() (*dialOutSession, error) {
	dialOutSes := &dialOutSession{
		respchan:  make(chan *gnmi.SubscribeResponse, 256),
		shutdown:  make(chan struct{}),
		errchan:   make(chan error),
		waitgroup: new(sync.WaitGroup),
	}

	return dialOutSes, nil
}

func (dialOutSes *dialOutSession) stopDialOutSession() {
	close(dialOutSes.errchan)
	close(dialOutSes.respchan)
	close(dialOutSes.shutdown)
	dialOutSes.waitgroup.Wait()
	dialOutSes.waitgroup = nil
}

func (client *dialOutClient) NewPublish() error {
	dialOutSes, err := startDialOutSession()
	if err != nil {
		return err
	}
	client.dialoutses = dialOutSes

	stream, err := (*client.client).Publish(client.ctx)
	if err != nil {
		return err
	}
	client.dialoutses.waitgroup.Add(2)

	//Receive publish config from server
	go recvPublishConf(stream, client.dialoutses)

	//Send subscribe response to server
	go sendSubscribeResp(stream, client.dialoutses)

	err = <-client.dialoutses.errchan
	log.Printf("receive error:: %v", err)
	client.dialoutses.stopDialOutSession()
	client.ctxcancel()
	return nil
}

func (client *dialOutClient) CloseDialOutClient() {
	client.connection.Close()
	client.dialoutses = nil
	client.connection = nil
	client.client = nil
	client.ctx = nil
	client.ctxcancel = nil
}

func NewDialOutClient(serverAddr string, tls bool, caFilePath string) (*dialOutClient, error) {
	var opts []grpc.DialOption
	var ctx context.Context = nil
	var cancel context.CancelFunc = nil

	if tls {
		if caFilePath == "" {
			caFilePath = data.Path("../../../../../github.com/robot303/gnmi.dialout/tls/client.crt")
		}
		creds, err := credentials.NewClientTLSFromFile(caFilePath, *serverHostOverride)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS crdentials %v", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	opts = append(opts, grpc.WithBlock())
	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial:: %v", err)
	}
	//defer conn.Close() -> Need to run after NewDialOutClient()!!!!!!
	client := pb.NewGNMIDialOutClient(conn)
	if client == nil {
		return nil, fmt.Errorf("failed to create new client")
	}

	ctype := contextType(*ctxType)
	cduration := time.Duration(*ctxDuration)
	if ctype == withCancel {
		ctx, cancel = context.WithCancel(context.Background())
	} else if ctype == withDeadline {
		if cduration > 0 {
			d := time.Now().Add(cduration)
			ctx, cancel = context.WithDeadline(context.Background(), d)
		}
	} else if ctype == withTimeout {
		if cduration > 0 {
			ctx, cancel = context.WithTimeout(context.Background(), cduration)
		}
	}

	if ctx == nil {
		ctx, cancel = context.WithCancel(context.Background())
	}
	if ctx == nil {
		cancel()
		return nil, fmt.Errorf("failed to create new context")
	}

	dialoutclient := &dialOutClient{
		client:     &client,
		connection: conn,
		ctx:        ctx,
		ctxcancel:  cancel,
	}

	return dialoutclient, nil
}

func TestRun() *dialOutClient {
	log.Println("[TestRun] Start")
	serverAddr := "localhost:8088"
	client, err := NewDialOutClient(serverAddr, false, "")
	if err != nil {
		log.Println("[TestRun] Failed to create new dial-out client")
		return nil
	}

	go func() {
		client.NewPublish()
	}()
	log.Println("[TestRun] Done")
	return client
}
