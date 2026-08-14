//go:build integration

package cache

import (
	"os"
	"testing"
	"time"
)

func testRedisAddr() string {
	if addr := os.Getenv("TEST_REDIS_ADDR"); addr != "" {
		return addr
	}
	return "localhost:6379"
}

func TestConnectRedis_Success(t *testing.T) {
	client, err := ConnectRedis(testRedisAddr(), 5*time.Second, 1, 0)
	if err != nil {
		t.Fatalf("did not expect an error, got: %v", err)
	}
	defer client.Close()
}

func TestConnectRedis_Unreachable(t *testing.T) {
	_, err := ConnectRedis("127.0.0.1:1", time.Second, 1, 100*time.Millisecond)
	if err == nil {
		t.Error("expected an error connecting to an unreachable address")
	}
}
