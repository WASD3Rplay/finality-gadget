package db

import (
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
	"github.com/babylonlabs-io/finality-gadget/types"
	"github.com/jackc/pgx/v5"
)

type IDatabaseHandler interface {
	CreateInitialSchema() error
	Close() error

	// Finality gadget
	InsertBlock(block *types.Block) error
	GetBlockByHeight(height uint64) (*types.Block, error)
	GetBlockByHash(hash string) (*types.Block, error)
	QueryIsBlockFinalizedByHeight(height uint64) (bool, error)
	QueryIsBlockFinalizedByHash(hash string) (bool, error)
	QueryLatestFinalizedBlock() (*types.Block, error)
	GetActivatedTimestamp() (uint64, error)
	SaveActivatedTimestamp(timestamp uint64) error

	// Indexer
	BeginTx() (pgx.Tx, error)
	CommitTx(tx pgx.Tx) error
	RollbackTx(tx pgx.Tx) error
	// SaveEvent(tx pgx.Tx, evt *types.Event) error
	SaveInitialFinalityProviders(fps []*bsctypes.FinalityProviderResponse) error
	SaveInitialDelegations(dels []*bbntypes.BTCDelegationResponse) error
	SaveEventNewFinalityProvider(tx pgx.Tx, txInfo *types.TxInfo, evtIdx int, evt *types.EventNewFinalityProvider) error
	SaveEventBTCDelegationStateUpdate(tx pgx.Tx, txInfo *types.TxInfo, evtIdx int, evt *types.EventBTCDelegationStateUpdate) error
	SaveEventJailedFinalityProvider(tx pgx.Tx, txInfo *types.TxInfo, evtIdx int, evt *types.EventJailedFinalityProvider) error
	SaveEventUnjailedFinalityProvider(tx pgx.Tx, txInfo *types.TxInfo, evtIdx int, evt *types.EventUnjailedFinalityProvider) error
	SaveEventSelectiveSlashing(tx pgx.Tx, txInfo *types.TxInfo, evtIdx int, evt *types.EventSelectiveSlashing) error
	SaveEventSlashedFinalityProvider(tx pgx.Tx, txInfo *types.TxInfo, evtIdx int, evt *types.EventSlashedFinalityProvider) error
}
