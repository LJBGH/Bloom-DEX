-- =========================================================
-- Bloom DEX - Spot Trading Tables (Markets/Orders/Trades)
-- Database: bld-dex
-- =========================================================

-- 说明：
-- 1) 该脚本只创建现货交易相关表，不覆盖 users/wallets/assets 等基础表。
-- 2) 运行前建议先执行 `init_bld_dex_tables.sql`，确保 `bld-dex` 与基础数据表已存在。

CREATE DATABASE IF NOT EXISTS `bld-dex` DEFAULT CHARACTER SET utf8mb4;
USE `bld-dex`;

-- -----------------------------
-- Drop tables (reverse order)
-- -----------------------------
DROP TABLE IF EXISTS `spot_fund_flows`;
DROP TABLE IF EXISTS `spot_trade_settlements`;
DROP TABLE IF EXISTS `spot_trades`;
DROP TABLE IF EXISTS `spot_asset_freezes`;
DROP TABLE IF EXISTS `spot_orders`;
DROP TABLE IF EXISTS `spot_markets`;

-- -----------------------------
-- spot_markets
-- - 现货交易对（例如：ETH/USDT）
-- -----------------------------
CREATE TABLE `spot_markets` (
  `id` INT NOT NULL AUTO_INCREMENT,
  `base_asset_id` INT NOT NULL COMMENT '关联 assets.id（交易的基础币）',
  `quote_asset_id` INT NOT NULL COMMENT '关联 assets.id（计价币）',
  `symbol` VARCHAR(64) NOT NULL COMMENT '例如：ETH/USDT',
  `status` VARCHAR(32) NOT NULL DEFAULT 'ACTIVE' COMMENT 'ACTIVE/PAUSED/DELISTED',
  `maker_fee_rate` DECIMAL(36,18) NOT NULL DEFAULT 0 COMMENT '费率（可做成“每笔收取比例”）',
  `taker_fee_rate` DECIMAL(36,18) NOT NULL DEFAULT 0,
  `min_price` DECIMAL(36,18) NOT NULL DEFAULT 0,
  `min_quantity` DECIMAL(36,18) NOT NULL DEFAULT 0,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- -----------------------------
-- spot_orders
-- -----------------------------
CREATE TABLE `spot_orders` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '下单用户，对应 users.id',
  `market_id` INT NOT NULL COMMENT '关联 spot_markets.id',
  `side` VARCHAR(8) NOT NULL COMMENT 'BUY/SELL',
  `order_type` VARCHAR(8) NOT NULL DEFAULT 'LIMIT' COMMENT 'LIMIT/MARKET',
  `amount_input_mode` VARCHAR(16) NOT NULL DEFAULT 'QUANTITY' COMMENT '用户下单维度：QUANTITY=按数量 TURNOVER=按成交额(报价币)；限价单固定QUANTITY',
  `price` DECIMAL(36,18) NULL COMMENT '限价（LIMIT订单）；MARKET可为空（直到成交）',
  `quantity` DECIMAL(36,18) NOT NULL COMMENT '下单数量（以 base 为单位）',
  `filled_quantity` DECIMAL(36,18) NOT NULL DEFAULT 0 COMMENT '已成交数量（以 base 为单位）',
  `remaining_quantity` DECIMAL(36,18) NOT NULL DEFAULT 0 COMMENT '剩余未成交数量（以 base 为单位）',
  `avg_fill_price` DECIMAL(36,18) NULL COMMENT '平均成交价（全部成交后可落库）',
  `status` VARCHAR(32) NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING/PARTIALLY_FILLED/FILLED/CANCELED/REJECTED',
  `client_order_id` VARCHAR(64) NULL COMMENT '客户端订单号（建议保证同一 user 下唯一）',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `cancelled_at` DATETIME NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- -----------------------------
-- spot_trades
-- - 每次撮合产生一笔成交记录（可以对应一次 price/quantity 的结果）
-- -----------------------------
CREATE TABLE `spot_trades` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `market_id` INT NOT NULL COMMENT '关联 spot_markets.id',
  `maker_order_id` BIGINT UNSIGNED NOT NULL COMMENT '做市方订单',
  `taker_order_id` BIGINT UNSIGNED NOT NULL COMMENT '吃单方订单',
  `price` DECIMAL(36,18) NOT NULL COMMENT '成交价（以 quote/base）',
  `quantity` DECIMAL(36,18) NOT NULL COMMENT '成交数量（以 base 为单位）',
  `fee_asset_id` INT NULL COMMENT '手续费币种，对应 assets.id（可为空表示手续费由系统/外部决定）',
  `fee_amount` DECIMAL(36,18) NOT NULL DEFAULT 0 COMMENT '手续费金额（以 fee_asset 为单位）',
  `tx_hash` VARCHAR(66) NULL COMMENT '若后续链上结算，可记录交易哈希',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- -----------------------------
-- spot_trade_settlements（钱包结算幂等，trade_id = spot_trades.id）
-- -----------------------------
CREATE TABLE `spot_trade_settlements` (
  `trade_id` BIGINT UNSIGNED NOT NULL COMMENT 'spot_trades.id',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`trade_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='成交钱包结算幂等标记';

-- -----------------------------
-- spot_fund_flows
-- - 资金流水（账本），记录用户在现货交易中的“可用/冻结”变化
-- -----------------------------
CREATE TABLE `spot_fund_flows` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '关联 users.id',
  `asset_id` INT NOT NULL COMMENT '关联 assets.id（变动币种）',
  `market_id` INT NULL COMMENT '关联 spot_markets.id（可选）',
  `order_id` BIGINT UNSIGNED NULL COMMENT '关联 spot_orders.id（可选）',
  `trade_id` BIGINT UNSIGNED NULL COMMENT '关联 spot_trades.id（可选）',

  `flow_type` VARCHAR(32) NOT NULL COMMENT 'PLACED_FREEZE/CANCEL_UNFREEZE/TRADE_EXECUTED/FEES/TRANSFER',
  `reason` VARCHAR(255) NULL COMMENT '可读原因（可选）',

  `available_delta` DECIMAL(36,18) NOT NULL DEFAULT 0 COMMENT 'available_balance 的增量（可正可负）',
  `frozen_delta` DECIMAL(36,18) NOT NULL DEFAULT 0 COMMENT 'frozen_balance 的增量（可正可负）',

  `tx_hash` VARCHAR(66) NULL COMMENT '若与链上结算关联，可记录交易哈希（可选）',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
