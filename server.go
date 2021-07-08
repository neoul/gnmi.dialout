package dialout

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"sync"

	pb "github.com/neoul/gnmi.dialout/proto/dialout"
	"github.com/neoul/open-gnmi/utilities/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

var sessionCount int

type GNMIDialoutServer struct {
	pb.UnimplementedGNMIDialOutServer
	GRPCServer *grpc.Server
	Listener   net.Listener

	stream     map[int]pb.GNMIDialOut_PublishServer
	waitgroup  map[int]*sync.WaitGroup
	stopSignal map[int]chan int64
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
		ss <- 0
	}
}

func (server *GNMIDialoutServer) RestartSession(sessionid int) {
	ss, ok := server.stopSignal[sessionid]
	if ok {
		ss <- -1
	}
}

func (server *GNMIDialoutServer) IntervalPauseSession(sessionid int, interval int64) {
	ss, ok := server.stopSignal[sessionid]
	if ok {
		if interval > 0 {
			ss <- interval
		}
	}
}

func (server *GNMIDialoutServer) GetSessionInfo(data []string) []string {
	for i := 1; i < sessionCount+1; i++ {
		if server.stream[i] == nil {
			break
		}
		meta, ok := GetMetadata(server.stream[i].Context())
		if !ok {
			continue
		}
		peer := fmt.Sprintf("%s:session=%d", meta["peer"], i)
		data = append(data, peer)
	}
	return data
}

func GetMetadata(ctx context.Context) (map[string]string, bool) {
	m := map[string]string{}
	headers, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return m, false
	}
	for k, v := range headers {
		k := strings.Trim(k, ":")
		m[k] = v[0]
	}
	p, ok := peer.FromContext(ctx)
	if ok {
		m["protocol"] = p.Addr.Network()
		m["peer"] = p.Addr.String()
		index := strings.LastIndex(p.Addr.String(), ":")
		m["peer-address"] = p.Addr.String()[:index]
		m["peer-port"] = p.Addr.String()[index+1:]
	}
	// fmt.Println("metadata", m)
	return m, true
}

// Close session
func sessionClose(server *GNMIDialoutServer, sessionid int) {
	close(server.stopSignal[sessionid])
	delete(server.stream, sessionid)
	delete(server.waitgroup, sessionid)
	delete(server.stopSignal, sessionid)
	LogPrintf("gnmi.dialout.server.session[%d].close.complete", sessionid)
}

// Receive session
func sessionRecv(server *GNMIDialoutServer, sessionid int) {
	defer server.waitgroup[sessionid].Done()
	if _, ok := server.stream[sessionid]; !ok {
		LogPrintf("gnmi.dialout.server.session[%d].recv.close", sessionid)
		return
	}
	for {
		response, err := server.stream[sessionid].Recv()
		if err != nil {
			LogPrintf("gnmi.dialout.server.session[%d].recv.err=%v", sessionid, err)
			return
		}
		LogPrintf("gnmi.dialout.server.session[%d].recv.msg=%s", sessionid, response)
	}
}

// Send session
func sessionSend(server *GNMIDialoutServer, sessionid int) {
	defer server.waitgroup[sessionid].Done()
	if _, ok := server.stream[sessionid]; !ok {
		LogPrintf("gnmi.dialout.server.session[%d].close", sessionid)
		return
	}
	for {
		select {
		case stop := <-server.stopSignal[sessionid]:
			request := buildPublishResponse(stop)
			if request == nil {
				LogPrintf("gnmi.dialout.server.session[%d].stop-signal.error=%s", sessionid, "not support range")
				return
			}
			if err := server.stream[sessionid].Send(request); err != nil {
				LogPrintf("gnmi.dialout.server.session[%d].stop-signal.error=%s", sessionid, err)
				return
			}
			if stop == 0 {
				LogPrintf("gnmi.dialout.server.session[%d].stop-signal.stop", sessionid)
			} else if stop == -1 {
				LogPrintf("gnmi.dialout.server.session[%d].stop-signal.restart", sessionid)
			} else if stop > 0 {
				LogPrintf("gnmi.dialout.server.session[%d].stop-signal.stop-interval=%v", sessionid, stop)
			}
		}
	}
}

