// Package encryption criptografa o conteúdo dos itens antes de salvar no banco
// (AES-256-GCM). Protege os JSONs caso o banco ou um backup vaze; não protege
// contra quem tiver acesso à chave (ex.: servidor comprometido).
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// Prefix identifica valores criptografados e a versão da chave/formato.
// Para rotacionar a chave no futuro, crie "v2:" e mantenha a v1 só para leitura.
const Prefix = "v1:"

const envKey = "JSON_ENCRYPTION_KEY"

var aead cipher.AEAD

// Init carrega a chave (JSON_ENCRYPTION_KEY: 32 bytes em base64).
// Gere com: openssl rand -base64 32
func Init(encoded string) error {
	if encoded == "" {
		return fmt.Errorf("%s não definida", envKey)
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("%s não é base64 válido: %w", envKey, err)
	}
	if len(key) != 32 {
		return fmt.Errorf("%s deve ter 32 bytes (tem %d)", envKey, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	aead = gcm
	return nil
}

// IsEncrypted indica se o valor já está no formato criptografado.
func IsEncrypted(stored string) bool {
	return strings.HasPrefix(stored, Prefix)
}

// Encrypt criptografa plain amarrando o resultado ao id do documento
// (additional data): um ciphertext copiado para outro documento não decifra.
func Encrypt(plain, id string) (string, error) {
	if aead == nil {
		return "", errors.New("encryption não inicializado")
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(plain), []byte(id))
	return Prefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverte Encrypt. Valores sem o prefixo são tratados como texto puro
// legado (anteriores à migração) e retornados sem alteração.
func Decrypt(stored, id string) (string, error) {
	if !IsEncrypted(stored) {
		return stored, nil
	}
	if aead == nil {
		return "", errors.New("encryption não inicializado")
	}
	raw, err := base64.StdEncoding.DecodeString(stored[len(Prefix):])
	if err != nil {
		return "", fmt.Errorf("ciphertext inválido: %w", err)
	}
	n := aead.NonceSize()
	if len(raw) < n+aead.Overhead() {
		return "", errors.New("ciphertext inválido: muito curto")
	}
	plain, err := aead.Open(nil, raw[:n], raw[n:], []byte(id))
	if err != nil {
		return "", fmt.Errorf("falha ao decifrar: %w", err)
	}
	return string(plain), nil
}
