package unit

import (
	"testing"
	"time"

	"github.com/henokakunemail-stack/Endpoint-Manager/server/core/auth"
)

func TestPasswordHashAndCompare(t *testing.T) {
	hash, err := auth.HashPassword("s3cret!")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "s3cret!" {
		t.Fatal("hash must not equal plaintext")
	}
	if err := auth.ComparePassword(hash, "s3cret!"); err != nil {
		t.Fatalf("ComparePassword(correct): %v", err)
	}
	if err := auth.ComparePassword(hash, "wrong"); err == nil {
		t.Fatal("ComparePassword(wrong) must fail")
	}
}

func TestJWTIssueAndParse(t *testing.T) {
	svc := auth.NewJWTService("test-secret-32bytes-minimum-length-ok", time.Minute, time.Hour)

	pair, err := svc.Issue("user-1", "alice", "admin")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("tokens must not be empty")
	}
	if pair.AccessToken == pair.RefreshToken {
		t.Fatal("access and refresh must differ")
	}

	claims, err := svc.Parse(pair.AccessToken)
	if err != nil {
		t.Fatalf("Parse access: %v", err)
	}
	if claims.UserID != "user-1" || claims.Username != "alice" || claims.Role != "admin" {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	if _, err := svc.Parse(pair.RefreshToken); err != nil {
		t.Fatalf("Parse refresh: %v", err)
	}
}

func TestJWTRejectsWrongSecretAndExpired(t *testing.T) {
	good := auth.NewJWTService("secret-a", time.Minute, time.Hour)
	bad := auth.NewJWTService("secret-b", time.Minute, time.Hour)

	pair, err := good.Issue("u", "bob", "viewer")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := bad.Parse(pair.AccessToken); err == nil {
		t.Fatal("token signed with different secret must be rejected")
	}

	expired := auth.NewJWTService("secret-a", -time.Minute, time.Hour)
	pair2, err := expired.Issue("u", "bob", "viewer")
	if err != nil {
		t.Fatalf("Issue expired: %v", err)
	}
	if _, err := good.Parse(pair2.AccessToken); err == nil {
		t.Fatal("expired token must be rejected")
	}
}
