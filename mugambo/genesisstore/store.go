package genesisstore

import (
	"github.com/mugambocoin/mugambo-base/kvdb"
	"github.com/mugambocoin/mugambo-base/kvdb/memorydb"
	"github.com/mugambocoin/mugambo-base/kvdb/table"

	"github.com/mugambocoin/mugambo-foundation/logger"
	"github.com/mugambocoin/mugambo-foundation/utils/rlpstore"
)

// Store is a node persistent storage working over physical key-value database.
type Store struct {
	db kvdb.Store

	table struct {
		Rules kvdb.Store `table:"c"`

		Blocks kvdb.Store `table:"b"`

		EvmAccounts kvdb.Store `table:"a"`
		EvmStorage  kvdb.Store `table:"s"`
		RawEvmItems kvdb.Store `table:"M"`

		Delegations kvdb.Store `table:"d"`
		Metadata    kvdb.Store `table:"m"`
	}

	rlp rlpstore.Helper
	logger.Instance
}

// NewMemStore creates store over memory map.
func NewMemStore() *Store {
	return NewStore(memorydb.New())
}

// NewStore creates store over key-value db.
func NewStore(db kvdb.Store) *Store {
	s := &Store{
		db:       db,
		Instance: logger.MakeInstance(),
		rlp:      rlpstore.Helper{logger.MakeInstance()},
	}

	table.MigrateTables(&s.table, s.db)

	return s
}

// Close leaves underlying database.
func (s *Store) Close() {
	table.MigrateTables(&s.table, nil)

	_ = s.db.Close()
}
