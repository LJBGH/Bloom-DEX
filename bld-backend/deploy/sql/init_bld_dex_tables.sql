-- =========================================================
-- Bloom DEX - Full Init Script (DROP + CREATE + SEED)
-- Database: bld-dex
-- =========================================================

CREATE DATABASE IF NOT EXISTS `bld-dex` DEFAULT CHARACTER SET utf8mb4;
USE `bld-dex`;

-- -----------------------------
-- Drop old tables (reverse FK order)
-- -----------------------------
DROP TABLE IF EXISTS `network_offsets`;
DROP TABLE IF EXISTS `withdraw_orders`;
DROP TABLE IF EXISTS `deposit_events`;
DROP TABLE IF EXISTS `wallet_balances`;
DROP TABLE IF EXISTS `wallets`;
DROP TABLE IF EXISTS `assets`;
DROP TABLE IF EXISTS `networks`;
DROP TABLE IF EXISTS `users`;

-- -----------------------------
-- users
-- -----------------------------
CREATE TABLE `users` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `username` VARCHAR(64) NOT NULL,
  `password_hash` VARCHAR(255) NOT NULL,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- -----------------------------
-- networks (new)
-- symbol: 缩写
-- name: 全称
-- rpc_url: RPC地址
-- chain_id: 链ID(EVM可填，非EVM可空)
-- crypto_type: 账户/签名体系（托管地址与密钥派生用），如 EVM / BITCOIN / SOLANA
-- -----------------------------
CREATE TABLE `networks` (
  `id` INT NOT NULL AUTO_INCREMENT,
  `symbol` VARCHAR(32) NOT NULL COMMENT '缩写，如 LOCALHOST/BTC/ETH/BSC/SOL',
  `name` VARCHAR(64) NOT NULL COMMENT '全称，如 Localhost/Bitcoin/Ethereum',
  `rpc_url` VARCHAR(255) NULL COMMENT 'RPC 地址/URL',
  `chain_id` BIGINT NULL COMMENT 'EVM chain id（非EVM可为空）',
  `crypto_type` VARCHAR(32) NOT NULL DEFAULT 'EVM' COMMENT 'EVM=以太坊系；BITCOIN/SOLANA=其他链',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- -----------------------------
-- wallets
-- -----------------------------
CREATE TABLE `wallets` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `network_id` INT NOT NULL COMMENT '关联 networks.id',
  `address` VARCHAR(128) NOT NULL,
  `privkey_enc` TEXT NOT NULL,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- -----------------------------
-- assets
-- chain_type: EVM/Bitcoin/Solana...
-- contract_address: ERC20才有，原生币为NULL
-- -----------------------------
CREATE TABLE `assets` (
  `id` INT NOT NULL AUTO_INCREMENT,
  `symbol` VARCHAR(32) NOT NULL,
  `name` VARCHAR(128) NOT NULL,
  `decimals` INT NOT NULL,
  `is_active` TINYINT(1) NOT NULL DEFAULT 1,
  `is_aggregate` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否聚合显示（如稳定币跨链）',
  `network_id` INT NOT NULL COMMENT '关联 networks.id',
  `contract_address` VARCHAR(128) NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- -----------------------------
-- asset_freezes
-- - 订单维度资产冻结明细（与 wallet_balances.frozen_balance 可对账）
-- -----------------------------
CREATE TABLE `asset_freezes` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '关联 users.id',
  `asset_id` INT NOT NULL COMMENT '关联 assets.id',
  `order_id` BIGINT UNSIGNED NOT NULL COMMENT '关联 spot_orders.id',
  `trading_type` VARCHAR(8) NOT NULL COMMENT '现货/合约：SPOT/CONTRACT',
  `frozen_amount` DECIMAL(36,18) NOT NULL DEFAULT 0 COMMENT '本条记录冻结金额（该资产）',
  `is_frozen` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1=冻结中 0=已解冻/释放',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- -----------------------------
-- wallet_balances
-- -----------------------------
CREATE TABLE `wallet_balances` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `asset_id` INT NOT NULL,
  `available_balance` DECIMAL(36,18) NOT NULL DEFAULT 0,
  `frozen_balance` DECIMAL(36,18) NOT NULL DEFAULT 0,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- -----------------------------
-- deposit_events (充值流水)
-- tx_hash + log_index 去重
-- -----------------------------
CREATE TABLE `deposit_events` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tx_hash` VARCHAR(66) NOT NULL,
  `log_index` INT NOT NULL DEFAULT -1,
  `block_number` BIGINT NOT NULL,
  `chain` VARCHAR(32) NOT NULL,
  `asset_id` INT NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `amount` DECIMAL(36,18) NOT NULL,
  `from_address` VARCHAR(128) NULL,
  `to_address` VARCHAR(128) NULL,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- -----------------------------
-- withdraw_orders (提现订单)
-- -----------------------------
CREATE TABLE `withdraw_orders` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `asset_id` INT NOT NULL,
  `dest_address` VARCHAR(128) NOT NULL,
  `amount` DECIMAL(36,18) NOT NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'SENT',
  `tx_hash` VARCHAR(66) NULL,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- -----------------------------
-- network_offsets (监听进度)
-- -----------------------------
CREATE TABLE `network_offsets` (
  `id` INT NOT NULL AUTO_INCREMENT,
  `network_id` INT NOT NULL COMMENT '关联 networks.id',
  `last_block` BIGINT NOT NULL,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- =========================================================
-- Seed Data
-- =========================================================

-- 1) 网络初始化
INSERT INTO `networks` (`symbol`, `name`, `rpc_url`, `chain_id`, `crypto_type`)
VALUES
('LOCALHOST', 'Localhost', 'http://127.0.0.1:8545', 31337, 'EVM'),
('BTC', 'Bitcoin', NULL, NULL, 'BITCOIN'),
('ETH', 'Ethereum', 'https://mainnet.infura.io/v3/YOUR_INFURA_KEY', 1, 'EVM'),
('BSC', 'BNB Smart Chain', 'https://bsc-dataseed.binance.org', 56, 'EVM'),
('SOL', 'Solana', 'https://api.mainnet-beta.solana.com', NULL, 'SOLANA');

-- 2) 资产初始化（示例：ETH + USDT）
-- 注意：USDT 合约地址请按你的部署调整
INSERT INTO `assets` (`symbol`, `name`, `decimals`, `is_active`, `network_id`, `contract_address`)
VALUES
  ('ETH',  'Ethereum', 18, 1, (SELECT id FROM networks WHERE symbol='LOCALHOST' ORDER BY id ASC LIMIT 1), NULL),
  ('USDT', 'Tether USD', 6, 1, (SELECT id FROM networks WHERE symbol='LOCALHOST' ORDER BY id ASC LIMIT 1), '0x5FbDB2315678afecb367f032d93F642f64180aa3');

-- 稳定币跨链聚合：默认开启（示例：USDT）
UPDATE `assets` SET `is_aggregate`=1 WHERE `symbol`='USDT';
