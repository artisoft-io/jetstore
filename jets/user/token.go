package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

var TokenExpiration int
var ApiSecret string

func CreateToken(email string) (string, error) {
	claims := jwt.MapClaims{}
	claims["authorized"] = true
	claims["email"] = email
	claims["exp"] = time.Now().Add(time.Minute * time.Duration(TokenExpiration)).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(ApiSecret))
}

// Check if token is valid, if valid returns the associated email
func TokenValid(r *http.Request) (string, error) {
	return TokenStringValid(ExtractToken(r))
}

// TokenStringValid checks a token string: HMAC signing method, signature with
// ApiSecret, expiry, and an email claim. Returns the email when valid.
// TokenValid applies it to a request's token; the documentation cookie
// applies it to the cookie's value.
func TokenStringValid(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(ApiSecret), nil
	})
	if err != nil {
		return "", err
	}
	// Token should be valid at this point, otherwise Parse would
	// have returned an error. To be safe though we still validate...
	if !token.Valid {
		return "", errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid token")
	}
	email, ok := claims["email"].(string)
	if !ok {
		return "", errors.New("invalid token: no email claim")
	}
	return email, nil
}

func TokenClaims(token *jwt.Token) (jwt.MapClaims, error) {
	if token == nil || !token.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if ok {
		Pretty(claims)
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

func ExtractToken(r *http.Request) string {
	keys := r.URL.Query()
	token := keys.Get("token")
	if token != "" {
		return token
	}
	bearerToken := r.Header.Get("Authorization")
	if len(strings.Split(bearerToken, " ")) == 2 {
		return strings.Split(bearerToken, " ")[1]
	}
	return ""
}

func ExtractTokenID(token string) (string, error) {
	jwtToken, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(ApiSecret), nil
	})
	if err != nil {
		return "", err
	}
	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if ok && jwtToken.Valid {
		email := claims["email"].(string)
		return email, nil
	}
	return "", nil
}

// Pretty display the claims nicely in the terminal
func Pretty(data any) {
	b, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		log.Println(err)
		return
	}

	log.Println(string(b))
}
