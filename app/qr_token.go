package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

const qrTokenLifetime = 5 * time.Minute

var errBadQRToken = errors.New("QR token is invalid or expired")

// A QR token is "<generation>.<expiry>.<signature>". Rotating the QR code
// moves the room to a new generation.
func (r room) newQRToken(now time.Time) string {
	claims := r.qrGenerationClaim() + "." + strconv.FormatInt(now.Add(qrTokenLifetime).Unix(), 10)
	return claims + "." + r.signQRClaims(claims)
}

func (r room) qrGenerationClaim() string { return strconv.Itoa(r.qrGeneration) }

func (r room) checkQRToken(token string, now time.Time) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errBadQRToken
	}
	generation, expiry, signature := parts[0], parts[1], parts[2]
	if !hmac.Equal([]byte(signature), []byte(r.signQRClaims(generation+"."+expiry))) {
		return errBadQRToken
	}
	expirySeconds, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil || generation != r.qrGenerationClaim() || !now.Before(time.Unix(expirySeconds, 0)) {
		return errBadQRToken
	}
	return nil
}

func (r room) signQRClaims(claims string) string {
	mac := hmac.New(sha256.New, []byte(r.qrKey))
	mac.Write([]byte(r.code + "." + claims))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *roomStore) rotateQR(code string) (room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found, ok := s.byCode[code]
	if !ok {
		return room{}, errRoomNotFound
	}
	found.qrGeneration++
	s.byCode[code] = found
	return found.snapshot(), nil
}
