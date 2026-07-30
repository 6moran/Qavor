package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"sync"
)

var (
	// encryptionKey 加密密钥，从环境变量读取
	encryptionKey []byte
	// keyOnce 确保密钥只初始化一次
	keyOnce sync.Once
	// keyInitErr 密钥初始化错误
	keyInitErr error
)

// getEncryptionKey 获取加密密钥
func getEncryptionKey() ([]byte, error) {
	keyOnce.Do(func() {
		// 从环境变量读取密钥
		key := os.Getenv("ENCRYPTION_KEY")
		if key == "" {
			// 使用默认密钥（仅用于开发环境）
			key = "qavor-default-key-32bytes!!"
		}

		// 确保密钥长度为 32 字节（AES-256）
		if len(key) < 32 {
			key = key + "!!!!!!!!!!!!!!!!!!!!!!!!!!!!"[:32-len(key)]
		} else if len(key) > 32 {
			key = key[:32]
		}

		encryptionKey = []byte(key)
	})

	return encryptionKey, keyInitErr
}

// Encrypt 加密字符串
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	key, err := getEncryptionKey()
	if err != nil {
		return "", fmt.Errorf("获取加密密钥失败: %w", err)
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	// 使用 CFB 模式
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("生成 IV 失败: %w", err)
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(plaintext))

	// Base64 编码
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密字符串
func Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	key, err := getEncryptionKey()
	if err != nil {
		return "", fmt.Errorf("获取加密密钥失败: %w", err)
	}

	// Base64 解码
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("Base64 解码失败: %w", err)
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	// 检查数据长度
	if len(data) < aes.BlockSize {
		return "", fmt.Errorf("密文长度无效")
	}

	// 使用 CFB 模式
	iv := data[:aes.BlockSize]
	data = data[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(data, data)

	return string(data), nil
}

// EncryptAPIKey 加密 API Key（别名）
func EncryptAPIKey(apiKey string) (string, error) {
	return Encrypt(apiKey)
}

// DecryptAPIKey 解密 API Key（别名）
func DecryptAPIKey(encrypted string) (string, error) {
	return Decrypt(encrypted)
}
