package token

import (
	"auth-service/internal/models"
	"auth-service/internal/service"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWKStore interface {
	GetActive(ctx context.Context) (*models.JwkKey, error)
	GetByKID(ctx context.Context, kid string) (*models.JwkKey, error)
	Create(ctx context.Context, rec *models.JwkKey) error
	SetActive(ctx context.Context, kid string) error
	ListPublic(ctx context.Context) ([]service.PublicJWK, error)
}

func (p *RSAProvider) EnsureKeyOnStart(ctx context.Context) error {
	return p.ensureActiveKey(ctx)
}

type RSAProvider struct {
	store    JWKStore
	issuer   string
	audience string

	mu        sync.RWMutex
	activeKid string
	privKey   *rsa.PrivateKey

	// кэш публичных ключей по kid
	pubMu   sync.RWMutex
	pubKeys map[string]*rsa.PublicKey

	now func() time.Time
}

func NewRSAProvider(store JWKStore, issuer, audience string) *RSAProvider {
	return &RSAProvider{
		store: store, issuer: issuer, audience: audience,
		pubKeys: make(map[string]*rsa.PublicKey),
		now:     time.Now,
	}
}

func (p *RSAProvider) ensureActiveKey(ctx context.Context) error {
	p.mu.RLock()
	if p.privKey != nil && p.activeKid != "" {
		p.mu.RUnlock()
		return nil
	}

	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.privKey != nil && p.activeKid != "" {
		return nil
	}

	rec, err := p.store.GetActive(ctx)
	if err == nil && rec != nil {
		pk, kid, err := parsePriv(rec.PrivPEM, rec.KID)
		if err != nil {
			return err
		}
		p.privKey = pk
		p.activeKid = kid
		return nil
	}

	pk, kid, rec, err := generateAndRecord()
	if err != nil {
		return err
	}

	if err := p.store.Create(ctx, rec); err != nil {
		return err
	}
	if err := p.store.SetActive(ctx, kid); err != nil {
		return err
	}

	p.privKey = pk
	p.activeKid = kid

	return nil
}

func parsePriv(privPEM []byte, kid string) (*rsa.PrivateKey, string, error) {
	block, _ := pem.Decode(privPEM)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, "", errors.New("invalid PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, "", err
	}
	return key, kid, nil
}

func generateAndRecord() (*rsa.PrivateKey, string, *models.JwkKey, error) {
	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, "", nil, err
	}
	kid := randomKid()
	// публичные параметры
	n := base64.RawURLEncoding.EncodeToString(pk.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(bigIntToBytes(pk.PublicKey.E))

	// приват в PEM
	b := x509.MarshalPKCS1PrivateKey(pk)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: b})

	rec := &models.JwkKey{
		KID: kid, Alg: "RS256", Kty: "RSA", Use: "sig",
		N: n, E: e, PrivPEM: pemBytes, Active: true,
	}
	return pk, kid, rec, nil
}

func bigIntToBytes(e int) []byte {
	// exponent обычно 65537, кодируем как big-endian без знака
	// компактная форма:
	if e == 0 {
		return []byte{0}
	}
	var b []byte
	for i := e; i > 0; i >>= 8 {
		b = append([]byte{byte(i & 0xff)}, b...)
	}
	return b
}

func randomKid() string {
	// kid удобнее как base64url(16 байт)
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}

// ===== Реализация TokenProvider =====

type customClaims struct {
	Sub  string `json:"sub"`
	Role string `json:"role"`
	Ver  int    `json:"ver,omitempty"`
	jwt.RegisteredClaims
}

// ListPublicJWK возвращает публичные ключи (JWKS) через store.
// Делегирование в репозиторий позволяет переиспользовать готовую выборку N/E/kid.
func (p *RSAProvider) ListPublicJWK(ctx context.Context) ([]service.PublicJWK, error) {
	return p.store.ListPublic(ctx)
}

func (p *RSAProvider) SignAccess(ctx context.Context, sub uuid.UUID, role string, ttl time.Duration) (string, time.Time, error) {
	if err := p.ensureActiveKey(ctx); err != nil {
		return "", time.Time{}, err
	}
	now := p.now()
	exp := now.Add(ttl)

	claims := customClaims{
		Sub:  sub.String(),
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    p.issuer,
			Subject:   sub.String(),
			Audience:  []string{p.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	t.Header["kid"] = p.activeKid

	signed, err := t.SignedString(p.privKey)
	return signed, exp, err
}

// opaque refresh без изменений
func (p *RSAProvider) NewRefresh(ctx context.Context, sub uuid.UUID, ttl time.Duration) (opaque, hash string, exp time.Time, err error) {
	exp = p.now().Add(ttl)
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", time.Time{}, err
	}
	opaque = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(opaque))
	hash = base64.RawURLEncoding.EncodeToString(sum[:])
	return
}

func (p *RSAProvider) ParseAndValidateAccess(ctx context.Context, token string) (*service.Claims, error) {
	keyfunc := func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodRS256 {
			return nil, errors.New("unexpected signing method")
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("missing kid")
		}

		// попробуем из кэша
		p.pubMu.RLock()
		if k, ok := p.pubKeys[kid]; ok {
			p.pubMu.RUnlock()
			return k, nil
		}
		p.pubMu.RUnlock()

		// загрузим из store
		rec, err := p.store.GetByKID(ctx, kid)
		if err != nil || rec == nil {
			return nil, errors.New("unknown kid")
		}
		pub, err := jwkToPublic(rec.N, rec.E)
		if err != nil {
			return nil, err
		}

		p.pubMu.Lock()
		p.pubKeys[kid] = pub
		p.pubMu.Unlock()

		return pub, nil
	}

	parsed, err := jwt.ParseWithClaims(token, &customClaims{}, keyfunc,
		jwt.WithIssuer(p.issuer), jwt.WithAudience(p.audience))
	if err != nil {
		return nil, err
	}
	cc, ok := parsed.Claims.(*customClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}

	uid, err := uuid.Parse(cc.Sub)
	if err != nil {
		return nil, err
	}
	return &service.Claims{UserID: uid, Role: cc.Role, Exp: cc.ExpiresAt.Time}, nil
}

func jwkToPublic(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, err
	}
	// big-endian int
	e := 0
	for _, b := range eBytes {
		e = (e << 8) | int(b)
	}
	pub := &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: e,
	}
	return pub, nil
}
