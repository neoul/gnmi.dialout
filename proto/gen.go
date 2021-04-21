package proto

// Generate gnmi dialout telemetry service code
//go:generate sh -c "export PATH=\"$PATH:$GOPATH/bin\" && mkdir -p dl && wget -c https://github.com/protocolbuffers/protobuf/releases/download/v3.15.8/protoc-3.15.8-linux-x86_64.zip -O dl/tmp.zip && cd dl && unzip tmp.zip && cp bin/protoc $GOPATH/bin && cd .. && rm -fR dl && go install google.golang.org/protobuf/cmd/protoc-gen-go && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc && protoc -I=$GOPATH/src -I$GOPATH/src/github.com/neoul/gnmi.dialout/proto --go_out=. --go-grpc_out=. --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative dialout/gnmi.dialout.proto"

// protoc --go_out=. --go_opt=paths=source_relative \
//     --go-grpc_out=. --go-grpc_opt=paths=source_relative \
//     helloworld/helloworld.proto
