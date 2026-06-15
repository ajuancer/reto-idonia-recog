package idonia

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func (c *Client) generateJWT() (string, error) {
	decodedSecret, err := decodeAPISecret(c.config.IdoniaAPISecret)
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"sub": c.config.IdoniaAPIKey,
		"iat": now.Add(-5 * time.Minute).Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["typ"] = "JWT"
	token.Header["alg"] = "HS256"

	return token.SignedString(decodedSecret)
}

func decodeAPISecret(secret string) ([]byte, error) {
	if !strings.HasPrefix(secret, "S2") {
		return nil, errors.New("invalid Idonia API Secret format: missing S2 prefix")
	}
	trimmedSecret := strings.TrimPrefix(secret, "S2")

	trimmedSecret = strings.ReplaceAll(trimmedSecret, "-", "+")
	trimmedSecret = strings.ReplaceAll(trimmedSecret, "_", "/")

	if m := len(trimmedSecret) % 4; m != 0 {
		trimmedSecret += strings.Repeat("=", 4-m)
	}

	decodedSecret, err := base64.StdEncoding.DecodeString(trimmedSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to decode secret: %w", err)
	}

	return decodedSecret, nil
}
