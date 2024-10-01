package bbnclient

import (
	"math"

	bbnClient "github.com/babylonlabs-io/babylon/client/client"
	bbncfg "github.com/babylonlabs-io/babylon/client/config"
	"github.com/babylonlabs-io/babylon/client/query"
	btcctypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
	cfg "github.com/babylonlabs-io/finality-gadget/config"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	"go.uber.org/zap"
)

type BabylonClient struct {
	queryClient *query.QueryClient
	logger      *zap.Logger
}

//////////////////////////////
// CONSTRUCTOR
//////////////////////////////

func NewBabylonClient(cfg *cfg.BBNConfig, logger *zap.Logger) (*BabylonClient, error) {
	// Load configs
	bbnConfig := bbncfg.DefaultBabylonConfig()
	bbnConfig.RPCAddr = cfg.BabylonRPCAddress
	bbnConfig.ChainID = cfg.BabylonChainId

	// Create Babylon client
	babylonClient, err := bbnClient.New(&bbnConfig, logger)
	if err != nil {
		return nil, err
	}

	return &BabylonClient{
		queryClient: babylonClient.QueryClient,
	}, nil
}

//////////////////////////////
// METHODS
//////////////////////////////

func (bbnClient *BabylonClient) QueryAllFinalityProviders(consumerId string) ([]*bsctypes.FinalityProviderResponse, error) {
	bbnClient.logger.Info("Querying all finality providers", zap.String("consumer_id", consumerId))

	pagination := &sdkquerytypes.PageRequest{}
	resp, err := bbnClient.queryClient.QueryConsumerFinalityProviders(consumerId, pagination)
	if err != nil {
		return nil, err
	}

	var pkArr []*bsctypes.FinalityProviderResponse

	for _, fp := range resp.FinalityProviders {
		pkArr = append(pkArr, fp)
	}
	return pkArr, nil
}

func (bbnClient *BabylonClient) QueryBTCDelegation(stakingTxHashHex string) (*bbntypes.BTCDelegationResponse, error) {
	bbnClient.logger.Info("Querying finality provider delegation", zap.String("staking_tx_hex_hash", stakingTxHashHex))

	resp, err := bbnClient.queryClient.BTCDelegation(stakingTxHashHex)
	if err != nil {
		return nil, err
	}

	return resp.GetBtcDelegation(), nil
}

