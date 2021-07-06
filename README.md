# gnmi.dialout

This is gNMI Dial-out Telemetry service definition that reuses the messages of gNMI Subscribe RPC.
The gNMI Dial-out Telemetry service is initiated by the device while the gNMI Telemetry is initiated by the collecter using gNMI client.

This repository includes the gNMI Dial-out Telemetry service definition, simple API and a sample test server for the reference implementation and testing.

## Definition

```proto
// Dial-out Telemetry service (gNMIDialOut) defines a telemetry service initiated by the device. 
// (gNMI Telemetry service is initiated by the collector, not the device.)
// The server is implemented at the collector, such that the device can initiate 
// connections to the collector, based on a configured set of telemetry subscriptions.
// The Publish RPC allows the device to send telemetry updates in the form of 
// SubscribeResponse messages, which have the same semantics as in the gNMI Subscribe RPC, 
// to a collector. The collector may optionally return the PublishResponse message 
// to control the flow of the telemetry updates.
service gNMIDialOut {
  rpc Publish(stream SubscribeResponse) returns (stream PublishResponse);
}

// PublishResponse is the message sent within the Publish RPC by the client (collector) 
// to the target. It allows the flow-control of the telemetry update messages.
message PublishResponse {
    oneof request {
        bool stop = 1;             // Stop signal; the target stops sending the update immediately.
        bool restart = 2;          // Restart signal; the target restart sending the update immediately.
        int64 stop_interval = 3;   // Stop interval in nanoseconds; the target doesn't send any of the updates in this interval.
    }
}
```

## API

The API of the dial-out client and server are defined in `client.go` and `server.go` files.
You can check the usage of the API in `server_test.go` file. Please test and review the test file for your gNMI Dial-out service.

```bash
# Test with grpc log
GRPC_GO_LOG_SEVERITY_LEVEL=info GRPC_GO_LOG_VERBOSITY_LEVEL=2 go test -v -run TestTLS
```

## Simple Test Server

The simple test server is implemented in `server/main.go`. This dial-out test server will print all the received Publish RPC (SubscribeResponse) messages to the screen.

```bash
cd server
go run main.go -h
go run main.go
```
