package logic

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"bld-backend/apps/walletapi/internal/svc"
	wtypes "bld-backend/apps/walletapi/internal/types"
	"bld-backend/core/enum"
	"bld-backend/core/util/amount"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

type WithdrawLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewWithdrawLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WithdrawLogic {
	return &WithdrawLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *WithdrawLogic) Withdraw(in *wtypes.WithdrawReq) (*wtypes.WithdrawResp, error) {
	userID := in.UserId
	if userID == 0 {
		return nil, errors.New("user_id required")
	}
	symbol := strings.TrimSpace(in.Symbol)
	if symbol == "" {
		return nil, errors.New("symbol required")
	}
	dest := strings.TrimSpace(in.DestAddress)
	if dest == "" {
		return nil, errors.New("dest_address required")
	}
	amountStr := strings.TrimSpace(in.Amount)
	if amountStr == "" {
		return nil, errors.New("amount required")
	}

	chain := strings.TrimSpace(in.Chain)
	if chain == "" {
		chain = "EVM"
	}
	if chain != "EVM" {
		return nil, errors.New("unsupported chain")
	}

	asset, err := l.svcCtx.AssetModel.FindBySymbol(l.ctx, symbol)
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, errors.New("asset not found")
	}
	// Ensure ledger row exists.
	if err := l.svcCtx.WalletBalanceModel.EnsureRow(l.ctx, userID, asset.ID); err != nil {
		return nil, err
	}

	amountWei, err := amount.DecimalToWei(amountStr, asset.Decimals)
	if err != nil {
		return nil, err
	}
	if amountWei.Sign() <= 0 {
		return nil, errors.New("amount must be > 0")
	}

	// Debit first to avoid double-withdraw from ledger perspective.
	if err := l.svcCtx.WalletBalanceModel.DebitAvailable(l.ctx, userID, asset.ID, amountStr); err != nil {
		return nil, err
	}

	withdrawID, err := l.svcCtx.WithdrawOrderModel.Create(l.ctx, userID, asset.ID, dest, amountStr, "SENT")
	if err != nil {
		return nil, err
	}

	evmClient, err := ethclient.Dial(l.svcCtx.Config.EvmRPC)
	if err != nil {
		return nil, err
	}
	defer evmClient.Close()

	hotPriv := strings.TrimPrefix(l.svcCtx.Config.HotWalletPrivateKey, "0x")
	privKey, err := crypto.HexToECDSA(hotPriv)
	if err != nil {
		return nil, err
	}
	hotAddr := crypto.PubkeyToAddress(privKey.PublicKey)

	chainID, err := evmClient.ChainID(l.ctx)
	if err != nil {
		return nil, err
	}
	nonce, err := evmClient.PendingNonceAt(l.ctx, hotAddr)
	if err != nil {
		return nil, err
	}
	gasPrice, err := evmClient.SuggestGasPrice(l.ctx)
	if err != nil {
		return nil, err
	}

	toAddr := common.HexToAddress(dest)
	var tx *types.Transaction

	if !asset.ContractAddress.Valid {
		// Native ETH transfer.
		gasLimit, err := evmClient.EstimateGas(l.ctx, ethereum.CallMsg{
			From:     hotAddr,
			To:       &toAddr,
			Value:    amountWei,
			GasPrice: gasPrice,
		})
		if err != nil {
			return nil, err
		}
		tx = types.NewTransaction(nonce, toAddr, amountWei, gasLimit, gasPrice, nil)
	} else {
		// ERC20 transfer.
		contractAddr := common.HexToAddress(asset.ContractAddress.String)
		erc20ABI, err := abi.JSON(strings.NewReader(`[{"constant":false,"inputs":[{"name":"_to","type":"address"},{"name":"_value","type":"uint256"}],"name":"transfer","outputs":[{"name":"","type":"bool"}],"type":"function"}]`))
		if err != nil {
			return nil, err
		}
		data, err := erc20ABI.Pack("transfer", toAddr, amountWei)
		if err != nil {
			return nil, err
		}

		gasLimit, err := evmClient.EstimateGas(l.ctx, ethereum.CallMsg{
			From:     hotAddr,
			To:       &contractAddr,
			Value:    big.NewInt(0),
			Data:     data,
			GasPrice: gasPrice,
		})
		if err != nil {
			return nil, err
		}
		tx = types.NewTransaction(nonce, contractAddr, big.NewInt(0), gasLimit, gasPrice, data)
	}

	signer := types.LatestSignerForChainID(chainID)
	signedTx, err := types.SignTx(tx, signer, privKey)
	if err != nil {
		return nil, err
	}
	if err := evmClient.SendTransaction(l.ctx, signedTx); err != nil {
		return nil, err
	}

	txHash := signedTx.Hash().Hex()
	_ = l.svcCtx.WithdrawOrderModel.SetTxHash(l.ctx, withdrawID, txHash)

	return &wtypes.WithdrawResp{
		WithdrawId: withdrawID,
		TxHash:     txHash,
		Status:     enum.Sent.String(),
	}, nil
}
