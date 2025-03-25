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
	PhoneKey = ContextKey("phone")
)

type AuthClaims struct {
	Phone string `json:"phone"`
	jwt.RegisteredClaims
}

func GenAccessToken(phone string) (string, error) {
	ac := AuthClaims{
		Phone: phone,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        time.Now().String(),
			Issuer:    "Fl0rencess720",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	accessSecret := viper.GetString("jwtc.accessSecret")
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, ac).SignedString([]byte(accessSecret))
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
	refreshSecret := viper.GetString("jwtc.refreshSecret")
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, rc).SignedString([]byte(refreshSecret))
	if err != nil {
		return "", err
	}
	return refreshToken, nil
}

func GenToken(phone string) (string, string, error) {
	accessToken, err := GenAccessToken(phone)
	if err != nil {
		return "", "", err
	}
	refreshToken, err := GenRefreshToken()
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

func ParseToken(aToken string) (*AuthClaims, bool, error) {
	accessSecret := viper.GetString("jwtc.accessSecret")
	accessToken, err := jwt.ParseWithClaims(aToken, &AuthClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(accessSecret), nil
	})
	if err != nil {
		return nil, false, err
	}
	if claims, ok := accessToken.Claims.(*AuthClaims); ok && accessToken.Valid {
		return claims, false, nil
	}
	return nil, true, errors.New("invalid token")
}

func RefreshToken(aToken, rToken string) (string, error) {
	accessSecret := viper.GetString("jwtc.accessSecret")
	refreshSecret := viper.GetString("jwtc.refreshSecret")
	rToken = strings.TrimPrefix(rToken, "Bearer ")
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
	if v == nil || v.Errors == jwt.ValidationErrorExpired {
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
				ctx = context.WithValue(ctx, PhoneKey, parsedToken.Phone)
			}
			return handler(ctx, req)
		}
	}
}
