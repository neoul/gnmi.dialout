package dialout

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	pb "github.com/neoul/gnmi.dialout/proto/dialout"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
	ctxCancel context.CancelFunc
}

func (client *GNMIDialOutClient) String() string {
	return "client[" + strconv.Itoa(client.Clientid) + "]"
}

func (client *GNMIDialOutClient) Close() {
	client.ctxCancel()
	close(client.respchan)
	client.respchan = nil
	client.conn.Close()
	client.waitgroup.Wait()
	client.waitgroup = nil
	LogPrintf("gnmi.dialout.%v.closed", client)
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
				LogPrintf("gnmi.dialout.%v.recv.closed", client)
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
			LogPrintf("gnmi.dialout.%v.send.shutdown", client)
			client.CloseSend()
			return
		}
		if err := client.Send(subscribeResponse); err != nil {
			LogPrintf("gnmi.dialout.%v.send.err=%v", client, err)
			client.CloseSend()
			return
		}
		LogPrintf("gnmi.dialout.%v.send.msg=%v", client, subscribeResponse)
	}
}

// serverName is used to verify the hostname of the server certificate unless skipverify is given.
// The serverName is also included in the client's handshake to support virtual hosting unless it is an IP address.
func NewGNMIDialOutClient(serverName, serverAddress string, insecure bool, skipverify bool, caCrt string,
	clientCert string, clientKey string, username string, password string, loadCertFromFiles bool) (*GNMIDialOutClient, error) {
	clientCount++

	opts, err := ClientCredentials(serverName, caCrt, clientCert, clientKey, skipverify, insecure, loadCertFromFiles)
	if err != nil {
		err := fmt.Errorf("gnmi.dialout.client[%v].credential.err=%v", clientCount, err)
		LogPrint(err)
		return nil, err
	}
	if !insecure {
		opts = append(opts, UserCredentials(username, password)...)
	}
	// opts = append(opts, grpc.WithBlock())
	// [FIXME] grpc.DialContext vs grpc.Dial
	conn, err := grpc.Dial(serverAddress, opts...)
	if err != nil {
		err := fmt.Errorf("gnmi.dialout.client[%v].dial.err=%v", clientCount, err)
		LogPrint(err)
		return nil, err
	}

	pbclient := pb.NewGNMIDialOutClient(conn)
	if pbclient == nil {
		err := fmt.Errorf("gnmi.dialout.client[%v].create.err", clientCount)
		LogPrint(err)
		return nil, err
	}

	client := &GNMIDialOutClient{
		GNMIDialOutClient: pbclient,
		conn:              conn,
		respchan:          make(chan *gnmi.SubscribeResponse, 256),
		waitgroup:         new(sync.WaitGroup),
		Clientid:          clientCount,
	}

	ctx, cancel := context.WithCancel(context.Background())
	client.ctxCancel = cancel
	stream, err := client.Publish(ctx)
	if err != nil {
		err := fmt.Errorf("gnmi.dialout.%v.stream.err=%v", client, err)
		LogPrint(err)
		return nil, err
	}
	client.GNMIDialOut_PublishClient = stream

	// Receive publish messages from server
	client.waitgroup.Add(1)
	go recv(client)

	// Send publish messages to server
	client.waitgroup.Add(1)
	go send(client)
	LogPrintf("gnmi.dialout.%v.created", client)
	return client, nil
}

// ClientCredentials generates gRPC DialOptions for existing credentials.
func ClientCredentials(serverName string, ca, clientCrt, clientKey string, skipVerifyTLS, insecure, isfile bool) ([]grpc.DialOption, error) {
	var opts []grpc.DialOption
	if insecure {
		opts = append(opts, grpc.WithInsecure())
	} else {
		tlsConfig := &tls.Config{}
		if skipVerifyTLS {
			tlsConfig.InsecureSkipVerify = true
		} else {
			var err error
			var certPool *x509.CertPool
			if isfile {
				certPool, err = LoadCAFromFile(ca)
			} else {
				certPool, err = LoadCA([]byte(ca))
			}
			if err != nil {
				return nil, fmt.Errorf("ca loading failed: %v", err)
			}
			var certificates []tls.Certificate
			if isfile {
				certificates, err = LoadCertificatesFromFile(clientCrt, clientKey)
			} else {
				certificates, err = LoadCertificates([]byte(clientCrt), []byte(clientKey))
			}
			if err != nil {
				return nil, fmt.Errorf("client certificates loading failed: %v", err)
			}
			tlsConfig.ServerName = serverName
			tlsConfig.Certificates = certificates
			tlsConfig.RootCAs = certPool
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		// grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, ""))
	}

	return opts, nil
}

type userCredentials struct {
	username string
	password string
}

func (uc *userCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"username": uc.username,
		"password": uc.password,
	}, nil
}

func (uc *userCredentials) RequireTransportSecurity() bool {
	return true
}

// UserCredentials generates gRPC DialOptions for user authentication.
func UserCredentials(username, password string) []grpc.DialOption {
	if username != "" {
		uc := &userCredentials{
			username: username,
			password: password,
		}
		return []grpc.DialOption{grpc.WithPerRPCCredentials(uc)}
	}
	return nil
}
