module stock_trader/market-service

go 1.23.2

replace stock_trader/common => ../common

require (
	github.com/shopspring/decimal v1.4.0
	google.golang.org/grpc v1.60.1
	google.golang.org/protobuf v1.32.0
	github.com/redis/go-redis/v9 v9.4.0
	github.com/google/uuid v1.5.0
	github.com/gorilla/websocket v1.5.1
	github.com/stretchr/testify v1.8.4
)
