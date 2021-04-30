# gnmi.dialout

This is gNMI Dial-out Telemetry service definition that reuses the messages of gNMI Subscribe RPC. You can generate the gRPC code stub of the service using the following command.

```bash
cd proto
go generate
```

## Test with grpc log

```bash
GRPC_GO_LOG_SEVERITY_LEVEL=info GRPC_GO_LOG_VERBOSITY_LEVEL=2 go test -v -run TestTLS
```
