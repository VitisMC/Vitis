// Package storage provides key-value store abstractions.
// TODO: Wire KV store into world/player data persistence layer.
package storage

// KV is a key-value store interface.
type KV interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte)
	Delete(key string) bool
	Has(key string) bool
	Keys() []string
	Len() int
	Close() error
}
