package walletgen

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mr-tron/base58"
)

// 与 networks.crypto_type 取值一致
const (
	TypeEVM     = "EVM"
	TypeBitcoin = "BITCOIN"
	TypeSolana  = "SOLANA"
)

// Generated 托管入库前的明文材料（私钥经 MasterKey 加密后再写入 DB）
type Generated struct {
	Address          string
	PrivKeyPlaintext string
}

// GenerateEVM secp256k1 + Keccak 地址；密文存 hex 编码私钥（与现有逻辑一致）
func GenerateEVM() (Generated, error) {
	// 生成 secp256k1 私钥
	priv, err := crypto.GenerateKey()
	if err != nil {
		return Generated{}, err
	}
	// 生成地址
	addr := crypto.PubkeyToAddress(priv.PublicKey).Hex()
	// 生成私钥
	privBytes := crypto.FromECDSA(priv)
	// 生成私钥 hex 编码
	privHex := hex.EncodeToString(privBytes)
	// 返回生成的钱包信息
	return Generated{Address: addr, PrivKeyPlaintext: privHex}, nil
}

// GenerateBitcoin 随机 secp256k1，P2WPKH（bc1… 主网）；私钥以 WIF 压缩格式存库便于后续签名
func GenerateBitcoin() (Generated, error) {
	// 生成随机 secp256k1 私钥
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		return Generated{}, err
	}
	// 生成 WIF 压缩格式私钥
	wif, err := btcutil.NewWIF(privKey, &chaincfg.MainNetParams, true)
	if err != nil {
		return Generated{}, err
	}
	// 生成公钥
	pubKey := privKey.PubKey()
	// 生成公钥哈希
	pubKeyHash := btcutil.Hash160(pubKey.SerializeCompressed())
	// 生成地址
	addr, err := btcutil.NewAddressWitnessPubKeyHash(pubKeyHash, &chaincfg.MainNetParams)
	if err != nil {
		return Generated{}, err
	}
	// 返回生成的钱包信息
	return Generated{
		Address:          addr.EncodeAddress(),
		PrivKeyPlaintext: wif.String(),
	}, nil
}

// GenerateSolana ed25519，地址为公钥 32 字节的 Base58；私钥存 64 字节 hex（Go ed25519 私钥格式）
func GenerateSolana() (Generated, error) {
	// 生成 ed25519 公钥和私钥
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Generated{}, err
	}
	// 返回生成的钱包信息
	return Generated{
		Address:          base58.Encode(pub),
		PrivKeyPlaintext: hex.EncodeToString(priv),
	}, nil
}

// GenerateByCryptoType 按 networks.crypto_type 分派
func GenerateByCryptoType(cryptoType string) (Generated, error) {
	t := strings.ToUpper(strings.TrimSpace(cryptoType))
	switch t {
	case TypeEVM:
		return GenerateEVM()
	case TypeBitcoin:
		return GenerateBitcoin()
	case TypeSolana:
		return GenerateSolana()
	default:
		return Generated{}, fmt.Errorf("unsupported crypto_type: %q", cryptoType)
	}
}
