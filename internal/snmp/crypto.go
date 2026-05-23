// Package snmp 提供 SNMP 核心业务功能
// crypto.go 实现 SNMP 凭据加密/解密，使用 AES-256-GCM 算法
package snmp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
)

// ============================================================================
// 常量定义
// ============================================================================

// 密钥文件名（存储在 SNMP 数据目录下）
const keyFileName = "snmp_key.bin"

// 旧版硬编码密钥（仅用于迁移旧数据，不再用于新加密）
// 迁移完成后此常量将被移除
var legacyHardcodedKey = []byte("netweaver-snmp-default-key-32b!")

// ============================================================================
// 全局加密管理器
// ============================================================================

var (
	globalCrypto     *CredentialCrypto
	globalCryptoOnce sync.Once
	globalCryptoErr  error
)

// GetCredentialCrypto 获取全局凭据加密管理器（单例）
// 如果密钥初始化失败，返回 nil
func GetCredentialCrypto() *CredentialCrypto {
	globalCryptoOnce.Do(func() {
		key, err := loadOrGenerateKey()
		if err != nil {
			globalCryptoErr = err
			logger.Error("SNMP", "-", "加密密钥初始化失败: %v", err)
			return
		}
		globalCrypto = NewCredentialCrypto(key)
	})
	return globalCrypto
}

// GetCredentialCryptoErr 获取密钥初始化错误（用于诊断）
func GetCredentialCryptoErr() error {
	return globalCryptoErr
}

// getKeyFilePath 获取密钥文件路径
func getKeyFilePath() string {
	pm := config.GetPathManager()
	// 密钥文件存储在 SNMP 数据目录下（与数据库同级）
	snmpDataDir := filepath.Join(pm.StorageRoot, "snmp")
	return filepath.Join(snmpDataDir, keyFileName)
}

// loadOrGenerateKey 从环境变量、密钥文件加载或自动生成新密钥
// 返回 32-byte AES-256 密钥
func loadOrGenerateKey() ([]byte, error) {
	// 优先级 1: 环境变量（最高优先级，适用于容器化部署）
	envKey := os.Getenv("NETWEAVER_SNMP_KEY")
	if envKey != "" {
		decoded, err := base64.StdEncoding.DecodeString(envKey)
		if err == nil && len(decoded) == 32 {
			logger.Info("SNMP", "-", "使用环境变量 NETWEAVER_SNMP_KEY 作为加密密钥")
			return decoded, nil
		}
		logger.Warn("SNMP", "-", "环境变量 NETWEAVER_SNMP_KEY 格式无效（需要 Base64 编码的 32 字节密钥），将尝试从文件加载")
	}

	// 优先级 2: 从密钥文件加载
	keyFilePath := getKeyFilePath()
	if key, err := loadKeyFile(keyFilePath); err == nil {
		logger.Info("SNMP", "-", "从密钥文件加载加密密钥: %s", keyFilePath)
		return key, nil
	} else if !os.IsNotExist(err) {
		// 文件存在但读取失败（权限问题或损坏）
		logger.Warn("SNMP", "-", "密钥文件读取失败: %v，将尝试生成新密钥", err)
	}

	// 优先级 3: 自动生成新密钥并写入文件
	logger.Info("SNMP", "-", "密钥文件不存在，自动生成新的随机密钥")
	key, err := generateAndSaveKey(keyFilePath)
	if err != nil {
		return nil, fmt.Errorf("生成并保存密钥失败: %w", err)
	}

	logger.Info("SNMP", "-", "SNMP 凭据加密密钥已初始化，密钥文件: %s", keyFilePath)
	return key, nil
}

// loadKeyFile 从文件加载密钥
func loadKeyFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) != 32 {
		return nil, fmt.Errorf("密钥文件长度无效（期望 32 字节，实际 %d）", len(data))
	}
	return data, nil
}

// generateAndSaveKey 生成随机密钥并保存到文件
func generateAndSaveKey(path string) ([]byte, error) {
	// 生成 32-byte AES-256 密钥
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("生成随机密钥失败: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建密钥目录失败: %w", err)
	}

	// 写入密钥文件（权限 0600：仅所有者可读写）
	if err := writeKeyFile(path, key); err != nil {
		return nil, fmt.Errorf("写入密钥文件失败: %w", err)
	}

	return key, nil
}

// writeKeyFile 写入密钥文件（权限 0600）
func writeKeyFile(path string, key []byte) error {
	// 使用 0600 权限写入（仅所有者可读写，防止其他用户访问）
	err := os.WriteFile(path, key, 0600)
	if err != nil {
		return err
	}
	logger.Info("SNMP", "-", "密钥文件已写入（权限 0600）: %s", path)
	return nil
}

// ============================================================================
// CredentialCrypto 实现
// ============================================================================

// NewCredentialCrypto 创建凭据加密管理器实例
// 要求 key 必须为 32 字节，否则返回 nil
func NewCredentialCrypto(key []byte) *CredentialCrypto {
	if len(key) != 32 {
		logger.Error("SNMP", "-", "加密密钥长度无效 (期望 32, 实际 %d)", len(key))
		return nil
	}

	return &CredentialCrypto{key: key}
}

