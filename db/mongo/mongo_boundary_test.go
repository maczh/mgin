package mongo

import "testing"

func TestUninitializedMongoOperationsReturnErrors(t *testing.T) {
	var client *Mongodb
	if err := client.Check(); err == nil {
		t.Fatal("Check on nil client should return an error")
	}
	if _, err := client.GetConnection(); err == nil {
		t.Fatal("GetConnection on nil client should return an error")
	}
	client.Close()

	client = &Mongodb{}
	if err := client.Check(); err == nil {
		t.Fatal("Check on uninitialized client should return an error")
	}
	if _, err := client.GetConnection(); err == nil {
		t.Fatal("GetConnection on uninitialized client should return an error")
	}
	client.Close()
}
