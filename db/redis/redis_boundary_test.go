package redis

import "testing"

func TestNilRedisClientSubscriptionAndCloseAreSafe(t *testing.T) {
	var client *RedisClient
	client.PSubscribe("", nil)
	client.Close()
	if err := client.Check(); err == nil {
		t.Fatal("Check on nil client should return an error")
	}

	client = &RedisClient{}
	client.PSubscribe("0", nil)
	client.Close()
	client.Close()
}
