package logic

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"bld-backend/apps/walletapi/internal/svc"
	wtypes "bld-backend/apps/walletapi/internal/types"
	"bld-backend/core/enum"
	"bld-backend/core/util/amount"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// 扫热钱包逻辑
type SweepToHotLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// 创建扫热钱包逻辑
func NewSweepToHotLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SweepToHotLogic {
	return &SweepToHotLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 扫热钱包逻辑
func (l *SweepToHotLogic) SweepToHot(in *wtypes.SweepToHotReq) (*wtypes.SweepToHotResp, error) {
	userID := in.UserId
	if userID == 0 {
		return nil, errors.New("user_id required")
	}
	symbol := strings.TrimSpace(in.Symbol)
	if symbol == "" {
		return nil, errors.New("symbol required")
	}
	// 归集金额
	amountStr := strings.TrimSpace(in.Amount)
	if amountStr == "" {
		amountStr = "0"
	}
	chain := strings.TrimSpace(in.Chain)
	if chain == "" {
		chain = "EVM"
	}
	if chain != "EVM" {
		return nil, errors.New("unsupported chain")
	}

	// 获取资产
	asset, err := l.svcCtx.AssetModel.FindBySymbol(l.ctx, symbol)
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, errors.New("asset not found")
	}
	shouldAgg, err := l.svcCtx.AssetModel.ShouldAggregateSymbol(l.ctx, symbol)
	if err != nil {
		return nil, err
	}

	// 如果归集金额为0，则归集所有可用余额
	if amountStr == "0" {
		if shouldAgg {
			avail, err := l.svcCtx.WalletBalanceModel.GetAvailableBySymbol(l.ctx, userID, symbol)
			if err != nil {
				return nil, err
			}
			amountStr = avail
		} else {
			avail, err := l.svcCtx.WalletBalanceModel.GetAvailable(l.ctx, userID, asset.ID)
			if err != nil {
				return nil, err
			}
			amountStr = avail
		}
	}

	// 先做余额校验：聚合币按 symbol 总余额校验
	if shouldAgg {
		totalAvail, err := l.svcCtx.WalletBalanceModel.GetAvailableBySymbol(l.ctx, userID, symbol)
		if err != nil {
			return nil, err
		}
		totalWei, err := amount.DecimalToWei(totalAvail, asset.Decimals)
		if err != nil {
			return nil, err
		}
		reqWei, err := amount.DecimalToWei(amountStr, asset.Decimals)
		if err != nil {
			return nil, err
		}
		if totalWei.Cmp(reqWei) < 0 {
			return nil, errors.New("insufficient balance")
		}
	}

	// 将归集金额转换为wei
	amountWei, err := amount.DecimalToWei(amountStr, asset.Decimals)
	if err != nil {
		return nil, err
	}
	if amountWei.Sign() <= 0 {
		return nil, errors.New("sweep amount must be > 0")
	}

	// 获取 custody 钱包
	w, err := l.svcCtx.WalletModel.FindByUser(l.ctx, userID)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, errors.New("custody wallet not found, please create it first")
	}

	// 解密私钥
	plainPriv, err := l.svcCtx.MasterKey.DecryptFromBase64(w.PrivKeyEnc)
	if err != nil {
		return nil, err
	}
	privHex := strings.TrimSpace(string(plainPriv))
	// 将私钥转换为ECDSA
	privKey, err := crypto.HexToECDSA(privHex)
	if err != nil {
		return nil, err
	}
	// 将公钥转换为地址
	fromAddr := crypto.PubkeyToAddress(privKey.PublicKey)

	// 获取热钱包地址
	hotAddr := common.HexToAddress(l.svcCtx.Config.HotWalletAddress)

	// 连接EVM客户端
	evmClient, err := ethclient.Dial(l.svcCtx.Config.EvmRPC)
	if err != nil {
		return nil, err
	}
	defer evmClient.Close()

	// 获取链ID
	chainID, err := evmClient.ChainID(l.ctx)
	if err != nil {
		return nil, err
	}

	// 获取非ce地址
	nonce, err := evmClient.PendingNonceAt(l.ctx, fromAddr)
	if err != nil {
		return nil, err
	}

	// 获取gas价格
	gasPrice, err := evmClient.SuggestGasPrice(l.ctx)
	if err != nil {
		return nil, err
	}

	// 如果资产为ERC20，则处理ERC20
	isERC20 := asset.ContractAddress.Valid && strings.TrimSpace(asset.ContractAddress.String) != ""

	// 创建交易
	var tx *types.Transaction
	// 计算所需的前置费用
	var requiredUpfront *big.Int
	if !isERC20 {
		gasLimit, err := evmClient.EstimateGas(l.ctx, ethereum.CallMsg{
			From:     fromAddr,
			To:       &hotAddr,
			Value:    amountWei,
			GasPrice: gasPrice,
		})
		if err != nil {
			return nil, err
		}
		// 计算所需的前置费用
		requiredUpfront = new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))
		// 计算所需的前置费用
		requiredUpfront = requiredUpfront.Add(requiredUpfront, amountWei)
		// 创建交易
		tx = types.NewTransaction(nonce, hotAddr, amountWei, gasLimit, gasPrice, nil)
	} else {
		contractAddr := common.HexToAddress(asset.ContractAddress.String)
		// 创建ERC20 ABI（含 transfer + balanceOf）
		erc20ABI, err := abi.JSON(strings.NewReader(`[
{"constant":false,"inputs":[{"name":"_to","type":"address"},{"name":"_value","type":"uint256"}],"name":"transfer","outputs":[{"name":"","type":"bool"}],"type":"function"},
{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"balance","type":"uint256"}],"type":"function"}
]`))
		if err != nil {
			return nil, err
		}
		// 先查链上 token 余额，避免节点只返回 "Internal error" 难排查
		balanceOfData, err := erc20ABI.Pack("balanceOf", fromAddr)
		if err != nil {
			return nil, fmt.Errorf("pack balanceOf failed: %w", err)
		}
		out, err := evmClient.CallContract(l.ctx, ethereum.CallMsg{
			To:   &contractAddr,
			Data: balanceOfData,
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("call balanceOf failed: %w", err)
		}
		decoded, err := erc20ABI.Unpack("balanceOf", out)
		if err != nil || len(decoded) == 0 {
			return nil, fmt.Errorf("decode balanceOf failed: %v", err)
		}
		onchainBal, ok := decoded[0].(*big.Int)
		if !ok || onchainBal == nil {
			return nil, errors.New("invalid balanceOf result")
		}
		if onchainBal.Cmp(amountWei) < 0 {
			return nil, fmt.Errorf("insufficient on-chain token balance: have=%s need=%s", amount.WeiToDecimal(onchainBal, asset.Decimals), amount.WeiToDecimal(amountWei, asset.Decimals))
		}
		// 打包数据
		data, err := erc20ABI.Pack("transfer", hotAddr, amountWei)
		if err != nil {
			return nil, fmt.Errorf("pack transfer failed: %w", err)
		}
		// 计算gas限制
		gasLimit, err := evmClient.EstimateGas(l.ctx, ethereum.CallMsg{
			From:     fromAddr,
			To:       &contractAddr,
			Value:    big.NewInt(0),
			Data:     data,
			GasPrice: gasPrice,
		})
		if err != nil {
			return nil, fmt.Errorf("estimate erc20 transfer gas failed: %w", err)
		}
		// 计算所需的前置费用
		requiredUpfront = new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))
		// 创建交易
		tx = types.NewTransaction(nonce, contractAddr, big.NewInt(0), gasLimit, gasPrice, data)
	}

	// 确保 custody 地址有足够的原生ETH用于gas，否则SendTransaction会失败。
	ethBal, err := evmClient.BalanceAt(l.ctx, fromAddr, nil)
	if err != nil {
		return nil, err
	}
	if requiredUpfront != nil && ethBal.Cmp(requiredUpfront) < 0 {
		// 从热钱包向 custody 钱包充值用于gas
		hotPriv := strings.TrimPrefix(l.svcCtx.Config.HotWalletPrivateKey, "0x")
		hotECDSA, err := crypto.HexToECDSA(hotPriv)
		if err != nil {
			return nil, err
		}
		// 将公钥转换为地址
		hotAddrFromPriv := crypto.PubkeyToAddress(hotECDSA.PublicKey)
		// 计算所需的前置费用
		need := new(big.Int).Sub(requiredUpfront, ethBal)
		// 添加小缓冲：+10%
		buffer := new(big.Int).Div(need, big.NewInt(10))
		need = need.Add(need, buffer)
		if need.Sign() <= 0 {
			need = new(big.Int).Add(requiredUpfront, big.NewInt(0))
		}

		// 获取非ce地址
		topupNonce, err := evmClient.PendingNonceAt(l.ctx, hotAddrFromPriv)
		if err != nil {
			return nil, err
		}

		// 计算gas限制
		topupGasLimit, err := evmClient.EstimateGas(l.ctx, ethereum.CallMsg{
			From:     hotAddrFromPriv,
			To:       &fromAddr,
			Value:    need,
			GasPrice: gasPrice,
		})
		if err != nil {
			return nil, err
		}

		// 创建交易
		topupTx := types.NewTransaction(topupNonce, fromAddr, need, topupGasLimit, gasPrice, nil)
		// 获取签名器
		signer := types.LatestSignerForChainID(chainID)
		signedTopupTx, err := types.SignTx(topupTx, signer, hotECDSA)
		if err != nil {
			return nil, err
		}
		// 发送交易
		if err := evmClient.SendTransaction(l.ctx, signedTopupTx); err != nil {
			return nil, err
		}

		// 等待一段时间，直到充值被挖出（本地链应该很快）
		var receipt *types.Receipt
		for i := 0; i < 20; i++ {
			// 获取交易收据
			receipt, err = evmClient.TransactionReceipt(l.ctx, signedTopupTx.Hash())
			if err == nil && receipt != nil {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		if receipt == nil || receipt.Status != 1 {
			return nil, errors.New("gas topup failed")
		}

		// 刷新发送者ETH余额
		ethBal, err = evmClient.BalanceAt(l.ctx, fromAddr, nil)
		if err != nil {
			return nil, err
		}
		// 确保发送者ETH余额足够
		if ethBal.Cmp(requiredUpfront) < 0 {
			return nil, errors.New("gas topup insufficient")
		}
	}

	// 获取签名器
	signer := types.LatestSignerForChainID(chainID)
	// 签名交易
	signedTx, err := types.SignTx(tx, signer, privKey)
	if err != nil {
		return nil, err
	}
	// 发送交易
	if err := evmClient.SendTransaction(l.ctx, signedTx); err != nil {
		return nil, err
	}

	// 链上发送成功后，扣减用户余额：
	// - 聚合币：按 symbol 跨链逐笔扣减，直到满足 amount
	// - 非聚合币：按单资产扣减
	if shouldAgg {
		if err := l.svcCtx.WalletBalanceModel.DebitAvailableBySymbol(l.ctx, userID, symbol, amountStr); err != nil {
			return nil, err
		}
	} else {
		if err := l.svcCtx.WalletBalanceModel.DebitAvailable(l.ctx, userID, asset.ID, amountStr); err != nil {
			return nil, err
		}
	}

	// 返回结果
	return &wtypes.SweepToHotResp{
		TxHash:      signedTx.Hash().Hex(),
		SweptAmount: amountStr,
		Status:      enum.Sent.String(),
	}, nil
}
