package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func setup(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", strings.Repeat("k", 48))
}

func TestAccessRoundTrip(t *testing.T) {
	setup(t)
	sid, _ := NewSessionID()
	tok, err := NewAccessToken("u1", "n", "e@x.com", sid)
	if err != nil {
		t.Fatal(err)
	}
	c, err := ParseAccess(tok)
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "u1" || c.SessionID != sid {
		t.Fatalf("claims inesperados: %+v", c)
	}
}

func TestAccessRejections(t *testing.T) {
	setup(t)
	key := []byte(strings.Repeat("k", 48))
	sign := func(m jwt.SigningMethod, k any, claims Claims) string {
		s, err := jwt.NewWithClaims(m, claims).SignedString(k)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	base := Claims{ID: "u1", Type: TypeAccess, SessionID: strings.Repeat("a", 32), RegisteredClaims: jwt.RegisteredClaims{
		Issuer: issuer, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	if _, err := ParseAccess(sign(jwt.SigningMethodHS256, key, base)); err != nil {
		t.Fatalf("token base deveria ser válido: %v", err)
	}

	cases := map[string]string{
		"segredo errado": sign(jwt.SigningMethodHS256, []byte("outro"), base),
		"HS512":          sign(jwt.SigningMethodHS512, key, base),
		"alg none":       sign(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, base),
		"malformado":     "lixo",
	}
	mut := func(f func(*Claims)) string { c := base; f(&c); return sign(jwt.SigningMethodHS256, key, c) }
	cases["sem exp"] = mut(func(c *Claims) { c.ExpiresAt = nil })
	cases["expirado"] = mut(func(c *Claims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute)) })
	cases["sem sid"] = mut(func(c *Claims) { c.SessionID = "" })
	cases["tipo errado"] = mut(func(c *Claims) { c.Type = "refresh" })
	cases["emissor errado"] = mut(func(c *Claims) { c.Issuer = "outro" })

	for name, tok := range cases {
		if _, err := ParseAccess(tok); err == nil {
			t.Errorf("%s: aceito indevidamente", name)
		}
	}
}

func TestRefreshToken(t *testing.T) {
	sid, _ := NewSessionID()
	tok, hash, err := NewRefreshToken(sid)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hash, strings.SplitN(tok, ".", 2)[1]) {
		t.Fatal("hash contém o segredo")
	}
	gotSid, gotHash, err := ParseRefreshToken(tok)
	if err != nil || gotSid != sid || gotHash != hash {
		t.Fatalf("parse: sid=%q hash ok=%v err=%v", gotSid, gotHash == hash, err)
	}
	_, other, _ := NewRefreshToken(sid)
	if other == hash {
		t.Fatal("dois refresh tokens com o mesmo segredo")
	}
	for _, bad := range []string{"", "abc", sid, sid + ".", "xyz." + strings.Repeat("a", 43), sid + ".curto", "eyJhbGciOi.eyJ.sig"} {
		if _, _, err := ParseRefreshToken(bad); err == nil {
			t.Errorf("aceitou refresh inválido %q", bad)
		}
	}
}

func TestInitRequiresStrongSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	if Init() == nil {
		t.Fatal("aceitou segredo vazio")
	}
	t.Setenv("JWT_SECRET", "curto")
	if Init() == nil {
		t.Fatal("aceitou segredo curto")
	}
	setup(t)
	if err := Init(); err != nil {
		t.Fatal(err)
	}
}