func (s *GNMIDialoutServer) Publish(stream pb.GNMIDialOut_PublishServer) error {
	meta, ok := GetMetadata(stream.Context())
	if !ok {
		return status.Errorf(codes.InvalidArgument, "no metadata")
	}
	username := meta["username"]
	password := meta["password"]
	peer := meta["peer"]

	wg := new(sync.WaitGroup)
	sessionCount++
	sessionid := sessionCount
	stopSignal := make(chan int64)
	s.stream[sessionid] = stream
	s.waitgroup[sessionid] = wg
	s.stopSignal[sessionid] = stopSignal
	LogPrintf("gnmi.dialout.server.session[%d].started addr=%s,username=%s,password=%s", sessionid, peer, username, password)

	// Close publish session
	defer sessionClose(s, sessionid)

	// Receive publish message from client
	s.waitgroup[sessionid].Add(1)
	go sessionRecv(s, sessionid)

	// Send publish message to client for control session
	s.waitgroup[sessionid].Add(1)
	go sessionSend(s, sessionid)

	s.waitgroup[sessionid].Wait()

	return nil
}

// NewGNMIDialoutServer creates new gnmi dialout server
func NewGNMIDialoutServer(address string, insecure bool, skipverify bool, cafile string,
	serverCert string, serverKey string, username string, password string) (*GNMIDialoutServer, error) {
	LogPrintf("gnmi.dialout.server.started")
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
		waitgroup:  make(map[int]*sync.WaitGroup),
		stopSignal: make(map[int]chan int64),
	}
	pb.RegisterGNMIDialOutServer(dialoutServer.GRPCServer, dialoutServer)
	return dialoutServer, nil
}

func buildPublishResponse(stop int64) *pb.PublishResponse {
	if stop == 0 {
		return &pb.PublishResponse{
			Request: &pb.PublishResponse_Stop{
				Stop: true,
			},
		}
	} else if stop == -1 {
		return &pb.PublishResponse{
			Request: &pb.PublishResponse_Restart{
				Restart: true,
			},
		}
	} else if stop > 0 {
		return &pb.PublishResponse{
			Request: &pb.PublishResponse_StopInterval{
				StopInterval: stop,
			},
		}
	}
	return nil
}

// LoadCA loads Root CA from file.
func LoadCAFromFile(cafile string) (*x509.CertPool, error) {
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

func LoadCA(ca []byte) (*x509.CertPool, error) {
	var certPool *x509.CertPool
	// Server runs without certpool if cafile is not configured.
	// creds, err := credentials.NewServerTLSFromFile(certfile, keyfile)
	if len(ca) > 0 {
		certPool = x509.NewCertPool()
		if ok := certPool.AppendCertsFromPEM([]byte(ca)); !ok {
			return nil, errors.New("failed to append ca certificate")
		}
	}
	return certPool, nil
}

// LoadCertificates loads certificates from file.
func LoadCertificatesFromFile(certfile, keyfile string) ([]tls.Certificate, error) {
	if certfile == "" && keyfile == "" {
		return []tls.Certificate{}, nil
	}
	certificate, err := tls.LoadX509KeyPair(certfile, keyfile)
	if err != nil {
		return nil, err
	}
	return []tls.Certificate{certificate}, nil
}

// LoadCertificates loads certificates from file.
func LoadCertificates(cert, key []byte) ([]tls.Certificate, error) {
	if len(cert) == 0 && len(key) == 0 {
		return []tls.Certificate{}, nil
	}
	certificate, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}
	return []tls.Certificate{certificate}, nil
}

func ServerCredentials(cafile, certfile, keyfile string, skipVerifyTLS, insecure bool) ([]grpc.ServerOption, error) {
	if insecure {
		return nil, nil
	}
	certPool, err := LoadCAFromFile(cafile)
	if err != nil {
		return nil, fmt.Errorf("ca loading failed: %v", err)
	}

	certificates, err := LoadCertificatesFromFile(certfile, keyfile)
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
