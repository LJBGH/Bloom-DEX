package worker

import (
	"bld-backend/apps/chain-monitor/internal/config"
	"context"
	"database/sql"
	"errors"
	"math/big"
	"strings"
	"time"

	"bld-backend/core/util/amount"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type Worker struct {
	cfg  config.Config
	stop chan struct{}
}

func New(cfg config.Config) *Worker {
	return &Worker{
		cfg:  cfg,
		stop: make(chan struct{}),
	}
}

func (w *Worker) Start() {
	logx.Infof("worker %s bootstrap completed", w.cfg.Name)

	go w.run()
}

func (w *Worker) Stop() {
	select {
	case <-w.stop:
		// already closed
	default:
		close(w.stop)
	}
	logx.Infof("worker %s shutdown completed", w.cfg.Name)
}

// 运行
func (w *Worker) run() {
	ctx := context.Background()
	conn := sqlx.NewMysql(w.cfg.Mysql.DataSource)
	// sqlx.SqlConn doesn't expose Close(); it should not be closed manually.

	networkID, err := w.getNetworkIDBySymbol(ctx, conn, "LOCALHOST")
	if err != nil {
		logx.Errorf("get network_id failed: %v", err)
		return
	}

	evmClient, err := ethclient.Dial(w.cfg.EvmRPC)
	if err != nil {
		logx.Errorf("evm dial failed: %v", err)
		return
	}
	defer evmClient.Close()

	chainID, err := evmClient.ChainID(ctx)
	if err != nil {
		logx.Errorf("chainID failed: %v", err)
		return
	}

	transferTopic0 := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

	chainKey := "EVM"

	// Batch loop
	for {
		select {
		case <-w.stop:
			return
		default:
		}

		currentBlock, err := evmClient.BlockNumber(ctx)
		if err != nil {
			logx.Errorf("BlockNumber failed: %v", err)
			time.Sleep(time.Duration(w.cfg.PollIntervalSeconds) * time.Second)
			continue
		}

		toBlock := int64(currentBlock) - w.cfg.Confirmation
		if toBlock < 0 {
			toBlock = 0
		}

		lastBlock, err := w.getLastBlock(ctx, conn, networkID)
		if err != nil {
			logx.Errorf("getLastBlock failed: %v", err)
			time.Sleep(time.Duration(w.cfg.PollIntervalSeconds) * time.Second)
			continue
		}

		if lastBlock >= toBlock {
			time.Sleep(time.Duration(w.cfg.PollIntervalSeconds) * time.Second)
			continue
		}

		// Load assets
		ethAsset, usdtAsset, err := w.loadAssets(ctx, conn, networkID)
		if err != nil {
			logx.Errorf("loadAssets failed: %v", err)
			time.Sleep(time.Duration(w.cfg.PollIntervalSeconds) * time.Second)
			continue
		}

		// Load custody addresses
		custodyMap, err := w.loadCustodyMap(ctx, conn, networkID)
		if err != nil {
			logx.Errorf("loadCustodyMap failed: %v", err)
			time.Sleep(time.Duration(w.cfg.PollIntervalSeconds) * time.Second)
			continue
		}

		// Process range with cap
		from := lastBlock + 1
		rangeEnd := toBlock
		const maxBatch = int64(25)
		if rangeEnd-from+1 > maxBatch {
			rangeEnd = from + maxBatch - 1
		}

		for b := from; b <= rangeEnd; b++ {
			if err := w.processBlock(ctx, conn, evmClient, chainID, transferTopic0, custodyMap, ethAsset, usdtAsset, chainKey, b); err != nil {
				logx.Errorf("process block %d failed: %v", b, err)
				// on error, stop this round to retry later
				break
			}
			if err := w.setLastBlock(ctx, conn, networkID, b); err != nil {
				logx.Errorf("setLastBlock %d failed: %v", b, err)
				break
			}
		}
	}
}

type assetRow struct {
	ID              int
	Symbol          string
	Decimals        int
	ContractAddress sql.NullString
}

// 加载资产
func (w *Worker) loadAssets(ctx context.Context, conn sqlx.SqlConn, networkID int) (*assetRow, *assetRow, error) {
	var eth assetRow
	var usdt assetRow

	err := conn.QueryRowCtx(ctx, &eth,
		"SELECT id,symbol,decimals,contract_address FROM assets WHERE symbol='ETH' AND network_id=? AND is_active=1 LIMIT 1",
		networkID)
	if err != nil {
		return nil, nil, err
	}
	err = conn.QueryRowCtx(ctx, &usdt,
		"SELECT id,symbol,decimals,contract_address FROM assets WHERE symbol='USDT' AND network_id=? AND is_active=1 LIMIT 1",
		networkID)
	if err != nil {
		return nil, nil, err
	}
	return &eth, &usdt, nil
}

// 获取 custody 地址
func (w *Worker) loadCustodyMap(ctx context.Context, conn sqlx.SqlConn, networkID int) (map[string]uint64, error) {
	// address(lowercase 0x...) => user_id
	db, err := conn.RawDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, "SELECT user_id,address FROM wallets WHERE network_id=?", networkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]uint64)
	for rows.Next() {
		var userID uint64
		var addr string
		if err := rows.Scan(&userID, &addr); err != nil {
			return nil, err
		}
		out[strings.ToLower(addr)] = userID
	}
	return out, nil
}

