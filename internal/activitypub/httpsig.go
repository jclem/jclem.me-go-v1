package activitypub

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-fed/httpsig"
	"github.com/jclem/jclem.me/internal/activitypub/identity"
	"github.com/jclem/jclem.me/internal/database"
)

func newSignedActivityRequest(
	ctx context.Context,
	id *identity.Service,
	userRecordID database.ULID,
	method string,
	url string,
	body []byte,
) (*http.Request, error) {
	r, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	r.Header.Set("Content-Type", ContentType)
	r.Header.Set("Accept", ContentType)
	r.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))

	user, err := id.GetUserByID(ctx, userRecordID)
	if err != nil {
		return nil, fmt.Errorf("error getting user: %w", err)
	}

	privateKeyPEM, err := id.GetPrivateKey(ctx, userRecordID)
	if err != nil {
		return nil, fmt.Errorf("error getting private key: %w", err)
	}

	if err := signJSONLDRequest(user, privateKeyPEM, r, body); err != nil {
		return nil, fmt.Errorf("error signing request: %w", err)
	}

	return r, nil
}

func signJSONLDRequest(user identity.User, privateKeyPEM identity.SigningKey, r *http.Request, b []byte) error {
	prefs := []httpsig.Algorithm{httpsig.RSA_SHA256}
	digestAlgo := httpsig.DigestSha256
	headers := []string{httpsig.RequestTarget, "date", "digest"}

	signer, _, err := httpsig.NewSigner(prefs, digestAlgo, headers, httpsig.Signature, 0)
	if err != nil {
		return fmt.Errorf("error creating signer: %w", err)
	}

	block, _ := pem.Decode([]byte(privateKeyPEM.PEM))
	if block == nil {
		return errors.New("error decoding private key")
	}

	pkey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("error parsing private key: %w", err)
	}

	rsaKey, ok := pkey.(*rsa.PrivateKey)
	if !ok {
		return errors.New("private key is not an RSA key")
	}

	if err := signer.SignRequest(rsaKey, ActorPublicKeyID(user), r, b); err != nil {
		return fmt.Errorf("error signing request: %w", err)
	}

	return nil
}