// TryDecryptWithLegacyKey 尝试使用旧版硬编码密钥解密数据
// 用于迁移旧密钥加密的凭据数据
// 返回：解密成功返回明文和 true，解密失败返回空字符串和 false
func (c *CredentialCrypto) TryDecryptWithLegacyKey(ciphertext string) (string, bool) {
	if ciphertext == "" {
		return "", false
	}

	// 创建旧版密钥的加密器
	legacyCrypto := &CredentialCrypto{key: legacyHardcodedKey}

	// 尝试解密
	plaintext, err := legacyCrypto.DecryptCredential(ciphertext)
	if err != nil {
		return "", false
	}

	return plaintext, true
}

// MigrateCredential 迁移单个凭据字段
// 如果数据是用旧密钥加密的，解密后用新密钥重新加密
func (c *CredentialCrypto) MigrateCredential(ciphertext string) (string, bool, error) {
	if ciphertext == "" {
		return "", false, nil // 空字符串无需迁移
	}

	// 先尝试用当前密钥解密（判断是否已是新密钥加密）
	_, err := c.DecryptCredential(ciphertext)
	if err == nil {
		// 当前密钥可以解密，说明已是新密钥加密或未加密，无需迁移
		return ciphertext, false, nil
	}

	// 当前密钥无法解密，尝试用旧版密钥解密
	plaintext, ok := c.TryDecryptWithLegacyKey(ciphertext)
	if !ok {
		// 两种密钥都无法解密，数据可能损坏或使用了其他密钥
		return ciphertext, false, fmt.Errorf("凭据数据无法解密（密钥不匹配或数据损坏）")
	}

	// 使用新密钥重新加密
	newCiphertext, err := c.EncryptCredential(plaintext)
	if err != nil {
		return ciphertext, false, fmt.Errorf("重新加密凭据失败: %w", err)
	}

	return newCiphertext, true, nil
}

// EncryptedPrefix 加密数据前缀标记，用于可靠识别加密数据
const EncryptedPrefix = "ENC1:"

// EncryptCredential 加密凭据明文
// 返回带前缀的 Base64 编码密文（ENC1: + nonce + ciphertext + tag）
func (c *CredentialCrypto) EncryptCredential(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil // 空字符串不加密
	}

	c.mu.RLock()
	key := c.key
	c.mu.RUnlock()

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM mode 失败: %w", err)
	}

	// 生成随机 nonce（GCM 标准 nonce 大小为 12 bytes）
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成 nonce 失败: %w", err)
	}

	// 加密数据（Seal 自动附加 authentication tag）
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// 返回带前缀的 Base64 编码结果
	return EncryptedPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptCredential 解密凭据密文
// 输入为带前缀的 Base64 编码密文（ENC1: + nonce + ciphertext + tag）
// 同时兼容旧格式（无前缀）的密文
func (c *CredentialCrypto) DecryptCredential(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil // 空字符串不解密
	}

	// 检查并去除前缀
	dataStr := ciphertext
	if strings.HasPrefix(ciphertext, EncryptedPrefix) {
		dataStr = strings.TrimPrefix(ciphertext, EncryptedPrefix)
	}

	// Base64 解码
	data, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		return "", fmt.Errorf("Base64 解码失败: %w", err)
	}

	c.mu.RLock()
	key := c.key
	c.mu.RUnlock()

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM mode 失败: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("密文长度不足，缺少 nonce")
	}

	// 分离 nonce 和实际密文
	nonce, encryptedData := data[:nonceSize], data[nonceSize:]

	// 解密并验证 authentication tag
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败（密钥不匹配或数据损坏）: %w", err)
	}

	return string(plaintext), nil
}

// ============================================================================
// 辅助函数
// ============================================================================

// IsEncrypted 判断字符串是否已加密
// 新格式：检查 ENC1: 前缀，可靠识别加密数据
// 旧格式兼容：尝试 Base64 解码并检查长度是否足够包含 nonce + tag
func IsEncrypted(s string) bool {
	if s == "" {
		return false
	}

	// 新格式：检查前缀标记（可靠）
	if strings.HasPrefix(s, EncryptedPrefix) {
		return true
	}

	// 旧格式兼容：尝试 Base64 解码并检查长度
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return false
	}
	// 检查长度是否足够包含 nonce + tag（最小 12 + 16 = 28 bytes）
	return len(decoded) >= 28
}

// RotateKey 密钥轮换（生成新密钥并重新加密所有凭据）
// 注意：此函数仅生成新密钥，实际重新加密需要在应用层完成
func (c *CredentialCrypto) RotateKey() ([]byte, error) {
	newKey := make([]byte, 32)
	if _, err := rand.Read(newKey); err != nil {
		return nil, fmt.Errorf("生成新密钥失败: %w", err)
	}

	// 返回新密钥的 Base64 编码，供存储到环境变量或配置文件
	return newKey, nil
}

// SetKey 设置新密钥（用于密钥轮换后更新）
// 使用写锁保证并发安全
func (c *CredentialCrypto) SetKey(newKey []byte) error {
	if len(newKey) != 32 {
		return fmt.Errorf("密钥长度必须为 32 bytes")
	}
	c.mu.Lock()
	c.key = newKey
	c.mu.Unlock()
	return nil
}

// GenerateKeyForExport 生成密钥并返回 Base64 编码字符串
// 用于首次部署时生成密钥并导出到环境变量
func GenerateKeyForExport() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("生成密钥失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}