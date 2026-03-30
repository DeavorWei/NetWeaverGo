package sshutil

import (
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"golang.org/x/crypto/ssh"
)

// SecurePreset 安全优先预设（仅使用现代安全算法）
// 适用于新设备，追求最高安全性
var SecurePreset = models.SSHAlgorithmSettings{
	Ciphers: []string{
		// 官方推荐的最安全的现代算法（AEAD）
		ssh.CipherAES128GCM,
		ssh.CipherAES256GCM,
		ssh.CipherChaCha20Poly1305,
		// 安全的传统对称加密算法（CTR 模式）
		ssh.CipherAES128CTR,
		ssh.CipherAES192CTR,
		ssh.CipherAES256CTR,
	},
	KeyExchanges: []string{
		// 官方推荐的最安全的现代算法（包含抗量子和椭圆曲线）
		ssh.KeyExchangeMLKEM768X25519,
		ssh.KeyExchangeCurve25519,
		ssh.KeyExchangeECDHP256,
		ssh.KeyExchangeECDHP384,
		ssh.KeyExchangeECDHP521,
		// 安全的传统 DH 算法
		ssh.KeyExchangeDH14SHA256,
		ssh.KeyExchangeDH16SHA512,
		ssh.KeyExchangeDHGEXSHA256,
	},
	MACs: []string{
		// 官方推荐的最安全的现代算法（AEAD 模式不需要 MAC，但以防万一也可配置）
		ssh.HMACSHA256ETM,
		ssh.HMACSHA512ETM,
		// 安全的传统哈希算法
		ssh.HMACSHA256,
		ssh.HMACSHA512,
	},
	HostKeyAlgorithms: []string{
		// 官方推荐的最安全的现代算法（椭圆曲线和 ED25519）
		ssh.KeyAlgoED25519,
		ssh.KeyAlgoECDSA256,
		ssh.KeyAlgoECDSA384,
		ssh.KeyAlgoECDSA521,
		// 基于硬件安全密钥 (SK, FIDO2/U2F) 的现代算法
		ssh.KeyAlgoSKED25519,
		ssh.KeyAlgoSKECDSA256,
		// 基于 OpenSSH 证书签发的现代算法
		ssh.CertAlgoED25519v01,
		ssh.CertAlgoECDSA256v01,
		ssh.CertAlgoECDSA384v01,
		ssh.CertAlgoECDSA521v01,
		ssh.CertAlgoSKED25519v01,
		ssh.CertAlgoSKECDSA256v01,
		ssh.CertAlgoRSASHA512v01,
		ssh.CertAlgoRSASHA256v01,
		// 安全的传统 RSA 算法（使用 SHA2）
		ssh.KeyAlgoRSASHA512,
		ssh.KeyAlgoRSASHA256,
	},
	PresetMode: "secure",
}

// CompatiblePreset 兼容性预设（包含老旧设备支持的算法）
// 适用于需要连接各种老旧网络设备的场景
var CompatiblePreset = models.SSHAlgorithmSettings{
	Ciphers: []string{
		// 现代安全算法
		ssh.CipherAES128GCM,
		ssh.CipherAES256GCM,
		ssh.CipherChaCha20Poly1305,
		ssh.CipherAES128CTR,
		ssh.CipherAES192CTR,
		ssh.CipherAES256CTR,
		// 兼容老旧设备的不安全算法
		ssh.InsecureCipherAES128CBC,
		"aes192-cbc", // golang.org/x/crypto/ssh 默认未公开此常量
		"aes256-cbc",
		ssh.InsecureCipherTripleDESCBC,
		ssh.InsecureCipherRC4,
		ssh.InsecureCipherRC4128,
		ssh.InsecureCipherRC4256,
	},
	KeyExchanges: []string{
		// 现代安全算法
		ssh.KeyExchangeMLKEM768X25519,
		ssh.KeyExchangeCurve25519,
		ssh.KeyExchangeECDHP256,
		ssh.KeyExchangeECDHP384,
		ssh.KeyExchangeECDHP521,
		ssh.KeyExchangeDH14SHA256,
		ssh.KeyExchangeDH16SHA512,
		ssh.KeyExchangeDHGEXSHA256,
		// 兼容老旧设备
		ssh.InsecureKeyExchangeDH14SHA1,  // 官方默认包含了这个
		ssh.InsecureKeyExchangeDH1SHA1,   // diffie-hellman-group1-sha1
		ssh.InsecureKeyExchangeDHGEXSHA1, // diffie-hellman-group-exchange-sha1
	},
	MACs: []string{
		// 现代安全算法
		ssh.HMACSHA256ETM,
		ssh.HMACSHA512ETM,
		ssh.HMACSHA256,
		ssh.HMACSHA512,
		// 兼容老旧设备
		ssh.HMACSHA1,
		ssh.InsecureHMACSHA196,
	},
	HostKeyAlgorithms: []string{
		// 现代安全算法
		ssh.KeyAlgoED25519,
		ssh.KeyAlgoECDSA256,
		ssh.KeyAlgoECDSA384,
		ssh.KeyAlgoECDSA521,
		// 基于硬件安全密钥 (SK, FIDO2/U2F) 的现代算法
		ssh.KeyAlgoSKED25519,
		ssh.KeyAlgoSKECDSA256,
		// 基于 OpenSSH 证书签发的现代/传统算法
		ssh.CertAlgoED25519v01,
		ssh.CertAlgoECDSA256v01,
		ssh.CertAlgoECDSA384v01,
		ssh.CertAlgoECDSA521v01,
		ssh.CertAlgoSKED25519v01,
		ssh.CertAlgoSKECDSA256v01,
		ssh.CertAlgoRSASHA512v01,
		ssh.CertAlgoRSASHA256v01,
		ssh.CertAlgoRSAv01,
		// 安全的传统 RSA 算法（使用 SHA2）
		ssh.KeyAlgoRSASHA512,
		ssh.KeyAlgoRSASHA256,
		// 兼容老旧设备添加的不安全算法或弃用证书（SHA1 和 DSS）
		ssh.KeyAlgoRSA,
		ssh.InsecureKeyAlgoDSA,
		ssh.InsecureCertAlgoDSAv01,
	},
	PresetMode: "compatible",
}

