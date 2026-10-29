// Package backup turns the settings into a compressed, password-protected
// file and back.
//
// File layout:  "HCBACKUP" | version (1 byte) | PBKDF2 iterations (uint32 BE)
//
//	| salt (16) | nonce (12) | AES-256-GCM( gzip(contents) )
//
// The key is PBKDF2-SHA256(password, salt, iterations). The header is
// authenticated too (GCM additional data), so any change, a cut file or a
// wrong password is always detected and nothing half-read is used.
package backup

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	Magic             = "HCBACKUP"
	formatVersion     = 1
	DefaultIterations = 600_000 // OWASP 2023 recommendation for PBKDF2-SHA256
	MinPassword       = 8
	maxIterations     = 10_000_000
	maxPlain          = 16 << 20 // a backup is a few KB; refuse "zip bombs"
	saltLen           = 16
	nonceLen          = 12
	headerLen         = len(Magic) + 1 + 4 + saltLen + nonceLen
)

// Iterations is used when sealing (tests lower it).
var Iterations = DefaultIterations

var (
	ErrNotBackup     = errors.New("not a housecontrol backup file")
	ErrWrongPassword = errors.New("wrong password, or the file is damaged")
	ErrWeakPassword  = fmt.Errorf("the password must be at least %d characters", MinPassword)
)

// IsSealed reports whether data is a password-protected backup (else it may
// be an unprotected JSON backup).
func IsSealed(data []byte) bool { return bytes.HasPrefix(data, []byte(Magic)) }

// Seal compresses and encrypts plain with password.
func Seal(plain []byte, password string) ([]byte, error) {
	if len([]rune(password)) < MinPassword {
		return nil, ErrWeakPassword
	}
	var zipped bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&zipped, gzip.BestCompression)
	if _, err := zw.Write(plain); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}

	header := make([]byte, headerLen)
	copy(header, Magic)
	header[len(Magic)] = formatVersion
	binary.BigEndian.PutUint32(header[len(Magic)+1:], uint32(Iterations))
	salt := header[len(Magic)+5 : len(Magic)+5+saltLen]
	nonce := header[len(Magic)+5+saltLen:]
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	gcm, err := newGCM(password, salt, Iterations)
	if err != nil {
		return nil, err
	}
	return gcm.Seal(header, nonce, zipped.Bytes(), header), nil
}

// Open decrypts and decompresses a file made by Seal.
func Open(data []byte, password string) ([]byte, error) {
	if len(data) < headerLen+16 || !bytes.HasPrefix(data, []byte(Magic)) {
		return nil, ErrNotBackup
	}
	if data[len(Magic)] != formatVersion {
		return nil, fmt.Errorf("%w (made by a newer version of housecontrol)", ErrNotBackup)
	}
	iter := int(binary.BigEndian.Uint32(data[len(Magic)+1:]))
	if iter < 1 || iter > maxIterations {
		return nil, ErrWrongPassword // only a damaged header gets here
	}
	header := data[:headerLen]
	salt := header[len(Magic)+5 : len(Magic)+5+saltLen]
	nonce := header[len(Magic)+5+saltLen:]
	gcm, err := newGCM(password, salt, iter)
	if err != nil {
		return nil, err
	}
	zipped, err := gcm.Open(nil, nonce, data[headerLen:], header)
	if err != nil {
		return nil, ErrWrongPassword
	}
	zr, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return nil, ErrNotBackup
	}
	plain, err := io.ReadAll(io.LimitReader(zr, maxPlain+1))
	if err != nil || len(plain) > maxPlain {
		return nil, ErrNotBackup
	}
	return plain, nil
}

func newGCM(password string, salt []byte, iter int) (cipher.AEAD, error) {
	key, err := pbkdf2.Key(sha256.New, password, salt, iter, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
