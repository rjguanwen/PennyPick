// Package crypto 提供拾财数据库文件的整库 AES-256-GCM 加解密能力。
//
// 加密文件格式：
//
//	┌─────────┬─────────┬──────────┬──────────┬────────────────────────────┐
//	│ magic   │ version │ salt     │ nonce    │ AES-GCM 密文 (+16B tag)   │
//	│ 8B      │ 2B(u16) │ 16B      │ 12B      │ 变长                        │
//	└─────────┴─────────┴──────────┴──────────┴────────────────────────────┘
//
// 密钥由主密码经 Argon2id 派生（memory=64MB, iter=3, parallel=4），
// 每次加密随机生成 salt 与 nonce。同一密码每次加密产物均不同。
package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/argon2"
)

const (
	// Magic 文件头标识，用于识别加密文件与校验格式。
	Magic = "PPENC1\x00\x00"
	// Version 当前加密格式版本。
	Version = uint16(1)

	SaltLen  = 16
	NonceLen = 12
	KeyLen   = 32 // AES-256

	// HeaderLen 固定头长度 = 8 + 2 + 16 + 12。
	HeaderLen = len(Magic) + 2 + SaltLen + NonceLen

	// Argon2id 参数：个人应用一次启动仅派生一次密钥，取安全档位。
	argonMemory      = 64 * 1024 // 64 MB
	argonIterations  = 3
	argonParallelism = 4
)

// ErrBadMagic 表示文件不是拾财加密数据库格式。
var ErrBadMagic = errors.New("not a pennypick encrypted database (bad magic)")

// DeriveKey 由主密码与盐派生 32 字节 AES 密钥。
func DeriveKey(passphrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(passphrase), salt, argonIterations, argonMemory, argonParallelism, KeyLen)
}

// EncryptBytes 将原始字节加密为带头的密文。
func EncryptBytes(data []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, SaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	nonce := make([]byte, NonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	block, err := aes.NewCipher(DeriveKey(passphrase, salt))
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	out := make([]byte, HeaderLen, HeaderLen+len(data)+gcm.Overhead())
	copy(out, Magic)
	binary.BigEndian.PutUint16(out[len(Magic):], Version)
	copy(out[len(Magic)+2:], salt)
	copy(out[len(Magic)+2+SaltLen:], nonce)
	out = gcm.Seal(out, nonce, data, nil)
	return out, nil
}

// DecryptBytes 校验头并用密码解密。
func DecryptBytes(enc []byte, passphrase string) ([]byte, error) {
	if len(enc) < HeaderLen || !bytes.Equal(enc[:len(Magic)], []byte(Magic)) {
		return nil, ErrBadMagic
	}
	ver := binary.BigEndian.Uint16(enc[len(Magic) : len(Magic)+2])
	if ver != Version {
		return nil, fmt.Errorf("unsupported encrypted format version: %d", ver)
	}
	salt := enc[len(Magic)+2 : len(Magic)+2+SaltLen]
	nonce := enc[len(Magic)+2+SaltLen : len(Magic)+2+SaltLen+NonceLen]
	block, err := aes.NewCipher(DeriveKey(passphrase, salt))
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	plain, err := gcm.Open(nil, nonce, enc[HeaderLen:], nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed (wrong passphrase or corrupted file): %w", err)
	}
	return plain, nil
}

// EncryptFile 将 src 明文文件加密为 dst（原子写：先写 dst.tmp 再 rename）。
func EncryptFile(src, dst, passphrase string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read source file: %w", err)
	}
	enc, err := EncryptBytes(data, passphrase)
	if err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, enc, 0o600); err != nil {
		return fmt.Errorf("write temp encrypted file: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace encrypted file: %w", err)
	}
	return nil
}

// DecryptFile 读取加密文件并解密。
func DecryptFile(src, passphrase string) ([]byte, error) {
	enc, err := os.ReadFile(src)
	if err != nil {
		return nil, fmt.Errorf("read encrypted file: %w", err)
	}
	return DecryptBytes(enc, passphrase)
}

// IsEncryptedFile 判断文件是否为拾财加密数据库格式（不存在视为 false）。
func IsEncryptedFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()
	buf := make([]byte, len(Magic))
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return false, err
	}
	return n == len(Magic) && bytes.Equal(buf, []byte(Magic)), nil
}