// GetPreset 根据预设模式返回对应的算法配置
// 如果模式为空或 "custom"，返回 nil 表示使用自定义配置
func GetPreset(presetMode string) *models.SSHAlgorithmSettings {
	switch presetMode {
	case "secure":
		preset := SecurePreset
		return &preset
	case "compatible":
		preset := CompatiblePreset
		return &preset
	default:
		return nil
	}
}

// GetEffectiveAlgorithms 获取实际生效的算法配置
// 优先级：自定义配置 > 预设配置 > 内置默认配置
func GetEffectiveAlgorithms(settings models.SSHAlgorithmSettings) (ciphers, keyExchanges, macs, hostKeyAlgorithms []string) {
	logger.Debug("SSH", "-", "GetEffectiveAlgorithms 被调用: PresetMode=%s", settings.PresetMode)
	logger.Debug("SSH", "-", "  - 输入 Ciphers(%d): %v", len(settings.Ciphers), settings.Ciphers)
	logger.Debug("SSH", "-", "  - 输入 KeyExchanges(%d): %v", len(settings.KeyExchanges), settings.KeyExchanges)
	logger.Debug("SSH", "-", "  - 输入 MACs(%d): %v", len(settings.MACs), settings.MACs)
	logger.Debug("SSH", "-", "  - 输入 HostKeyAlgorithms(%d): %v", len(settings.HostKeyAlgorithms), settings.HostKeyAlgorithms)

	// 如果是自定义模式且有配置，使用自定义配置
	if settings.PresetMode == "custom" {
		logger.Debug("SSH", "-", "  进入 custom 模式处理逻辑")
		if len(settings.Ciphers) > 0 {
			ciphers = settings.Ciphers
		}
		if len(settings.KeyExchanges) > 0 {
			keyExchanges = settings.KeyExchanges
		}
		if len(settings.MACs) > 0 {
			macs = settings.MACs
		}
		if len(settings.HostKeyAlgorithms) > 0 {
			hostKeyAlgorithms = settings.HostKeyAlgorithms
		}
		// 如果自定义模式下有任何配置，直接返回
		hasConfig := len(ciphers) > 0 || len(keyExchanges) > 0 || len(macs) > 0 || len(hostKeyAlgorithms) > 0
		logger.Debug("SSH", "-", "  custom 模式 hasConfig=%v", hasConfig)
		if hasConfig {
			logger.Debug("SSH", "-", "  使用自定义配置返回")
			return
		}
		logger.Debug("SSH", "-", "  custom 模式无配置，继续向下执行")
	}

	// 尝试获取预设配置
	preset := GetPreset(settings.PresetMode)
	logger.Debug("SSH", "-", "  GetPreset(%s) 返回: %v", settings.PresetMode, preset != nil)
	if preset != nil {
		ciphers = preset.Ciphers
		keyExchanges = preset.KeyExchanges
		macs = preset.MACs
		hostKeyAlgorithms = preset.HostKeyAlgorithms
		logger.Debug("SSH", "-", "  使用预设配置返回: %s", settings.PresetMode)
		return
	}

	// 返回空，表示使用内置默认配置
	logger.Debug("SSH", "-", "  返回空(将使用内置默认配置)")
	return nil, nil, nil, nil
}
