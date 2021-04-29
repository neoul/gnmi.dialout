package dialout

import (
	"fmt"
	"net"
	"sync"

	"github.com/grpc/grpc-go/examples/data"
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

func (server *GNMIDialoutServer) Serve() error {
	if err := server.GRPCServer.Serve(server.Listener); err != nil {
		server.GRPCServer.Stop()
		err := fmt.Errorf("gnmi.dialout.server.serve.err=%v", err)
		Print(err)
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
		Printf("gnmi.dialout.server.session[%d].close.complete", sessionid)
		wg.Wait()
	}()

	go func() {
		defer wg.Done()
		for {
			stop, ok := <-stopSignal
			if !ok {
				Printf("gnmi.dialout.server.session[%d].stop-signal.closed", sessionid)
				return
			}
			request := buildPublishResponse(stop)
			if err := stream.Send(request); err != nil {
				Printf("gnmi.dialout.server.session[%d].stop-signal.error=%s", sessionid, err)
				return
			}
			if stop {
				Printf("gnmi.dialout.server.session[%d].stop-signal.stop", sessionid)
			} else {
				Printf("gnmi.dialout.server.session[%d].stop-signal.restart", sessionid)
			}
		}
	}()

	for {
		response, err := stream.Recv()
		if err != nil {
			return err
		}
		Printf("gnmi.dialout.server.recv.msg=%s", response)
	}
}

func NewGNMIDialoutServer(listenAddr string, tls bool, caFilePath string, keyFilePath string) (*GNMIDialoutServer, error) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		err := fmt.Errorf("gnmi.dialout.server.listen.err=%s", err)
		Print(err)
		return nil, err
	}

	var opts []grpc.ServerOption
	if tls {
		if caFilePath == "" {
			caFilePath = data.Path("../../../../../github.com/neoul/gnmi.dialout/tls/server.crt")
		}
		if keyFilePath == "" {
			keyFilePath = data.Path("../../../../../github.com/neoul/gnmi.dialout/tls/server.key")
		}
		creds, err := credentials.NewServerTLSFromFile(caFilePath, keyFilePath)
		if err != nil {
			err := fmt.Errorf("gnmi.dialout.server.creds.err=%v", err)
			Print(err)
			return nil, err
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
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
