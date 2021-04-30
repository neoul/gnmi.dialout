package dialout

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"sync"

	pb "github.com/neoul/gnmi.dialout/proto/dialout"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var sessionCount int

type GNMIDialoutServer struct {
	pb.UnimplementedGNMIDialOutServer
	GRPCServer *grpc.Server
	Listener   net.Listener

	stream     map[int]pb.GNMIDialOut_PublishServer
	stopSignal map[int]chan bool
}

func (server *GNMIDialoutServer) Close() error {
	// server.GRPCServer.GracefulStop()
	server.GRPCServer.Stop()
	// for i := range server.stream {
	// }

	// Listener seems to be closed ahead.
	// err := server.Listener.Close()
	// if err != nil {
	// 	err := fmt.Errorf("gnmi.dialout.server.close.err=%v", err)
	// 	LogPrint(err)
	// }
	return nil
}

func (server *GNMIDialoutServer) Serve() error {
	if err := server.GRPCServer.Serve(server.Listener); err != nil {
		server.GRPCServer.Stop()
		err := fmt.Errorf("gnmi.dialout.server.serve.err=%v", err)
		LogPrint(err)
		return err
	}
	return nil
}

func (server *GNMIDialoutServer) PauseSession(sessionid int) {
	ss, ok := server.stopSignal[sessionid]
	if ok {
		ss <- true
	}
}

func (server *GNMIDialoutServer) RestartSession(sessionid int) {
	ss, ok := server.stopSignal[sessionid]
	if ok {
		ss <- false
	}
}

func (s *GNMIDialoutServer) Publish(stream pb.GNMIDialOut_PublishServer) error {
	wg := new(sync.WaitGroup)
	wg.Add(1)
	sessionCount++
	sessionid := sessionCount
	stopSignal := make(chan bool)
	s.stream[sessionid] = stream
	s.stopSignal[sessionid] = stopSignal
	defer func() {
		close(stopSignal)
		delete(s.stream, sessionid)
		delete(s.stopSignal, sessionid)
		LogPrintf("gnmi.dialout.server.session[%d].close.complete", sessionid)
		wg.Wait()
	}()

	go func() {
		defer wg.Done()
		for {
			stop, ok := <-stopSignal
			if !ok {
				LogPrintf("gnmi.dialout.server.session[%d].stop-signal.closed", sessionid)
				return
			}
			request := buildPublishResponse(stop)
			if err := stream.Send(request); err != nil {
				LogPrintf("gnmi.dialout.server.session[%d].stop-signal.error=%s", sessionid, err)
				return
			}
			if stop {
				LogPrintf("gnmi.dialout.server.session[%d].stop-signal.stop", sessionid)
			} else {
				LogPrintf("gnmi.dialout.server.session[%d].stop-signal.restart", sessionid)
			}
		}
	}()

	for {
		response, err := stream.Recv()
		if err != nil {
			return err
		}
		LogPrintf("gnmi.dialout.server.recv.msg=%s", response)
	}
}

// NewGNMIDialoutServer creates new gnmi dialout server
func NewGNMIDialoutServer(address string, insecure bool, skipverify bool, cafile string,
	serverCert string, serverKey string, username string, password string) (*GNMIDialoutServer, error) {

	listener, err := net.Listen("tcp", address)
	if err != nil {
		err := fmt.Errorf("gnmi.dialout.server.listen.err=%s", err)
		LogPrint(err)
		return nil, err
	}

	opts, err := ServerCredentials(cafile, serverCert, serverKey, skipverify, insecure)
	if err != nil {
		err := fmt.Errorf("gnmi.dialout.server.credential.err=%s", err)
		LogPrint(err)
		return nil, err
	}

	dialoutServer := &GNMIDialoutServer{
		GRPCServer: grpc.NewServer(opts...),
		Listener:   listener,
		stream:     make(map[int]pb.GNMIDialOut_PublishServer),
		stopSignal: make(map[int]chan bool),
	}
	pb.RegisterGNMIDialOutServer(dialoutServer.GRPCServer, dialoutServer)
	return dialoutServer, nil
}

func buildPublishResponse(stop bool) *pb.PublishResponse {
	if stop {
		return &pb.PublishResponse{
			Request: &pb.PublishResponse_Stop{
				Stop: true,
			},
		}
	}
	return &pb.PublishResponse{
		Request: &pb.PublishResponse_Restart{
			Restart: true,
		},
	}
}

// LoadCA loads Root CA from file.
func LoadCA(cafile string) (*x509.CertPool, error) {
	var certPool *x509.CertPool
	// Server runs without certpool if cafile is not configured.
	// creds, err := credentials.NewServerTLSFromFile(certfile, keyfile)
	if cafile != "" {
		certPool = x509.NewCertPool()
		cabytes, err := ioutil.ReadFile(cafile)
		if err != nil {
			return nil, err
		}
		if ok := certPool.AppendCertsFromPEM(cabytes); !ok {
			return nil, errors.New("failed to append ca certificate")
		}
	}
	return certPool, nil
}

// LoadCertificates loads certificates from file.
func LoadCertificates(certfile, keyfile string) ([]tls.Certificate, error) {
	if certfile == "" && keyfile == "" {
		return []tls.Certificate{}, nil
	}
	certificate, err := tls.LoadX509KeyPair(certfile, keyfile)
	if err != nil {
		return nil, err
	}
	return []tls.Certificate{certificate}, nil
}

func ServerCredentials(cafile, certfile, keyfile string, skipVerifyTLS, insecure bool) ([]grpc.ServerOption, error) {
	if insecure {
		return nil, nil
	}
	certPool, err := LoadCA(cafile)
	if err != nil {
		return nil, fmt.Errorf("ca loading failed: %v", err)
	}

	certificates, err := LoadCertificates(certfile, keyfile)
	if err != nil {
		return nil, fmt.Errorf("server certificates loading failed: %v", err)
	}
	if skipVerifyTLS {
		return []grpc.ServerOption{grpc.Creds(credentials.NewTLS(&tls.Config{
			ClientAuth:   tls.VerifyClientCertIfGiven,
			Certificates: certificates,
			ClientCAs:    certPool,
		}))}, nil
	}
	return []grpc.ServerOption{grpc.Creds(credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: certificates,
		ClientCAs:    certPool,
	}))}, nil
}
