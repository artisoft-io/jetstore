package user

import (
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

// A signed, unexpired token whose claims carry no string email: ExtractTokenID
// must refuse it with an error, as TokenStringValid does, and not panic on the
// type assertion.
func TestExtractTokenIDWithoutEmailClaim(t *testing.T) {
	ApiSecret = "test-secret-not-a-real-one"
	for name, claims := range map[string]jwt.MapClaims{
		"no email":      {"exp": time.Now().Add(time.Minute).Unix()},
		"numeric email": {"email": 42, "exp": time.Now().Add(time.Minute).Unix()},
	} {
		t.Run(name, func(t *testing.T) {
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(ApiSecret))
			if err != nil {
				t.Fatal(err)
			}
			email, err := ExtractTokenID(token)
			if err == nil {
				t.Errorf("ExtractTokenID = %q, nil; want an error", email)
			}
			if _, err := TokenStringValid(token); err == nil {
				t.Errorf("TokenStringValid accepted a token with no email claim")
			}
		})
	}
}

func TestExtractTokenIDReturnsTheEmail(t *testing.T) {
	ApiSecret = "test-secret-not-a-real-one"
	TokenExpiration = 5
	token, err := CreateToken("someone@example.com")
	if err != nil {
		t.Fatal(err)
	}
	email, err := ExtractTokenID(token)
	if err != nil || email != "someone@example.com" {
		t.Errorf("ExtractTokenID = %q, %v; want someone@example.com, nil", email, err)
	}
}
