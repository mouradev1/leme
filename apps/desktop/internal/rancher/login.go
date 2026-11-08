package rancher

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"leme/internal/platform/i18n"
	"math/big"
	"net/http"
	"net/url"
	"time"
)

// Browser login, the same flow as Rancher's own CLI:
//
//  1. Leme creates a throwaway RSA key pair and a random request id.
//  2. The browser opens Rancher's login page with the id and the public key.
//     The user logs in there with whatever the server uses (Keycloak...).
//  3. Rancher stores a new token for that request id, encrypted with the
//     public key, so only this Leme can read it.
//  4. Leme polls for it, decrypts it and deletes the request on the server.
//
// Leme never sees the user's password, and the token never passes through
// the browser in clear text.

type Token struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"` // RFC 3339, empty = no expiry
}

type Login struct {
	URL  string // open in the browser
	c    *Client
	id   string
	priv *rsa.PrivateKey
}

const idChars = "bcdfghjklmnpqrstvwxz2456789" // like Rancher's CLI: valid in object names

func randomID(n int) (string, error) {
	b := make([]byte, n)
	max := big.NewInt(int64(len(idChars)))
	for i := range b {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = idChars[v.Int64()]
	}
	return string(b), nil
}

func (c *Client) StartLogin() (*Login, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	pub, err := json.Marshal(priv.PublicKey) // {"N":...,"E":65537}, what Rancher expects
	if err != nil {
		return nil, err
	}
	id, err := randomID(32)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("requestId", id)
	q.Set("publicKey", base64.StdEncoding.EncodeToString(pub))
	q.Set("responseType", "kubeconfig")
	return &Login{URL: c.Base + "/dashboard/auth/login?" + q.Encode(), c: c, id: id, priv: priv}, nil
}

// Wait polls until the user finishes logging in, ctx ends, or it fails.
func (l *Login) Wait(ctx context.Context, every time.Duration) (Token, error) {
	path := "/v3-public/authTokens/" + l.id
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		var got Token
		err := l.c.do(ctx, http.MethodGet, path, "", &got)
		switch {
		case err == nil && got.Token != "":
			tok, derr := l.decrypt(got.Token)
			dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			_ = l.c.do(dctx, http.MethodDelete, path, "", nil) // one-time: remove it from the server
			cancel()
			if derr != nil {
				return Token{}, derr
			}
			return Token{Token: tok, ExpiresAt: got.ExpiresAt}, nil
		case err != nil && !errors.Is(err, errNotFound) && ctx.Err() == nil:
			// Transient (network blip, 5xx): keep waiting, the user may still be typing.
		}
		select {
		case <-ctx.Done():
			return Token{}, ctx.Err()
		case <-t.C:
		}
	}
}

func (l *Login) decrypt(b64 string) (string, error) {
	enc, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf(i18n.T("rancher: resposta de login inválida: %w", "rancher: invalid login response: %w"), err)
	}
	plain, err := rsa.DecryptOAEP(sha256.New(), nil, l.priv, enc, nil)
	if err != nil {
		return "", fmt.Errorf(i18n.T("rancher: não foi possível ler o token recebido: %w", "rancher: could not read the received token: %w"), err)
	}
	return string(plain), nil
}
