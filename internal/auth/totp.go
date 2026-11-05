package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"time"
)

var totpEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func FactorSecret() string {
	var b [20]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return totpEncoding.EncodeToString(b[:])
}

func FactorURI(secret, account string) string {
	q := url.Values{"secret": {secret}, "issuer": {"Shipper"}}
	return "otpauth://totp/" + url.PathEscape("Shipper:"+account) + "?" + q.Encode()
}

func factorCode(key []byte, step int64) string {
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step))
	h := hmac.New(sha1.New, key)
	h.Write(counter[:])
	sum := h.Sum(nil)
	offset := sum[len(sum)-1] & 15
	return fmt.Sprintf("%06d", (binary.BigEndian.Uint32(sum[offset:offset+4])&0x7fffffff)%1000000)
}

// MatchFactor accepts at most one step of clock drift. The caller must persist
// the accepted counter atomically and reject its reuse across sessions.
func MatchFactor(secret, code string, now time.Time) (int64, bool) {
	key, err := totpEncoding.DecodeString(secret)
	if err != nil || len(code) != 6 {
		return 0, false
	}
	current := now.Unix() / 30
	var matched int64
	valid := false
	for step := current - 1; step <= current+1; step++ {
		if subtle.ConstantTimeCompare([]byte(factorCode(key, step)), []byte(code)) == 1 {
			matched, valid = step, true
		}
	}
	return matched, valid
}
