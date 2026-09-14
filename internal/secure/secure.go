package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"golang.org/x/crypto/argon2"
	"os"
	"path/filepath"
	"strings"
)

func Random(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func Hash(s string) string   { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func Equal(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }
func Password(s string) string {
	salt := make([]byte, 16)
	rand.Read(salt)
	key := argon2.IDKey([]byte(s), salt, 2, 32*1024, 2, 32)
	return base64.RawStdEncoding.EncodeToString(salt) + "." + base64.RawStdEncoding.EncodeToString(key)
}
func CheckPassword(encoded, password string) bool {
	parts := strings.Split(encoded, ".")
	if len(parts) != 2 {
		return false
	}
	salt, e := base64.RawStdEncoding.DecodeString(parts[0])
	if e != nil || len(salt) != 16 {
		return false
	}
	key, e := base64.RawStdEncoding.DecodeString(parts[1])
	if e != nil || len(key) != 32 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, 2, 32*1024, 2, 32)
	return subtle.ConstantTimeCompare(key, got) == 1
}

type Vault struct {
	aead cipher.AEAD
	Key  []byte
}

func OpenVault(dir string, existing bool) (*Vault, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "master.key")
	var key []byte
	if env := os.Getenv("MCPDBHUB_MASTER_KEY"); env != "" {
		var e error
		key, e = base64.StdEncoding.DecodeString(env)
		if e != nil {
			return nil, errors.New("MCPDBHUB_MASTER_KEY must be base64")
		}
	} else {
		var e error
		key, e = os.ReadFile(path)
		if os.IsNotExist(e) {
			if existing {
				return nil, errors.New("master key missing for existing configuration")
			}
			key = make([]byte, 32)
			rand.Read(key)
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return nil, err
			}
			_, e = f.Write(key)
			if err := f.Close(); e == nil {
				e = err
			}
		}
		if e != nil {
			return nil, e
		}
	}
	if len(key) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	a, e := cipher.NewGCM(block)
	return &Vault{aead: a, Key: key}, e
}
func (v *Vault) Seal(data []byte, context string) string {
	n := make([]byte, v.aead.NonceSize())
	rand.Read(n)
	return base64.RawURLEncoding.EncodeToString(v.aead.Seal(n, n, data, []byte(context)))
}
func (v *Vault) Open(s, context string) ([]byte, error) {
	b, e := base64.RawURLEncoding.DecodeString(s)
	if e != nil || len(b) < v.aead.NonceSize() {
		return nil, errors.New("invalid encrypted value")
	}
	n := v.aead.NonceSize()
	data, e := v.aead.Open(nil, b[:n], b[n:], []byte(context))
	if e != nil {
		return nil, fmt.Errorf("cannot decrypt value: %w", e)
	}
	return data, nil
}
