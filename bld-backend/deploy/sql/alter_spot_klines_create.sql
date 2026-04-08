-- spot_klines: pre-aggregated OHLCV for TradingView / market-ws
CREATE TABLE IF NOT EXISTS `spot_klines` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'K线主键ID',
  `market_id` INT NOT NULL COMMENT '交易对ID',
  `interval` VARCHAR(8) NOT NULL COMMENT '1m/5m/15m/1h/4h/1d...',
  `open_time_ms` BIGINT NOT NULL COMMENT 'K线开盘时间戳（ms，按 interval 对齐）',

  `open` DECIMAL(36,18) NOT NULL COMMENT '开盘价',
  `high` DECIMAL(36,18) NOT NULL COMMENT '最高价',
  `low` DECIMAL(36,18) NOT NULL COMMENT '最低价',
  `close` DECIMAL(36,18) NOT NULL COMMENT '收盘价',
  `volume` DECIMAL(36,18) NOT NULL DEFAULT 0 COMMENT '成交量（基准币）',
  `turnover` DECIMAL(36,18) NOT NULL DEFAULT 0 COMMENT '成交额（报价币）',
  `trades_count` INT NOT NULL DEFAULT 0 COMMENT '成交笔数',

  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

