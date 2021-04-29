package server

import (
	"flag"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/grpc/grpc-go/examples/data"
	pb "github.com/robot303/gnmi.dialout/proto/dialout"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	dbFile = flag.String("db_file", "", "A data file")
)

type gnmiDialoutServer struct {
	pb.UnimplementedGNMIDialOutServer
	mu sync.Mutex
}

func buildPublishResponse() *pb.PublishResponse {
	restart := true

	publishRes := &pb.PublishResponse{
		Request: &pb.PublishResponse_Restart{
			Restart: restart,
		},
	}

	return publishRes
}

var idx int = 0

func (s *gnmiDialoutServer) Publish(stream pb.GNMIDialOut_PublishServer) error {
	wg := new(sync.WaitGroup)
	shutdown := make(chan bool)
	errc := make(chan error)
	wg.Add(2)

	//Receive subscribe response from client
	go func() {
		defer wg.Done()
		for {
			select {
			case <-shutdown:
				log.Printf("receive shutdown command")
				return
			default:
				in, err := stream.Recv()
				if err == io.EOF {
					log.Printf("receive stream shutdown")
					errc <- err
					return
				}
				if err != nil {
					log.Printf("failed to receive:: %v", err)
					errc <- err
					return
				}

				//Print subscribe response
				s.mu.Lock()
				log.Printf("Got message:: %v", in.String())
				s.mu.Unlock()
			}
		}
	}()

	//Send publish config to client
	var pub bool = false
	go func() {
		defer wg.Done()
		for {
			select {
			case <-shutdown:
				log.Printf("receive shutdown command")
				return
			default:
				if pub {
					out := buildPublishResponse()
					if err := stream.Send(out); err != nil {
						log.Printf("failed to send:: %v", err)
						errc <- err
						return
					}
				}
			}
			time.Sleep(time.Second * 5)
		}
	}()

	err := <-errc
	log.Printf("receive error:: %v", err)
	close(errc)
	close(shutdown)
	wg.Wait()
	return err
}

func newServer() *gnmiDialoutServer {
	s := &gnmiDialoutServer{}
	return s
}

func NewDialOutServer(listenAddr string, tls bool, caFilePath string, keyFilePath string) error {
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Printf("failed to listen: %v", err)
		return err
	}

	var opts []grpc.ServerOption
	if tls {
		if caFilePath == "" {
			caFilePath = data.Path("../../../../../github.com/robot303/gnmi.dialout/tls/server.crt")
		}
		if keyFilePath == "" {
			keyFilePath = data.Path("../../../../../github.com/robot303/gnmi.dialout/tls/server.key")
		}
		creds, err := credentials.NewServerTLSFromFile(caFilePath, keyFilePath)
		if err != nil {
			log.Printf("failed to generate credentials %v", err)
			return err
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}

	grpcServer := grpc.NewServer(opts...)
	gnmiServer := newServer()
	pb.RegisterGNMIDialOutServer(grpcServer, gnmiServer)
	if err := grpcServer.Serve(lis); err != nil {
		log.Printf("failed to serve: %v", err)
		grpcServer.Stop()
		grpcServer = nil
		gnmiServer = nil
		return err
	}

	return nil
}

func TestRun() {
	log.Println("[TestRun] Start")
	listenAddr := "localhost:8088"
	tlsEnable := true
	tlsFilePath := ""
	keyFilePath := ""
	err := NewDialOutServer(listenAddr, tlsEnable, tlsFilePath, keyFilePath)
	if err != nil {
		log.Println("[TestRun] Failed to create new dial-out client")
		return
	}
	log.Println("[TestRun] Done")
}
