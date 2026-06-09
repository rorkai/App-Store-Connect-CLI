package asc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateJWT_TeamKeyUsesIssuerClaim(t *testing.T) {
	privateKey := testJWTPrivateKey(t)

	tokenString, err := GenerateJWT("KEY123", "ISS456", privateKey)
	if err != nil {
		t.Fatalf("GenerateJWT() error: %v", err)
	}

	claims := parseJWTClaims(t, tokenString, privateKey)
	if claims.Issuer != "ISS456" {
		t.Fatalf("issuer claim = %q, want ISS456", claims.Issuer)
	}
	if claims.Subject != "" {
		t.Fatalf("subject claim = %q, want empty", claims.Subject)
	}
}

func TestGenerateJWT_IndividualKeyUsesUserSubjectClaim(t *testing.T) {
	privateKey := testJWTPrivateKey(t)

	tokenString, err := GenerateJWT("KEY123", "", privateKey)
	if err != nil {
		t.Fatalf("GenerateJWT() error: %v", err)
	}

	claims := parseJWTClaims(t, tokenString, privateKey)
	if claims.Issuer != "" {
		t.Fatalf("issuer claim = %q, want empty", claims.Issuer)
	}
	if claims.Subject != "user" {
		t.Fatalf("subject claim = %q, want user", claims.Subject)
	}
	if !jwtAudienceContains(claims.Audience, "appstoreconnect-v1") {
		t.Fatalf("audience claim = %v, want appstoreconnect-v1", claims.Audience)
	}
}

func testJWTPrivateKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey() error: %v", err)
	}
	return key
}

func parseJWTClaims(t *testing.T, tokenString string, privateKey *ecdsa.PrivateKey) jwt.RegisteredClaims {
	t.Helper()

	claims := jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	}, jwt.WithAudience("appstoreconnect-v1"))
	if err != nil {
		t.Fatalf("ParseWithClaims() error: %v", err)
	}
	if !token.Valid {
		t.Fatal("expected token to be valid")
	}
	return claims
}

func jwtAudienceContains(audience jwt.ClaimStrings, value string) bool {
	for _, item := range audience {
		if item == value {
			return true
		}
	}
	return false
}
