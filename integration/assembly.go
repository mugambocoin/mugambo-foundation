package integration

import (
	"crypto/ecdsa"
	"errors"
	"fmt"

	"github.com/mugambocoin/mugambo-foundation/mugambo"
	"github.com/mugambocoin/mugambo-base/abft"
	"github.com/mugambocoin/mugambo-base/hash"
	"github.com/mugambocoin/mugambo-base/inter/idx"
	"github.com/mugambocoin/mugambo-base/kvdb"
	"github.com/mugambocoin/mugambo-base/kvdb/flushable"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"github.com/mugambocoin/mugambo-foundation/gossip"
	"github.com/mugambocoin/mugambo-foundation/mugambo/genesisstore"
	"github.com/mugambocoin/mugambo-foundation/utils/adapters/vecmt2dagidx"
	"github.com/mugambocoin/mugambo-foundation/vecmt"
)

// GenesisMismatchError is raised when trying to overwrite an existing
// genesis block with an incompatible one.
type GenesisMismatchError struct {
	Stored, New hash.Hash
}

// Error implements error interface.
func (e *GenesisMismatchError) Error() string {
	return fmt.Sprintf("database contains incompatible gossip genesis (have %s, new %s)", e.Stored.String(), e.New.String())
}

type Configs struct {
	Zilionixx       gossip.Config
	ZilionixxStore  gossip.StoreConfig
	MugamboBFT      abft.Config
	MugamboBFTStore abft.StoreConfig
	VectorClock     vecmt.IndexConfig
	AllowedGenesis  map[uint64]hash.Hash
}

type InputGenesis struct {
	Hash  hash.Hash
	Read  func(*genesisstore.Store) error
	Close func() error
}

func panics(name string) func(error) {
	return func(err error) {
		log.Crit(fmt.Sprintf("%s error", name), "err", err)
	}
}

func mustOpenDB(producer kvdb.DBProducer, name string) kvdb.DropableStore {
	db, err := producer.OpenDB(name)
	if err != nil {
		utils.Fatalf("Failed to open '%s' database: %v", name, err)
	}
	return db
}

func getStores(producer kvdb.FlushableDBProducer, cfg Configs) (*gossip.Store, *abft.Store, *genesisstore.Store) {
	gdb := gossip.NewStore(producer, cfg.ZilionixxStore)

	cMainDb := mustOpenDB(producer, "mugambobft")
	cGetEpochDB := func(epoch idx.Epoch) kvdb.DropableStore {
		return mustOpenDB(producer, fmt.Sprintf("mugambobft-%d", epoch))
	}
	cdb := abft.NewStore(cMainDb, cGetEpochDB, panics("mugambobft store"), cfg.MugamboBFTStore)
	genesisStore := genesisstore.NewStore(mustOpenDB(producer, "genesis"))
	return gdb, cdb, genesisStore
}

func rawApplyGenesis(gdb *gossip.Store, cdb *abft.Store, g mugambo.Genesis, cfg Configs) error {
	_, _, _, err := rawMakeEngine(gdb, cdb, g, cfg, true)
	return err
}

func rawMakeEngine(gdb *gossip.Store, cdb *abft.Store, g mugambo.Genesis, cfg Configs, applyGenesis bool) (*abft.MugamboBFT, *vecmt.Index, gossip.BlockProc, error) {
	blockProc := gossip.DefaultBlockProc(g)

	if applyGenesis {
		_, err := gdb.ApplyGenesis(blockProc, g)
		if err != nil {
			return nil, nil, blockProc, fmt.Errorf("failed to write Gossip genesis state: %v", err)
		}

		err = cdb.ApplyGenesis(&abft.Genesis{
			Epoch:      gdb.GetEpoch(),
			Validators: gdb.GetValidators(),
		})
		if err != nil {
			return nil, nil, blockProc, fmt.Errorf("failed to write MugamboBFT genesis state: %v", err)
		}
	}

	// create consensus
	vecClock := vecmt.NewIndex(panics("Vector clock"), cfg.VectorClock)
	engine := abft.NewMugamboBFT(cdb, &GossipStoreAdapter{gdb}, vecmt2dagidx.Wrap(vecClock), panics("MugamboBFT"), cfg.MugamboBFT)
	return engine, vecClock, blockProc, nil
}

func makeFlushableProducer(rawProducer kvdb.IterableDBProducer) (*flushable.SyncedPool, error) {
	existingDBs := rawProducer.Names()
	err := CheckDBList(existingDBs)
	if err != nil {
		return nil, fmt.Errorf("malformed chainstore: %v", err)
	}
	dbs := flushable.NewSyncedPool(rawProducer, FlushIDKey)
	err = dbs.Initialize(existingDBs)
	if err != nil {
		return nil, fmt.Errorf("failed to open existing databases: %v", err)
	}
	return dbs, nil
}

