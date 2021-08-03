package dialout

import (
	"log"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
)

func TestTLS(t *testing.T) {
	// Logging

	Print = log.Print
	Printf = log.Printf
	address := "localhost:8088"

	type server struct {
		insecure   bool
		skipverify bool
		cafile     string
		serverCert string
		serverKey  string
		username   string
		password   string
	}
	type client struct {
		serverName string
		insecure   bool
		skipverify bool
		cafile     string
		clientCert string
		clientKey  string
		username   string
		password   string
	}
	tests := []struct {
		name    string
		server  server
		client  client
		wantErr bool
	}{
		{
			name: "tls setup - set all",
			server: server{
				insecure:   false,
				skipverify: false,
				cafile:     "tls/ca.crt",
				serverCert: "tls/server.crt",
				serverKey:  "tls/server.key",
				username:   "myaccount",
				password:   "mypassword",
			},
			client: client{
				serverName: "hfrnet.com", // server name must be server's SAN (Subject Alternative Name).
				insecure:   false,
				skipverify: false,
				cafile:     "tls/ca.crt",
				clientCert: "tls/client.crt",
				clientKey:  "tls/client.key",
				username:   "myaccount",
				password:   "mypassword",
			},
		},
		{
			name: "tls setup - skip-verify", // in skip-verify mode, server.crt, server.key are only required.
			server: server{
				insecure:   false,
				skipverify: true,
				serverCert: "tls/server.crt",
				serverKey:  "tls/server.key",
				username:   "myaccount",
				password:   "mypassword",
			},
			client: client{
				// serverName: "hfrnet.com",
				insecure:   false,
				skipverify: true,
				username:   "myaccount",
				password:   "mypassword",
			},
		},
		{
			name: "tls setup - insecure",
			server: server{
				insecure:   true,
				skipverify: false,
				// cafile:     "tls/ca.crt",
				// serverCert: "tls/server.crt",
				// serverKey:  "tls/server.key",
				username: "myaccount",
				password: "mypassword",
			},
			client: client{
				// serverName: "hfrnet.com",
				insecure:   true,
				skipverify: false,
				// cafile:     "tls/ca.crt",
				// clientCert: "tls/client.crt",
				// clientKey:  "tls/client.key",
				username: "myaccount",
				password: "mypassword",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewGNMIDialoutServer(
				address, tt.server.insecure, tt.server.skipverify,
				tt.server.cafile, tt.server.serverCert, tt.server.serverKey,
				tt.server.username, tt.server.password)
			if err != nil {
				t.Error(err)
				return
			}
			go server.Serve()
			defer func() {
				server.Close()
				time.Sleep(time.Millisecond * 10)
			}()
			client, err := NewGNMIDialOutClient(
				tt.client.serverName, address, tt.client.insecure, tt.client.skipverify,
				tt.client.cafile, tt.client.clientCert, tt.client.clientKey,
				tt.client.username, tt.client.password, true, "HFR")
			if err != nil {
				t.Error(err)
				return
			}
			defer func() {
				client.Close()
				time.Sleep(time.Millisecond * 10)
			}()
			if err := client.Send(
				[]*gnmi.SubscribeResponse{
					&gnmi.SubscribeResponse{
						Response: &gnmi.SubscribeResponse_SyncResponse{
							SyncResponse: true,
						},
					},
				},
			); err != nil {
				t.Error(err)
				return
			}
			time.Sleep(time.Millisecond * 50)
		})
	}
}

func TestGNMIDialOut(t *testing.T) {
	// Logging
	// Print = log.Print
	// Printf = log.Printf

	address := "localhost:8088"
	insecure := true

	server, err := NewGNMIDialoutServer(address, insecure, false, "", "", "", "", "")
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		server.Close()
		time.Sleep(time.Millisecond * 10)
	}()
	go server.Serve()
	client, err := NewGNMIDialOutClient("", address, insecure, false, "", "", "", "", "", true, "HFR")
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		client.Close()
		time.Sleep(time.Millisecond * 10)
	}()
	if err := client.Send(
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
	time.Sleep(time.Millisecond * 50)
}

func TestGNMIDialOutErr(t *testing.T) {
	// Logging
	Print = log.Print
	Printf = log.Printf

	address := "localhost:8088"
	insecure := true

	server, err := NewGNMIDialoutServer(address, insecure, false, "", "", "", "", "")
	if err != nil {
		t.Error(err)
		return
	}
	// defer func() {
	// 	server.Close()
	// 	time.Sleep(time.Millisecond * 10)
	// }()
	go server.Serve()
	time.Sleep(time.Second * 1)

	client, err := NewGNMIDialOutClient("", address, insecure, false, "", "", "", "", "", true, "HFR")
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		client.Close()
		time.Sleep(time.Millisecond * 10)
	}()

	time.Sleep(time.Second * 1)
	server.Close()
	server = nil
	time.Sleep(time.Second * 1)

	server, err = NewGNMIDialoutServer(address, insecure, false, "", "", "", "", "")
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		server.Close()
		time.Sleep(time.Millisecond * 10)
	}()
	go server.Serve()
	time.Sleep(time.Second * 1)

	if err := client.Send(
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
	time.Sleep(time.Second * 1)
}

func TestGNMIDialOutControlSession(t *testing.T) {
	// Logging
	// Print = log.Print
	// Printf = log.Printf

	address := "localhost:8088"
	insecure := true
	wg := new(sync.WaitGroup)
	quit := make(chan struct{})

	server, err := NewGNMIDialoutServer(address, insecure, false, "", "", "", "", "")
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		server.Close()
		time.Sleep(time.Millisecond * 10)
	}()
	go server.Serve()
	client, err := NewGNMIDialOutClient("", address, insecure, false, "", "", "", "", "", true, "HFR")
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		client.Close()
		time.Sleep(time.Millisecond * 10)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-quit:
				return
			default:
				if err := client.Send(
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
					return
				}
			}
			time.Sleep(time.Second)
		}
	}()
	time.Sleep(time.Second * 10)

	var interval int64 = 1000000000 * 5 //5sec
	if list := server.GetSessionInfo(); len(list) < 0 {
		t.Error("there's no connection between server and client")
		return
	} else {
		for key, _ := range list {
			if err := server.PauseSession(key); err != nil {
				t.Error(err)
				return
			}

			if err := server.RestartSession(key); err != nil {
				t.Error(err)
				return
			}

			if err := server.IntervalPauseSession(key, interval); err != nil {
				t.Error(err)
				return
			}
		}
	}

	close(quit)
	wg.Wait()
	time.Sleep(time.Second * 1)
}
