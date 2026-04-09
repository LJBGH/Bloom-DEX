package logic

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"bld-backend/apps/ordersapi/internal/svc"
	"bld-backend/apps/ordersapi/internal/types"
	"bld-backend/core/enum"
	"bld-backend/core/model"

	"github.com/zeromicro/go-zero/core/logx"
)

type CancelSpotOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelSpotOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelSpotOrderLogic {
	return &CancelSpotOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CancelSpotOrder 撤销未完结限价单：校验后发往 Kafka，由 exchange 落库、撤簿、解冻。
func (l *CancelSpotOrderLogic) CancelSpotOrder(req *types.CancelSpotOrderReq) (*types.CancelSpotOrderResp, error) {
	if req.UserId == 0 {
		return nil, errors.New("user_id required")
	}
	s := strings.TrimSpace(req.OrderId)
	if s == "" {
		return nil, errors.New("order_id required")
	}
	orderID, err := strconv.ParseUint(s, 10, 64)
	if err != nil || orderID == 0 {
		return nil, errors.New("invalid order_id")
	}

	// 检查订单 ID 是否在 Bloom 过滤器中，如果没有直接返回
	if l.svcCtx.OrdersBF != nil {
		ok, err := l.svcCtx.OrdersBF.ExistsString(l.ctx, strconv.FormatUint(orderID, 10))
		if err != nil {
			// Bloom 仅用于加速，不应因 Redis 异常导致无法撤单
			logx.Errorf("orders bloom exists failed: order_id=%d err=%v", orderID, err)
		} else if !ok {
			return nil, errors.New("order not found")
		}
	}

	row, err := l.svcCtx.SpotOrderModel.GetByIDAndUser(l.ctx, orderID, req.UserId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("order not found")
		}
		return nil, err
	}

	if row.OrderType != enum.Limit.String() {
		return nil, errors.New("only LIMIT orders can be canceled")
	}
	if row.Status != enum.SOS_Pending.String() && row.Status != enum.SOS_PartiallyFilled.String() {
		return nil, errors.New("order cannot be canceled")
	}

	mkt, err := l.svcCtx.SpotMarketModel.GetByID(l.ctx, row.MarketID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("market not found")
		}
		return nil, err
	}

	if l.svcCtx.KafkaProducer == nil {
		return nil, errors.New("kafka producer not initialized")
	}
	if l.svcCtx.Config.Kafka.Partitions <= 0 {
		return nil, errors.New("kafka partitions must be > 0")
	}

	partition := int32(row.MarketID) % l.svcCtx.Config.Kafka.Partitions
	key := strconv.Itoa(row.MarketID)

	msg := &model.SpotOrderKafkaMsg{
		OrderID:         orderID,
		UserID:          req.UserId,
		MarketID:        row.MarketID,
		CreatedAtMs:     row.CreatedAt.UnixMilli(),
		Side:            row.Side,
		OrderType:       row.OrderType,
		AmountInputMode: row.AmountInputMode,
		Price:           spotNullStringPtr(row.Price),
		Quantity:        row.Quantity,
		MaxQuoteAmount:  spotNullStringPtr(row.MaxQuoteAmount),
		FilledQuantity:  row.FilledQuantity,
		RemainingQty:    row.RemainingQuantity,
		AvgFillPrice:    spotNullStringPtr(row.AvgFillPrice),
		BaseAssetID:     mkt.BaseAssetID,
		QuoteAssetID:    mkt.QuoteAssetID,
		MakerFeeRate:    mkt.MakerFeeRate,
		TakerFeeRate:    mkt.TakerFeeRate,
		ClientOrderID:   spotNullStringPtr(row.ClientOrderID),
		Status:          enum.SOS_Canceled.String(),
	}
	if err := l.svcCtx.KafkaProducer.Publish(l.ctx, l.svcCtx.Config.Kafka.Topic, partition, key, msg); err != nil {
		return nil, err
	}

	return &types.CancelSpotOrderResp{
		OrderId: strconv.FormatUint(orderID, 10),
		Status:  enum.SOS_Canceled.String(),
	}, nil
}

// 将 sql.NullString 转换为 *string
func spotNullStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}
