module stock_trader/api-gateway

go 1.23.2

replace stock_trader/common => ../common

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/go-redis/redis/v8 v8.11.5
	github.com/golang-jwt/jwt/v5 v5.2.0
	github.com/google/uuid v1.5.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.18.1
	github.com/sony/gobreaker v0.5.0
	github.com/stretchr/testify v1.8.4
	google.golang.org/grpc v1.60.1
	google.golang.org/protobuf v1.32.0
)
