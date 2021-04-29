package client

import (
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/neoul/gnxi/utilities/test"
	"github.com/openconfig/gnmi/proto/gnmi"
)

func TestClient(t *testing.T) {
	type testsubscribe struct {
		name     string
		msgfile  string
		waittime time.Duration
	}

	tests := []testsubscribe{
		{
			name:     "poll subscription",
			msgfile:  "data/subscribe-poll.prototxt",
			waittime: time.Second * 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := TestRun()
			if client == nil {
				t.Errorf("failed to create new dial-out client")
			}

			testobj, err := test.LoadTestFile(tc.msgfile)
			if err != nil {
				t.Errorf("loading '%s' got error: %v", tc.msgfile, err)
			}

			for i := range testobj {
				if testobj[i].Name == "" || testobj[i].Text == "" {
					continue
				}
				switch {
				case strings.Contains(testobj[i].Name, "SubscribeResponse"):
					resp := &gnmi.SubscribeResponse{}
					if err := proto.UnmarshalText(testobj[i].Text, resp); err != nil {
						t.Fatalf("proto message unmarshaling get error: %v", err)
					}
					client.dialoutses.respchan <- resp
				}
			}
		})
	}
}
