package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

type Claims struct {
	ChannelID string `json:"channel_id"`
	jwt.RegisteredClaims
}

type JWTService struct {
	secret     []byte
	expiresIn  int
	signingAlg jwt.SigningMethod
}

// NewJWTService creates a new JWT service
func NewJWTService(secret string, expiresIn int, algo string) (*JWTService, error) {
	var signingAlg jwt.SigningMethod
	switch algo {
	case "HS256":
		signingAlg = jwt.SigningMethodHS256
	case "HS384":
		signingAlg = jwt.SigningMethodHS384
	case "HS512":
		signingAlg = jwt.SigningMethodHS512
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", algo)
	}

	return &JWTService{
		secret:     []byte(secret),
		expiresIn:  expiresIn,
		signingAlg: signingAlg,
	}, nil
}

// GenerateToken generates a new JWT token for a channel
func (s *JWTService) GenerateToken(channelID string) (string, error) {
	now := time.Now()
	claims := Claims{
		ChannelID: channelID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(s.expiresIn) * time.Second)),
		},
	}

	token := jwt.NewWithClaims(s.signingAlg, claims)
	return token.SignedString(s.secret)
}

// ValidateToken validates a JWT token and returns the channel ID
func (s *JWTService) ValidateToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method
		if token.Method != s.signingAlg {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", ErrExpiredToken
		}
		return "", fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return "", ErrInvalidToken
	}

	return claims.ChannelID, nil
}
