module dizzycoder.xyz/market-data-service

go 1.25.1

require (
	dizzycode.xyz/logger v0.0.0
	dizzycode.xyz/websocket v0.0.0
	github.com/joho/godotenv v1.5.1
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
)

replace dizzycode.xyz/logger => ../../go-packages/logger

replace dizzycode.xyz/websocket => ../../go-packages/websocket
