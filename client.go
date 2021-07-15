package dialout

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	nokiapb "github.com/karimra/sros-dialout"
	pb "github.com/neoul/gnmi.dialout/proto/dialout"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var clientCount int

type GNMIDialOutClient struct {
	Clientid   int
	StopSingal time.Duration // -1: start, 0: stop, 0>: stop interval
	Error      error

	client      pb.GNMIDialOutClient
	stream      pb.GNMIDialOut_PublishClient
	nokiaclient nokiapb.DialoutTelemetryClient
	nokiastream nokiapb.DialoutTelemetry_PublishClient
	conn        *grpc.ClientConn
	respchan    chan *gnmi.SubscribeResponse
	waitgroup   *sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	protocol    string
}

func (client *GNMIDialOutClient) String() string {
	return "client[" + strconv.Itoa(client.Clientid) + "]"
}

func (client *GNMIDialOutClient) Close() {
	if client == nil {
		return
	}
	if client.respchan != nil {
		close(client.respchan)
		client.respchan = nil
	}
	if client.waitgroup != nil {
		client.waitgroup.Wait()
		client.waitgroup = nil
	}
	if client.conn != nil {
		client.conn.Close()
		client.conn = nil
	}
	LogPrintf("gnmi.dialout.%v.closed", client)
}

func (client *GNMIDialOutClient) Send(responses []*gnmi.SubscribeResponse) error {
	if client == nil || client.respchan == nil {
		return fmt.Errorf("gnmi dial-out publish channel closed")
	}
	for i := range responses {
		client.respchan <- responses[i]
	}
	return nil
}

func (client *GNMIDialOutClient) Channel() chan *gnmi.SubscribeResponse {
	if client == nil || client.respchan == nil {
		return nil
	}
	return client.respchan
}

func currentTime() time.Duration {
	return time.Duration(time.Now().UnixNano())
}

func compareTime(client *GNMIDialOutClient) bool {
	if client.StopSingal == 0 {
		return true
	} else if client.StopSingal == -1 {
		return false
	} else if client.StopSingal > 0 {
		if currentTime() > client.StopSingal {
			return false
		} else {
			return true
		}
	}

	return false
}

func recv(client *GNMIDialOutClient) {
	defer client.waitgroup.Done()
	for {
		if client.stream == nil {
			LogPrintf("gnmi.dialout.%v.recv.null.stream", client)
			return
		}
		LogPrintf("gnmi.dialout.%v.recv.started", client)
		publishResponse, err := client.stream.Recv()
		if err != nil {
			LogPrintf("gnmi.dialout.%v.recv.closed.err=%v", client, err)
			return
		}
		switch msg := publishResponse.GetRequest().(type) {
		case *pb.PublishResponse_Stop:
			client.StopSingal = 0
		case *pb.PublishResponse_Restart:
			client.StopSingal = -1
		case *pb.PublishResponse_StopInterval:
			client.StopSingal = time.Duration(msg.StopInterval) + currentTime()
		}
	}
}

func send(client *GNMIDialOutClient) {
	defer client.waitgroup.Done()
	for {
		subscribeResponse, ok := <-client.respchan
		if !ok {
			if client.stream != nil {
				client.stream.CloseSend()
				client.stream = nil
				LogPrintf("gnmi.dialout.%v.send.closed", client)
			}
			if client.cancel != nil {
				client.cancel()
				client.cancel = nil
			}
			LogPrintf("gnmi.dialout.%v.send.shutdown", client)
			return
		}
		if compareTime(client) {
			LogPrintf("gnmi.dialout.%v.send.stop=%v", client, client.StopSingal)
			continue
		}
		client.Error = io.EOF
		if client.stream != nil {
			client.Error = client.stream.Send(subscribeResponse)
		}
		if client.Error == io.EOF {
			if client.stream != nil {
				client.stream.CloseSend()
				client.stream = nil
				if client.cancel != nil {
					client.cancel()
					client.cancel = nil
				}
				LogPrintf("gnmi.dialout.%v.send.canceled.old.stream", client)
			}
			client.ctx, client.cancel = context.WithCancel(context.Background())
			client.stream, client.Error = client.client.Publish(client.ctx)
			if client.Error == nil {
				LogPrintf("gnmi.dialout.%v.send.(re)started", client)
				client.waitgroup.Add(1)
				go recv(client)
				client.Error = client.stream.Send(subscribeResponse)
			}
		}
		if client.Error != nil {
			client.Error = fmt.Errorf("gnmi.dialout.%v.send.err=%v", client, client.Error)
			if client.stream != nil {
				client.stream.CloseSend()
				client.stream = nil
				LogPrintf("gnmi.dialout.%v.send.closed", client)
			}
			if client.cancel != nil {
				client.cancel()
				client.cancel = nil
			}
		}
	}
}

