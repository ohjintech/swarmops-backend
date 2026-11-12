module github.com/swarm-ops/swarmops-backend

go 1.24

require (
	connectrpc.com/connect v1.18.1
	connectrpc.com/cors v0.1.0
	github.com/coder/websocket v1.8.13
	github.com/eclipse/paho.golang v0.22.0
	github.com/redis/go-redis/v9 v9.10.0
	github.com/rs/cors v1.11.1
	github.com/swarm-ops/swarmops-contracts/gen/go v0.0.0
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250324211829-b45e905df463 // indirect
	google.golang.org/grpc v1.73.0 // indirect
)

// Local sibling checkout until swarmops-contracts publishes gen/go/vX.Y.Z.
// Then delete this line and `go get github.com/swarm-ops/swarmops-contracts/gen/go@vX.Y.Z`.
replace github.com/swarm-ops/swarmops-contracts/gen/go => ../swarmops-contracts/gen/go
