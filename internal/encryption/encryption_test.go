package encryption

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func setup(t *testing.T) {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	if err := Init(base64.StdEncoding.EncodeToString(key)); err != nil {
		t.Fatal(err)
	}
}

func TestRoundTrip(t *testing.T) {
	setup(t)
	plain := `{"hello":"mundo","n":[1,2,3]}`
	enc, err := Encrypt(plain, "id1")
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncrypted(enc) || enc == plain {
		t.Fatalf("valor não criptografado: %q", enc)
	}
	got, err := Decrypt(enc, "id1")
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("got %q, want %q", got, plain)
	}
}

func TestNonceIsRandom(t *testing.T) {
	setup(t)
	a, _ := Encrypt("x", "id1")
	b, _ := Encrypt("x", "id1")
	if a == b {
		t.Fatal("dois Encrypt iguais geraram o mesmo ciphertext")
	}
}

func TestWrongIdFails(t *testing.T) {
	setup(t)
	enc, _ := Encrypt("segredo", "id1")
	if _, err := Decrypt(enc, "id2"); err == nil {
		t.Fatal("esperava erro ao decifrar com id diferente")
	}
}

func TestTamperedFails(t *testing.T) {
	setup(t)
	enc, _ := Encrypt("segredo", "id1")
	raw, _ := base64.StdEncoding.DecodeString(enc[len(Prefix):])
	raw[len(raw)-1] ^= 0xff
	tampered := Prefix + base64.StdEncoding.EncodeToString(raw)
	if _, err := Decrypt(tampered, "id1"); err == nil {
		t.Fatal("esperava erro com ciphertext adulterado")
	}
}

func TestLegacyPlaintextPassthrough(t *testing.T) {
	setup(t)
	got, err := Decrypt(`{"a":1}`, "id1")
	if err != nil || got != `{"a":1}` {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestInitRejectsBadKey(t *testing.T) {
	if err := Init(base64.StdEncoding.EncodeToString([]byte("curta"))); err == nil {
		t.Fatal("esperava erro com chave curta")
	}
	if err := Init(""); err == nil {
		t.Fatal("esperava erro com chave vazia")
	}
}