func applyGenesis(rawProducer kvdb.DBProducer, inputGenesis InputGenesis, cfg Configs) error {
	rawDbs := &DummyFlushableProducer{rawProducer}
	gdb, cdb, genesisStore := getStores(rawDbs, cfg)
	defer gdb.Close()
	defer cdb.Close()
	defer genesisStore.Close()
	log.Info("Decoding genesis file")
	err := inputGenesis.Read(genesisStore)
	if err != nil {
		return err
	}
	log.Info("Applying genesis state")
	networkID := genesisStore.GetRules().NetworkID
	if want, ok := cfg.AllowedGenesis[networkID]; ok && want != inputGenesis.Hash {
		return fmt.Errorf("genesis hash is not allowed for the network %d: want %s, got %s", networkID, want.String(), inputGenesis.Hash.String())
	}
	err = rawApplyGenesis(gdb, cdb, genesisStore.GetGenesis(), cfg)
	if err != nil {
		return err
	}
	err = gdb.Commit()
	if err != nil {
		return err
	}
	return nil
}

func makeEngine(rawProducer kvdb.IterableDBProducer, inputGenesis InputGenesis, emptyStart bool, cfg Configs) (*abft.MugamboBFT, *vecmt.Index, *gossip.Store, *abft.Store, *genesisstore.Store, gossip.BlockProc, error) {
	dbs, err := makeFlushableProducer(rawProducer)
	if err != nil {
		return nil, nil, nil, nil, nil, gossip.BlockProc{}, err
	}

	if emptyStart {
		// close flushable DBs and open raw DBs for performance reasons
		err := dbs.Close()
		if err != nil {
			return nil, nil, nil, nil, nil, gossip.BlockProc{}, fmt.Errorf("failed to close existing databases: %v", err)
		}

		err = applyGenesis(rawProducer, inputGenesis, cfg)
		if err != nil {
			return nil, nil, nil, nil, nil, gossip.BlockProc{}, fmt.Errorf("failed to apply genesis state: %v", err)
		}

		// re-open dbs
		dbs, err = makeFlushableProducer(rawProducer)
		if err != nil {
			return nil, nil, nil, nil, nil, gossip.BlockProc{}, err
		}
	}
	gdb, cdb, genesisStore := getStores(dbs, cfg)
	defer func() {
		if err != nil {
			gdb.Close()
			cdb.Close()
			genesisStore.Close()
		}
	}()

	// compare genesis with the input
	if !emptyStart {
		genesisHash := gdb.GetGenesisHash()
		if genesisHash == nil {
			err = errors.New("malformed chainstore: genesis hash is not written")
			return nil, nil, nil, nil, nil, gossip.BlockProc{}, err
		}
		if *genesisHash != inputGenesis.Hash {
			err = &GenesisMismatchError{*genesisHash, inputGenesis.Hash}
			return nil, nil, nil, nil, nil, gossip.BlockProc{}, err
		}
	}

	engine, vecClock, blockProc, err := rawMakeEngine(gdb, cdb, genesisStore.GetGenesis(), cfg, false)
	if err != nil {
		err = fmt.Errorf("failed to make engine: %v", err)
		return nil, nil, nil, nil, nil, gossip.BlockProc{}, err
	}

	if *gdb.GetGenesisHash() != inputGenesis.Hash {
		err = fmt.Errorf("genesis hash mismatch with genesis file header: %s != %s", gdb.GetGenesisHash().String(), inputGenesis.Hash.String())
		return nil, nil, nil, nil, nil, gossip.BlockProc{}, err
	}

	err = gdb.Commit()
	if err != nil {
		err = fmt.Errorf("failed to commit DBs: %v", err)
		return nil, nil, nil, nil, nil, gossip.BlockProc{}, err
	}

	return engine, vecClock, gdb, cdb, genesisStore, blockProc, nil
}

// MakeEngine makes consensus engine from config.
func MakeEngine(rawProducer kvdb.IterableDBProducer, genesis InputGenesis, cfg Configs) (*abft.MugamboBFT, *vecmt.Index, *gossip.Store, *abft.Store, *genesisstore.Store, gossip.BlockProc) {
	dropAllDBsIfInterrupted(rawProducer)
	existingDBs := rawProducer.Names()

	engine, vecClock, gdb, cdb, genesisStore, blockProc, err := makeEngine(rawProducer, genesis, len(existingDBs) == 0, cfg)
	if err != nil {
		if len(existingDBs) == 0 {
			dropAllDBs(rawProducer)
		}
		utils.Fatalf("Failed to make engine: %v", err)
	}

	if len(existingDBs) == 0 {
		log.Info("Applied genesis state", "hash", genesis.Hash.String())
	} else {
		log.Info("Genesis is already written", "hash", genesis.Hash.String())
	}

	return engine, vecClock, gdb, cdb, genesisStore, blockProc
}

// SetAccountKey sets key into accounts manager and unlocks it with pswd.
func SetAccountKey(
	am *accounts.Manager, key *ecdsa.PrivateKey, pswd string,
) (
	acc accounts.Account,
) {
	kss := am.Backends(keystore.KeyStoreType)
	if len(kss) < 1 {
		log.Crit("Keystore is not found")
		return
	}
	ks := kss[0].(*keystore.KeyStore)

	acc = accounts.Account{
		Address: crypto.PubkeyToAddress(key.PublicKey),
	}

	imported, err := ks.ImportECDSA(key, pswd)
	if err == nil {
		acc = imported
	} else if err.Error() != "account already exists" {
		log.Crit("Failed to import key", "err", err)
	}

	err = ks.Unlock(acc, pswd)
	if err != nil {
		log.Crit("failed to unlock key", "err", err)
	}

	return
}
