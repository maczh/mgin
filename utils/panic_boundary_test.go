package utils

import "testing"

func TestNilValuesReceiverIsSafe(t *testing.T) {
	var values *Values
	values.Put("x", 1)
	values.Merge(map[string]any{"x": 2})
	values.Clear()
	if values.Get("x") != nil {
		t.Fatal("Get on nil Values should return nil")
	}
	if got := values.GetAll(); got == nil {
		t.Fatal("GetAll on nil Values should return an empty map")
	}
}

func TestStructMapHandlesPointerInputs(t *testing.T) {
	type sample struct {
		Name string `json:"name"`
	}
	value := &sample{Name: "ok"}
	if got := Struct2Map(value)["name"]; got != "ok" {
		t.Fatalf("Struct2Map pointer result = %#v", got)
	}
	if got := Struct2MapString(value)["name"]; got != "ok" {
		t.Fatalf("Struct2MapString pointer result = %#v", got)
	}
	var nilValue *sample
	if got := Struct2Map(nilValue); len(got) != 0 {
		t.Fatalf("nil pointer result = %#v", got)
	}
}

func TestRingBufferUsesBackingSliceBounds(t *testing.T) {
	buffer := NewRingBuffer[int](2)
	buffer.Size = 10
	buffer.Write(1)
	buffer.Write(2)
	buffer.Write(3)
	if got := buffer.Latest(); got != 3 {
		t.Fatalf("Latest() = %d, want 3", got)
	}
}

func TestNilRSASecurityReturnsErrors(t *testing.T) {
	var rsa *RSASecurity
	if err := rsa.SetPublicKey(""); err == nil {
		t.Fatal("SetPublicKey should reject nil receiver")
	}
	if _, err := rsa.PubKeyENCTYPT(nil); err == nil {
		t.Fatal("PubKeyENCTYPT should reject nil receiver")
	}
	if err := rsa.VerifySignSha256WithRsa("", ""); err == nil {
		t.Fatal("Verify should reject nil receiver")
	}
}

func TestNilSafeMapReceiverIsSafe(t *testing.T) {
	var values *Map[int]
	if values.Load("x") != nil || values.Len() != 0 || values.Map() == nil {
		t.Fatal("nil Map should return empty results")
	}
	values.Store("x", nil)
	values.Delete("x")
	values.DeleteDirectly("x")
	values.LoadAndStore("x", nil)
	values.LoadAndDelete("x")
	values.Range(nil)
	values.Clear()
}
