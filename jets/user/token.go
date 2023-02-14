package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
)

var TokenExpiration int
var ApiSecret string

func CreateToken(email string) (string, error) {
	claims := jwt.MapClaims{}
	claims["authorized"] = true
	claims["email"] = email
	// claims["exp"] = time.Now().Add(time.Hour * 1).Unix() //Token expires after 1 hour
	claims["exp"] = time.Now().Add(time.Minute * time.Duration(TokenExpiration)).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(ApiSecret))
}

// Check if token is valid, if valid returns the associated email
func TokenValid(r *http.Request) (string, error) {
	tokenString := ExtractToken(r)
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
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
	claims := token.Claims.(jwt.MapClaims)

	return claims["email"].(string), nil
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
	jwtToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
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

//Pretty display the claims nicely in the terminal
func Pretty(data interface{}) {
	b, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		log.Println(err)
		return
	}

	fmt.Println(string(b))
}