func (bbnClient *BabylonClient) QueryBTCCheckpointParams() (*btcctypes.QueryParamsResponse, error) {
	bbnClient.logger.Info("Querying BTC checkpoint params")

	resp, err := bbnClient.queryClient.BTCCheckpointParams()
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (bbnClient *BabylonClient) QueryBTCStakingParams() (*bbntypes.QueryParamsResponse, error) {
	bbnClient.logger.Info("Querying BTC staking params")

	resp, err := bbnClient.queryClient.BTCStakingParams()
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (bbnClient *BabylonClient) QueryFPDelegations(fpBtcPk string) ([]*bbntypes.BTCDelegationResponse, error) {
	bbnClient.logger.Info("Querying finality provider delegations", zap.String("fp_btc_pk", fpBtcPk))

	pagination := &sdkquerytypes.PageRequest{
		Limit: 100,
	}

	resp, err := bbnClient.queryClient.FinalityProviderDelegations(fpBtcPk, pagination)
	if err != nil {
		return nil, err
	}

	var delArr []*bbntypes.BTCDelegationResponse
	for _, del := range resp.BtcDelegatorDelegations {
		delArr = append(delArr, del.Dels...)
	}

	return delArr, nil
}

func (bbnClient *BabylonClient) QueryAllFpBtcPubKeys(consumerId string) ([]string, error) {
	pagination := &sdkquerytypes.PageRequest{}
	resp, err := bbnClient.queryClient.QueryConsumerFinalityProviders(consumerId, pagination)
	if err != nil {
		return nil, err
	}

	var pkArr []string

	for _, fp := range resp.FinalityProviders {
		pkArr = append(pkArr, fp.BtcPk.MarshalHex())
	}
	return pkArr, nil
}

func (bbnClient *BabylonClient) QueryFpPower(fpPubkeyHex string, btcHeight uint64) (uint64, error) {
	totalPower := uint64(0)
	pagination := &sdkquerytypes.PageRequest{}
	// queries the BTCStaking module for all delegations of a finality provider
	resp, err := bbnClient.queryClient.FinalityProviderDelegations(fpPubkeyHex, pagination)
	if err != nil {
		return 0, err
	}
	for {
		// btcDels contains all the queried BTC delegations
		for _, btcDels := range resp.BtcDelegatorDelegations {
			for _, btcDel := range btcDels.Dels {
				// check whether the delegation is active
				isActive, err := bbnClient.isDelegationActive(btcDel, btcHeight)
				if err != nil {
					return 0, err
				}
				if isActive {
					totalPower += btcDel.TotalSat
				}
			}
		}
		if resp.Pagination == nil || resp.Pagination.NextKey == nil {
			break
		}
		pagination.Key = resp.Pagination.NextKey
	}

	return totalPower, nil
}

func (bbnClient *BabylonClient) QueryMultiFpPower(
	fpPubkeyHexList []string,
	btcHeight uint64,
) (map[string]uint64, error) {
	fpPowerMap := make(map[string]uint64)

	for _, fpPubkeyHex := range fpPubkeyHexList {
		fpPower, err := bbnClient.QueryFpPower(fpPubkeyHex, btcHeight)
		if err != nil {
			return nil, err
		}
		fpPowerMap[fpPubkeyHex] = fpPower
	}

	return fpPowerMap, nil
}

// QueryEarliestActiveDelBtcHeight returns the earliest active BTC staking height
func (bbnClient *BabylonClient) QueryEarliestActiveDelBtcHeight(fpPkHexList []string) (uint64, error) {
	allFpEarliestDelBtcHeight := uint64(math.MaxUint64)

	for _, fpPkHex := range fpPkHexList {
		fpEarliestDelBtcHeight, err := bbnClient.QueryFpEarliestActiveDelBtcHeight(fpPkHex)
		if err != nil {
			return math.MaxUint64, err
		}
		if fpEarliestDelBtcHeight < allFpEarliestDelBtcHeight {
			allFpEarliestDelBtcHeight = fpEarliestDelBtcHeight
		}
	}

	return allFpEarliestDelBtcHeight, nil
}

func (bbnClient *BabylonClient) QueryFpEarliestActiveDelBtcHeight(fpPubkeyHex string) (uint64, error) {
	pagination := &sdkquerytypes.PageRequest{
		Limit: 100,
	}

	// queries the BTCStaking module for all delegations of a finality provider
	resp, err := bbnClient.queryClient.FinalityProviderDelegations(fpPubkeyHex, pagination)
	if err != nil {
		return math.MaxUint64, err
	}

	// queries BtcConfirmationDepth, CovenantQuorum, and the latest BTC header
	btccheckpointParams, err := bbnClient.queryClient.BTCCheckpointParams()
	if err != nil {
		return math.MaxUint64, err
	}

	// get the BTC staking params
	btcstakingParams, err := bbnClient.queryClient.BTCStakingParams()
	if err != nil {
		return math.MaxUint64, err
	}

	// get the latest BTC header
	btcHeader, err := bbnClient.queryClient.BTCHeaderChainTip()
	if err != nil {
		return math.MaxUint64, err
	}

	kValue := btccheckpointParams.GetParams().BtcConfirmationDepth
	covQuorum := btcstakingParams.GetParams().CovenantQuorum
	latestBtcHeight := btcHeader.GetHeader().Height

	earliestBtcHeight := uint64(math.MaxUint64)
	for {
		// btcDels contains all the queried BTC delegations
		for _, btcDels := range resp.BtcDelegatorDelegations {
			for _, btcDel := range btcDels.Dels {
				activationHeight := getDelFirstActiveHeight(btcDel, latestBtcHeight, kValue, covQuorum)
				if activationHeight < earliestBtcHeight {
					earliestBtcHeight = activationHeight
				}
			}
		}
		if resp.Pagination == nil || resp.Pagination.NextKey == nil {
			break
		}
		pagination.Key = resp.Pagination.NextKey
	}
	return earliestBtcHeight, nil
}

//////////////////////////////
// INTERNAL
//////////////////////////////

// we implemented exact logic as in GetStatus
// https://github.com/babylonlabs-io/babylon-private/blob/3d8f190c9b0c0795f6546806e3b8582de716cd60/x/btcstaking/types/btc_delegation.go#L90-L111
func (bbnClient *BabylonClient) isDelegationActive(
	btcDel *bbntypes.BTCDelegationResponse,
	btcHeight uint64,
) (bool, error) {
	btccheckpointParams, err := bbnClient.queryClient.BTCCheckpointParams()
	if err != nil {
		return false, err
	}
	btcstakingParams, err := bbnClient.queryClient.BTCStakingParams()
	if err != nil {
		return false, err
	}
	kValue := btccheckpointParams.GetParams().BtcConfirmationDepth
	wValue := btccheckpointParams.GetParams().CheckpointFinalizationTimeout
	covQuorum := btcstakingParams.GetParams().CovenantQuorum
	ud := btcDel.UndelegationResponse

	if len(ud.GetDelegatorUnbondingSigHex()) > 0 {
		return false, nil
	}

	// k is not involved in the `GetStatus` logic as Babylon will accept a BTC delegation request
	// only when staking tx is k-deep on BTC.
	//
	// But the msg handler performs both checks 1) ensure staking tx is k-deep, and 2) ensure the
	// staking tx's timelock has at least w BTC blocks left.
	// (https://github.com/babylonlabs-io/babylon-private/blob/3d8f190c9b0c0795f6546806e3b8582de716cd60/x/btcstaking/keeper/msg_server.go#L283-L292)
	//
	// So after the msg handler accepts BTC delegation the 1st check is no longer needed
	// the k-value check is added per
	//
	// So in our case, we need to check both to ensure the delegation is active
	if btcHeight < btcDel.StartHeight+kValue || btcHeight+wValue > btcDel.EndHeight {
		return false, nil
	}

	if uint32(len(btcDel.CovenantSigs)) < covQuorum {
		return false, nil
	}
	if len(ud.CovenantUnbondingSigList) < int(covQuorum) {
		return false, nil
	}
	if len(ud.CovenantSlashingSigs) < int(covQuorum) {
		return false, nil
	}

	return true, nil
}

// The active delegation needs to satisfy:
// 1) the staking tx is k-deep in Bitcoin, i.e., start_height + k
// 2) it receives a quorum number of covenant committee signatures
//
// return math.MaxUint64 if the delegation is not active
//
// Note: the delegation can be unbounded and that's totally fine and shouldn't affect when the chain was activated
func getDelFirstActiveHeight(btcDel *bbntypes.BTCDelegationResponse, latestBtcHeight, kValue uint64, covQuorum uint32) uint64 {
	activationHeight := btcDel.StartHeight + kValue
	// not activated yet
	if latestBtcHeight < activationHeight || uint32(len(btcDel.CovenantSigs)) < covQuorum {
		return math.MaxUint64
	}
	return activationHeight
}
