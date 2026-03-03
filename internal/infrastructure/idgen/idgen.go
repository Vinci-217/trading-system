package idgen

import (
	"context"
	"fmt"
	"hash/crc32"
	"time"
	
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type IDGenerator struct {
	redis     *redis.Client
	serviceID string
}

func NewIDGenerator(redis *redis.Client, serviceID string) *IDGenerator {
	return &IDGenerator{
		redis:     redis,
		serviceID: serviceID,
	}
}

func (g *IDGenerator) GenerateOrderID(ctx context.Context, userID string) (string, error) {
	datePart := time.Now().Format("20060102")
	servicePart := g.serviceID
	userPart := fmt.Sprintf("%04s", userID[len(userID)-min(4, len(userID)):])
	
	seqKey := fmt.Sprintf("order:seq:%s", datePart)
	seq, err := g.redis.Incr(ctx, seqKey).Result()
	if err != nil {
		return "", err
	}
	
	g.redis.Expire(ctx, seqKey, 48*time.Hour)
	seqPart := fmt.Sprintf("%06d", seq%1000000)
	
	rawID := datePart + servicePart + userPart + seqPart
	checksum := g.calculateChecksum(rawID)
	
	return rawID + checksum, nil
}

func (g *IDGenerator) calculateChecksum(id string) string {
	crc := crc32.ChecksumIEEE([]byte(id))
	return fmt.Sprintf("%02X", crc%256)
}

func (g *IDGenerator) ValidateOrderID(orderID string) bool {
	if len(orderID) != 22 {
		return false
	}
	rawID := orderID[:20]
	checksum := orderID[20:]
	expected := g.calculateChecksum(rawID)
	return checksum == expected
}

func (g *IDGenerator) GenerateTradeID() string {
	return fmt.Sprintf("T%s%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])
}

func (g *IDGenerator) GenerateFlowID(flowType string) string {
	return fmt.Sprintf("F%s%s%s", flowType, time.Now().Format("20060102150405"), uuid.New().String()[:8])
}

func (g *IDGenerator) GenerateTransactionID() string {
	return uuid.New().String()
}

func (g *IDGenerator) GenerateDiscrepancyID() string {
	return fmt.Sprintf("D%s%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])
}
