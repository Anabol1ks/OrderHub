module auth-service

go 1.25.1

require (
	github.com/joho/godotenv v1.5.1
	orderhub-utils-go v0.0.0
)

require (
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
)

replace orderhub-utils-go => ../pkg
