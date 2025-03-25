package jwtc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/golang-jwt/jwt/v4"
	"github.com/spf13/viper"
)

type ContextKey string

var (
	phoneKey = ContextKey("phone")
)

var (
	accessSecret  = viper.GetString("jwtc.accessSecret")
	refreshSecret = viper.GetString("jwtc.refreshSecret")
)

type AuthClaims struct {
	Phone string `json:"phone"`
	jwt.RegisteredClaims
}

func GenAccessToken(phone string) (string, error) {
	ac := AuthClaims{
		phone,
		jwt.RegisteredClaims{
			ID:        time.Now().String(),
			Issuer:    "Fl0rencess720",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, ac).SignedString(accessSecret)
	if err != nil {
		return "", err
	}
	return accessToken, nil
}

func GenRefreshToken() (string, error) {
	rc := jwt.RegisteredClaims{
		ID:        time.Now().String(),
		Issuer:    "Fl0rencess720",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, rc).SignedString(refreshSecret)
	if err != nil {
		return "", err
	}
	return refreshToken, nil
}

func GenToken(phone string) (string, string, error) {
	accessToken, err := GenAccessToken(phone)
	if err != nil {
		return "", "", nil
	}
	refreshToken, err := GenRefreshToken()
	if err != nil {
		return "", "", nil
	}
	return accessToken, refreshToken, nil
}

func ParseToken(aToken string) (*AuthClaims, bool, error) {
	accessToken, err := jwt.ParseWithClaims(aToken, &AuthClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(accessSecret), nil
	})
	if err != nil {
		return nil, false, errors.New("invalid token")
	}
	if claims, ok := accessToken.Claims.(*AuthClaims); ok && accessToken.Valid {
		return claims, false, nil
	}
	return nil, true, errors.New("invalid token")
}

func RefreshToken(aToken, rToken string) (string, error) {
	_, err := jwt.Parse(rToken, func(token *jwt.Token) (interface{}, error) {
		return []byte(refreshSecret), nil
	})
	if err != nil {
		return "", err
	}
	var claims AuthClaims
	_, err = jwt.ParseWithClaims(aToken, &claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(accessSecret), nil
	})
	v, _ := err.(*jwt.ValidationError)
	if v.Errors == jwt.ValidationErrorExpired {
		return GenAccessToken(claims.Phone)
	}
	return "", err
}

func Auth() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				tokenString := tr.RequestHeader().Get("Authorization")
				if tokenString == "" {
					return nil, errors.New("miss token string")
				}
				parts := strings.Split(tokenString, " ")
				if len(parts) != 2 || parts[0] != "Bearer" {
					return nil, errors.New("wrong token format")
				}
				parsedToken, isExpire, err := ParseToken(parts[1])
				if err != nil {
					return nil, errors.New("invalid token")
				}
				if isExpire {
					return nil, errors.New("token expired")
				}
				ctx = context.WithValue(ctx, phoneKey, parsedToken.Phone)
			}
			return handler(ctx, req)
		}
	}
}
