package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/NetWeaverGo/core/internal/logger"
)

// 密钥文件名
const credentialKeyFile = ".credential_key"

// 密钥长度 (AES-256 需要 32 字节)
const keyLength = 32

// 加密前缀标识
const encryptedPrefix = "enc:"

// CredentialCipher 凭据加密器
type CredentialCipher struct {
	key []byte
	mu  sync.RWMutex
}

// 全局加密器实例
var (
	credentialCipher     *CredentialCipher
	credentialCipherOnce sync.Once
)

// GetCredentialCipher 获取全局加密器实例（单例）
func GetCredentialCipher() *CredentialCipher {
	credentialCipherOnce.Do(func() {
		credentialCipher = newCredentialCipher()
	})
	return credentialCipher
}

// newCredentialCipher 创建加密器实例
func newCredentialCipher() *CredentialCipher {
	cipher := &CredentialCipher{}
	if err := cipher.initKey(); err != nil {
		logger.Error("Config", "-", "初始化凭据加密器失败: %v", err)
		// 创建临时密钥，确保程序可以继续运行
		cipher.key = make([]byte, keyLength)
		if _, err := rand.Read(cipher.key); err != nil {
			panic(fmt.Sprintf("无法生成临时密钥: %v", err))
		}
	}
	return cipher
}

// initKey 初始化或加载密钥
func (c *CredentialCipher) initKey() error {
	pm := GetPathManager()
	pm.mu.RLock()
	storageRoot := pm.StorageRoot
	pm.mu.RUnlock()

	keyPath := filepath.Join(storageRoot, credentialKeyFile)

	// 尝试加载现有密钥
	if keyData, err := os.ReadFile(keyPath); err == nil {
		key, err := hex.DecodeString(strings.TrimSpace(string(keyData)))
		if err == nil && len(key) == keyLength {
			c.mu.Lock()
			c.key = key
			c.mu.Unlock()
			logger.Info("Config", "-", "已加载凭据加密密钥")
			return nil
		}
		logger.Warn("Config", "-", "现有密钥文件格式无效，将重新生成")
	}

	// 生成新密钥
	key := make([]byte, keyLength)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("生成密钥失败: %w", err)
	}

	// 保存密钥文件（仅所有者可读写）
	keyHex := hex.EncodeToString(key)
	if err := os.WriteFile(keyPath, []byte(keyHex), 0600); err != nil {
		return fmt.Errorf("保存密钥文件失败: %w", err)
	}

	c.mu.Lock()
	c.key = key
	c.mu.Unlock()

	logger.Info("Config", "-", "已生成新的凭据加密密钥: %s", keyPath)
	return nil
}

// Encrypt 加密明文密码
func (c *CredentialCipher) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	c.mu.RLock()
	key := c.key
	c.mu.RUnlock()

	if len(key) == 0 {
		return "", fmt.Errorf("加密器未初始化")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建加密块失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 模式失败: %w", err)
	}

	// 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成 nonce 失败: %w", err)
	}

	// 加密
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// 返回 base64 编码的密文，带前缀标识
	return encryptedPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密密文密码
func (c *CredentialCipher) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// 检查是否是加密格式
	if !strings.HasPrefix(ciphertext, encryptedPrefix) {
		// 不是加密格式，直接返回（兼容旧数据）
		return ciphertext, nil
	}

	// 去除前缀
	ciphertext = strings.TrimPrefix(ciphertext, encryptedPrefix)

	c.mu.RLock()
	key := c.key
	c.mu.RUnlock()

	if len(key) == 0 {
		return "", fmt.Errorf("加密器未初始化")
	}

	// Base64 解码
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("base64 解码失败: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建解密块失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 模式失败: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("密文长度不足")
	}

	// 提取 nonce 和密文
	nonce, encryptedData := data[:nonceSize], data[nonceSize:]

	// 解密
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %w", err)
	}

	return string(plaintext), nil
}

// IsEncrypted 检查字符串是否为加密格式
func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, encryptedPrefix)
}

// Reencrypt 重新加密（用于密钥轮换）
func (c *CredentialCipher) Reencrypt(ciphertext string) (string, error) {
	plaintext, err := c.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return c.Encrypt(plaintext)
}
