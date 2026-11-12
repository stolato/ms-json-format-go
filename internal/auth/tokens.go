// Package auth emite e valida as credenciais da aplicação.
//
//   - Access token: JWT curto enviado em Authorization (e no websocket). Carrega
//     o id da sessão (sid); o middleware confere no banco que a sessão ainda
//     existe, então logout/revogação valem imediatamente.
//   - Refresh token: opaco, no formato "<sid>.<segredo>". O banco guarda apenas
//     o SHA-256 do segredo; um vazamento do banco não permite renovar sessões.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	AccessTTL  = 24 * time.Hour
	RefreshTTL = 7 * 24 * time.Hour // renovado a cada uso do refresh token

	TypeAccess = "access"

	issuer          = "ms-json-format"
	minSecretLength = 32
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty"`
	Type      string `json:"typ"`
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

// Init garante que JWT_SECRET existe e é longo o bastante. Com segredo vazio
// qualquer pessoa conseguiria assinar tokens válidos.
func Init() error {
	if len(os.Getenv("JWT_SECRET")) < minSecretLength {
		return fmt.Errorf("JWT_SECRET deve ter pelo menos %d caracteres (gere com: openssl rand -base64 48)", minSecretLength)
	}
	return nil
}

func secret() []byte {
	return []byte(os.Getenv("JWT_SECRET"))
}

// NewAccessToken emite um access token vinculado à sessão sid.
func NewAccessToken(userID, name, email, sid string) (string, error) {
	now := time.Now()
	claims := Claims{
		ID:        userID,
		Name:      name,
		Email:     email,
		Type:      TypeAccess,
		SessionID: sid,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret())
}

// ParseAccess valida assinatura, expiração, emissor, tipo e presença do sid.
// Não consulta o banco: quem chama deve conferir se a sessão está ativa.
func ParseAccess(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(*jwt.Token) (interface{}, error) { return secret(), nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(issuer),
	)
	if err != nil || token == nil || !token.Valid ||
		claims.Type != TypeAccess || claims.ID == "" || claims.SessionID == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// NewSessionID gera um id de sessão aleatório.
func NewSessionID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// NewRefreshToken gera um refresh token para a sessão sid. Retorna o token
// (entregue ao cliente) e o hash do segredo (o único valor salvo no banco).
func NewRefreshToken(sid string) (token, secretHash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	sec := base64.RawURLEncoding.EncodeToString(buf)
	return sid + "." + sec, HashSecret(sec), nil
}

// ParseRefreshToken separa o sid e devolve o hash do segredo para consulta.
func ParseRefreshToken(token string) (sid, secretHash string, err error) {
	sid, sec, ok := strings.Cut(token, ".")
	if !ok || len(sid) != 32 || sec == "" {
		return "", "", ErrInvalidToken
	}
	if _, err := hex.DecodeString(sid); err != nil {
		return "", "", ErrInvalidToken
	}
	if raw, err := base64.RawURLEncoding.DecodeString(sec); err != nil || len(raw) != 32 {
		return "", "", ErrInvalidToken
	}
	return sid, HashSecret(sec), nil
}

// NewOpaqueToken gera um token aleatório (ex.: verificação de email) e o hash
// que deve ser salvo no banco.
func NewOpaqueToken() (token, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(buf)
	return token, HashSecret(token), nil
}

func HashSecret(sec string) string {
	sum := sha256.Sum256([]byte(sec))
	return hex.EncodeToString(sum[:])
}