// 获取最后区块
func (w *Worker) getLastBlock(ctx context.Context, conn sqlx.SqlConn, networkID int) (int64, error) {
	var last int64
	err := conn.QueryRowCtx(ctx, &last, "SELECT last_block FROM network_offsets WHERE network_id=? ORDER BY id ASC LIMIT 1", networkID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// init
			_, insErr := conn.ExecCtx(ctx, "INSERT INTO network_offsets(network_id,last_block) VALUES(?,?)", networkID, w.cfg.InitBlockHeight)
			if insErr != nil {
				return 0, insErr
			}
			return w.cfg.InitBlockHeight, nil
		}
		return 0, err
	}
	return last, nil
}

// 设置最后区块
func (w *Worker) setLastBlock(ctx context.Context, conn sqlx.SqlConn, networkID int, b int64) error {
	_, err := conn.ExecCtx(ctx, "UPDATE network_offsets SET last_block=? WHERE network_id=? ORDER BY id ASC LIMIT 1", b, networkID)
	return err
}

func (w *Worker) getNetworkIDBySymbol(ctx context.Context, conn sqlx.SqlConn, symbol string) (int, error) {
	var id int
	err := conn.QueryRowCtx(ctx, &id, "SELECT id FROM networks WHERE symbol=? ORDER BY id ASC LIMIT 1", symbol)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// 处理区块
func (w *Worker) processBlock(
	ctx context.Context,
	conn sqlx.SqlConn,
	evmClient *ethclient.Client,
	chainID *big.Int,
	transferTopic0 common.Hash,
	custodyMap map[string]uint64,
	ethAsset *assetRow,
	usdtAsset *assetRow,
	chainKey string,
	blockNum int64,
) error {
	// ETH deposits: scan txs in block.
	block, err := evmClient.BlockByNumber(ctx, big.NewInt(blockNum))
	if err != nil {
		return err
	}
	txs := block.Transactions()
	for _, tx := range txs {
		to := tx.To()
		if to == nil {
			continue
		}
		if tx.Value().Sign() == 0 {
			continue
		}
		toAddr := strings.ToLower(to.Hex())
		userID, ok := custodyMap[toAddr]
		if !ok {
			continue
		}

		amountStr := amount.WeiToDecimal(tx.Value(), ethAsset.Decimals)
		txHash := tx.Hash().Hex()
		// log_index = -1 for native deposits
		inserted, err := w.insertDepositEvent(ctx, conn, txHash, -1, blockNum, chainKey, ethAsset.ID, userID, amountStr, "", to.Hex())
		if err != nil {
			return err
		}
		if inserted {
			if err := w.creditWalletBalance(ctx, conn, userID, ethAsset.ID, amountStr); err != nil {
				return err
			}
		}
	}

	// USDT deposits: scan logs for Transfer(to=depositAddress)
	if usdtAsset.ContractAddress.Valid {
		contractAddr := common.HexToAddress(usdtAsset.ContractAddress.String)
		query := ethereum.FilterQuery{
			FromBlock: big.NewInt(blockNum),
			ToBlock:   big.NewInt(blockNum),
			Addresses: []common.Address{contractAddr},
			Topics:    [][]common.Hash{{transferTopic0}},
		}
		logs, err := evmClient.FilterLogs(ctx, query)
		if err != nil {
			return err
		}
		for _, lg := range logs {
			// topics[1]=from, topics[2]=to
			if len(lg.Topics) < 3 {
				continue
			}
			to := common.BytesToAddress(lg.Topics[2].Bytes()).Hex()
			userID, ok := custodyMap[strings.ToLower(to)]
			if !ok {
				continue
			}
			amtWei := new(big.Int).SetBytes(lg.Data)
			amountStr := amount.WeiToDecimal(amtWei, usdtAsset.Decimals)
			txHash := lg.TxHash.Hex()
			inserted, err := w.insertDepositEvent(ctx, conn, txHash, int(lg.Index), blockNum, chainKey, usdtAsset.ID, userID, amountStr, lg.TxHash.Hex(), to)
			if err != nil {
				return err
			}
			if inserted {
				if err := w.creditWalletBalance(ctx, conn, userID, usdtAsset.ID, amountStr); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// 插入充值事件
func (w *Worker) insertDepositEvent(
	ctx context.Context,
	conn sqlx.SqlConn,
	txHash string,
	logIndex int,
	blockNum int64,
	chain string,
	assetID int,
	userID uint64,
	amountStr string,
	fromAddr string,
	toAddr string,
) (bool, error) {
	// No unique indexes: do manual dedup check
	var exists int
	err := conn.QueryRowCtx(ctx, &exists,
		"SELECT 1 FROM deposit_events WHERE tx_hash=? AND log_index=? LIMIT 1",
		txHash, logIndex,
	)
	if err == nil && exists == 1 {
		return false, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}

	_, err = conn.ExecCtx(ctx,
		"INSERT INTO deposit_events(tx_hash,log_index,block_number,chain,asset_id,user_id,amount,from_address,to_address) VALUES(?,?,?,?,?,?,?,?,?)",
		txHash, logIndex, blockNum, chain, assetID, userID, amountStr, fromAddr, toAddr,
	)
	if err != nil {
		return false, err
	}
	return true, nil
}

// 增加钱包余额
func (w *Worker) creditWalletBalance(ctx context.Context, conn sqlx.SqlConn, userID uint64, assetID int, amountStr string) error {
	return conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var firstID uint64
		err := session.QueryRowCtx(ctx, &firstID,
			"SELECT id FROM wallet_balances WHERE user_id=? AND asset_id=? ORDER BY id ASC LIMIT 1",
			userID, assetID,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// no row: create one directly with credited amount
				_, err = session.ExecCtx(ctx,
					"INSERT INTO wallet_balances(user_id,asset_id,available_balance,frozen_balance) VALUES(?,?,?,0)",
					userID, assetID, amountStr,
				)
				return err
			}
			return err
		}

		// consolidate possible duplicate rows into first row
		var sums struct {
			Available string `db:"available_balance"`
			Frozen    string `db:"frozen_balance"`
		}
		if err := session.QueryRowCtx(ctx, &sums,
			"SELECT COALESCE(SUM(available_balance),0) AS available_balance, COALESCE(SUM(frozen_balance),0) AS frozen_balance FROM wallet_balances WHERE user_id=? AND asset_id=?",
			userID, assetID,
		); err != nil {
			return err
		}
		if _, err := session.ExecCtx(ctx,
			"UPDATE wallet_balances SET available_balance=?, frozen_balance=? WHERE id=?",
			sums.Available, sums.Frozen, firstID,
		); err != nil {
			return err
		}
		if _, err := session.ExecCtx(ctx,
			"DELETE FROM wallet_balances WHERE user_id=? AND asset_id=? AND id<>?",
			userID, assetID, firstID,
		); err != nil {
			return err
		}

		// apply this credit on the kept row
		_, err = session.ExecCtx(ctx,
			"UPDATE wallet_balances SET available_balance = available_balance + ? WHERE id=?",
			amountStr, firstID,
		)
		return err
	})
}