func send_nokia(client *GNMIDialOutClient) {
	defer client.waitgroup.Done()
	for {
		subscribeResponse, ok := <-client.respchan
		if !ok {
			if client.nokiastream != nil {
				client.nokiastream.CloseSend()
				client.nokiastream = nil
				LogPrintf("gnmi.dialout.%v.send.closed", client)
			}
			if client.cancel != nil {
				client.cancel()
				client.cancel = nil
			}
			LogPrintf("gnmi.dialout.%v.send.shutdown", client)
			return
		}
		client.Error = io.EOF
		if client.nokiastream != nil {
			client.Error = client.nokiastream.Send(subscribeResponse)
		}
		if client.Error == io.EOF {
			if client.nokiastream != nil {
				client.nokiastream.CloseSend()
				client.nokiastream = nil
				if client.cancel != nil {
					client.cancel()
					client.cancel = nil
				}
				LogPrintf("gnmi.dialout.%v.send.canceled.old.stream", client)
			}
			client.ctx, client.cancel = context.WithCancel(context.Background())
			client.nokiastream, client.Error = client.nokiaclient.Publish(client.ctx)
			if client.Error == nil {
				LogPrintf("gnmi.dialout.%v.send.(re)started", client)
				client.Error = client.nokiastream.Send(subscribeResponse)
			}
		}
		if client.Error != nil {
			client.Error = fmt.Errorf("gnmi.dialout.%v.send.err=%v", client, client.Error)
			if client.nokiastream != nil {
				client.nokiastream.CloseSend()
				client.nokiastream = nil
				LogPrintf("gnmi.dialout.%v.send.closed", client)
			}
			if client.cancel != nil {
				client.cancel()
				client.cancel = nil
			}
		}
	}
}

// serverName is used to verify the hostname of the server certificate unless skipverify is given.
// The serverName is also included in the client's handshake to support virtual hosting unless it is an IP address.
func NewGNMIDialOutClient(serverName, serverAddress string, insecure bool, skipverify bool, caCrt string,
	clientCert string, clientKey string, username string, password string, loadCertFromFiles bool, protocol string) (*GNMIDialOutClient, error) {
	var pbclient pb.GNMIDialOutClient = nil            //Default
	var npbclient nokiapb.DialoutTelemetryClient = nil //Nokia
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
	// fmt.Println(conn.Target(), conn.GetState())

	if strings.Compare(protocol, "NOKIA") == 0 {
		npbclient = nokiapb.NewDialoutTelemetryClient(conn)
		if npbclient == nil {
			err := fmt.Errorf("gnmi.dialout.client[%v].create.err", clientCount)
			LogPrint(err)
			return nil, err
		}
	} else {
		pbclient = pb.NewGNMIDialOutClient(conn)
		if pbclient == nil {
			err := fmt.Errorf("gnmi.dialout.client[%v].create.err", clientCount)
			LogPrint(err)
			return nil, err
		}
		if len(protocol) <= 0 {
			protocol = "HFR"
		}
	}

	client := &GNMIDialOutClient{
		client:      pbclient,
		nokiaclient: npbclient,
		StopSingal:  time.Duration(-2),
		conn:        conn,
		respchan:    make(chan *gnmi.SubscribeResponse, 32),
		waitgroup:   new(sync.WaitGroup),
		Clientid:    clientCount,
		protocol:    protocol,
	}

	// Send publish messages to server
	client.waitgroup.Add(1)
	if strings.Compare(protocol, "NOKIA") == 0 {
		go send_nokia(client)
	} else {
		go send(client)
	}
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
