package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
)

var KeyPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func KeyOf(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
