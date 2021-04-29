package dialout

import (
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
)

func TestGNMIDialOut(t *testing.T) {
	listenAddr := "localhost:8088"
	tlsEnable := false
	tlsFilePath := ""
	keyFilePath := ""
	server, err := NewGNMIDialoutServer(listenAddr, tlsEnable, tlsFilePath, keyFilePath)
	if err != nil {
		t.Error(err)
		return
	}
	go server.Serve()

	client, err := NewGNMIDialOutClient(listenAddr, tlsEnable, tlsFilePath)
	if err != nil {
		t.Error(err)
		return
	}
	if err := client.SendMessage(
		[]*gnmi.SubscribeResponse{
			&gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_SyncResponse{
					SyncResponse: true,
				},
			},
			&gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{
					Update: &gnmi.Notification{
						Timestamp: 0,
						Prefix: &gnmi.Path{
							Elem: []*gnmi.PathElem{
								&gnmi.PathElem{
									Name: "interfaces",
								},
								&gnmi.PathElem{
									Name: "interface",
									Key: map[string]string{
										"name": "1/1",
									},
								},
							},
						},
						Alias: "#1/1",
					},
				},
			},
			&gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{
					Update: &gnmi.Notification{
						Timestamp: 0,
						Prefix: &gnmi.Path{
							Elem: []*gnmi.PathElem{
								&gnmi.PathElem{
									Name: "#1/1",
								},
							},
						},
						Update: []*gnmi.Update{
							&gnmi.Update{
								Path: &gnmi.Path{
									Elem: []*gnmi.PathElem{
										&gnmi.PathElem{
											Name: "state",
										},
										&gnmi.PathElem{
											Name: "counters",
										},
										&gnmi.PathElem{
											Name: "in-pkts",
										},
									},
								},
								Val: &gnmi.TypedValue{
									Value: &gnmi.TypedValue_UintVal{
										UintVal: 100,
									},
								},
							},
						},
					},
				},
			},
		},
	); err != nil {
		t.Error(err)
		return
	}
	// give the time to send
	time.Sleep(time.Millisecond * 10)
	client.Close()
	server.GRPCServer.GracefulStop()
}
