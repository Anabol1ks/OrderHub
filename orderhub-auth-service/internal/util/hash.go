package util

import (
	"crypto/sha256"
	"encoding/base64"
)

// sha256Base64URL возвращает base64url (без паддинга) от sha256(plain)
func Sha256Base64URL(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
