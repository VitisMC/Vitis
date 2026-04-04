package storage

import "testing"

func TestMemoryKVSetGet(t *testing.T) {
	kv := NewMemoryKV()
	kv.Set("key1", []byte("value1"))
	v, ok := kv.Get("key1")
	if !ok || string(v) != "value1" {
		t.Fatalf("Get = %q, %v", v, ok)
	}
}

func TestMemoryKVMissing(t *testing.T) {
	kv := NewMemoryKV()
	_, ok := kv.Get("missing")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestMemoryKVDelete(t *testing.T) {
	kv := NewMemoryKV()
	kv.Set("k", []byte("v"))
	if !kv.Delete("k") {
		t.Fatal("expected true")
	}
	if kv.Has("k") {
		t.Fatal("expected deleted")
	}
}

func TestMemoryKVKeys(t *testing.T) {
	kv := NewMemoryKV()
	kv.Set("a", []byte("1"))
	kv.Set("b", []byte("2"))
	keys := kv.Keys()
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
}

func TestMemoryKVLen(t *testing.T) {
	kv := NewMemoryKV()
	if kv.Len() != 0 {
		t.Fatal("expected 0")
	}
	kv.Set("x", []byte("y"))
	if kv.Len() != 1 {
		t.Fatal("expected 1")
	}
}

func TestMemoryKVCopyIsolation(t *testing.T) {
	kv := NewMemoryKV()
	data := []byte("original")
	kv.Set("k", data)
	data[0] = 'X'
	v, _ := kv.Get("k")
	if string(v) != "original" {
		t.Fatal("expected isolation from original slice")
	}
	v[0] = 'Y'
	v2, _ := kv.Get("k")
	if string(v2) != "original" {
		t.Fatal("expected isolation from returned slice")
	}
}

func TestMemoryKVInterface(t *testing.T) {
	var kv KV = NewMemoryKV()
	kv.Set("test", []byte("data"))
	if kv.Len() != 1 {
		t.Fatal("interface assertion failed")
	}
	kv.Close()
}
