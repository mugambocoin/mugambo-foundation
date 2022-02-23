package verwatcher

import (
	"github.com/mugambocoin/mugambo-base/kvdb"

	"github.com/mugambocoin/mugambo-foundation/logger"
)

// Store is a node persistent storage working over physical key-value database.
type Store struct {
	mainDB kvdb.Store

	logger.Instance
}

// NewStore creates store over key-value db.
func NewStore(mainDB kvdb.Store) *Store {
	s := &Store{
		mainDB:   mainDB,
		Instance: logger.MakeInstance(),
	}

	return s
}
