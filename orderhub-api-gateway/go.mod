module api-gateway

go 1.25.1

require (
	github.com/joho/godotenv v1.5.1
	google.golang.org/grpc v1.75.1
	orderhub-proto v0.0.0
	orderhub-utils-go v0.0.0
)

require (
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250707201910-8d1bb00bc6a7 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
)

replace orderhub-proto => ../orderhub-proto

replace orderhub-utils-go => ../pkg
