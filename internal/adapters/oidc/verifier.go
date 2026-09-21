package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	coreoidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

const accessTokenClockSkew = 30 * time.Second

type Verifier struct {
	keySet   coreoidc.KeySet
	issuer   string
	audience string
}

type discoveryDocument struct {
	JWKSURI string `json:"jwks_uri"`
}

type accessTokenClaims struct {
	Issuer          string   `json:"iss"`
	Subject         string   `json:"sub"`
	Audience        []string `json:"aud"`
	ClientID        string   `json:"client_id"`
	AuthorizedParty string   `json:"azp"`
	Scope           string   `json:"scope"`
	ExpiresAt       int64    `json:"exp"`
	NotBefore       int64    `json:"nbf"`
}

func NewVerifier(
	ctx context.Context,
	issuerURL string,
	clientID string,
	audience string,
) (*Verifier, error) {
	issuerURL = strings.TrimRight(
		strings.TrimSpace(issuerURL),
		"/",
	)
	clientID = strings.TrimSpace(clientID)
	audience = strings.TrimSpace(audience)

	if issuerURL == "" {
		return nil, fmt.Errorf(
			"OIDC issuer URL is required",
		)
	}

	if clientID == "" {
		return nil, fmt.Errorf(
			"OIDC client ID is required",
		)
	}

	if audience == "" {
		return nil, fmt.Errorf(
			"OIDC audience is required",
		)
	}

	jwksURI, err := discoverJWKSURI(
		ctx,
		issuerURL,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"discover OIDC JWKS URI: %w",
			err,
		)
	}

	keySet := coreoidc.NewRemoteKeySet(
		ctx,
		jwksURI,
	)

	return &Verifier{
		keySet:   keySet,
		issuer:   issuerURL,
		audience: audience,
	}, nil
}

func (v *Verifier) Validate(
	ctx context.Context,
	token string,
) (ports.AuthenticatedPrincipal, error) {
	if v == nil || v.keySet == nil {
		return ports.AuthenticatedPrincipal{}, fmt.Errorf(
			"OIDC verifier is not initialized",
		)
	}

	token = strings.TrimSpace(token)

	if token == "" {
		return ports.AuthenticatedPrincipal{}, fmt.Errorf(
			"access token is required",
		)
	}

	payload, err := v.keySet.VerifySignature(
		ctx,
		token,
	)
	if err != nil {
		return ports.AuthenticatedPrincipal{}, fmt.Errorf(
			"validate access token signature: %w",
			err,
		)
	}

	var claims accessTokenClaims

	if err := json.Unmarshal(
		payload,
		&claims,
	); err != nil {
		return ports.AuthenticatedPrincipal{}, fmt.Errorf(
			"decode access token claims: %w",
			err,
		)
	}

	if claims.Issuer != v.issuer {
		return ports.AuthenticatedPrincipal{}, fmt.Errorf(
			"access token issuer mismatch",
		)
	}

	if claims.Subject == "" {
		return ports.AuthenticatedPrincipal{}, fmt.Errorf(
			"access token subject is required",
		)
	}

	if claims.ExpiresAt <= 0 {
		return ports.AuthenticatedPrincipal{}, fmt.Errorf(
			"access token expiration is required",
		)
	}

	now := time.Now()

	if now.After(
		time.Unix(
			claims.ExpiresAt,
			0,
		).Add(accessTokenClockSkew),
	) {
		return ports.AuthenticatedPrincipal{}, fmt.Errorf(
			"access token is expired",
		)
	}

	if claims.NotBefore > 0 &&
		now.Before(
			time.Unix(
				claims.NotBefore,
				0,
			).Add(-accessTokenClockSkew),
		) {
		return ports.AuthenticatedPrincipal{}, fmt.Errorf(
			"access token is not active yet",
		)
	}

	if !containsAudience(
		claims.Audience,
		v.audience,
	) {
		return ports.AuthenticatedPrincipal{}, fmt.Errorf(
			"access token audience does not contain %q",
			v.audience,
		)
	}

	clientID := strings.TrimSpace(
		claims.ClientID,
	)

	if clientID == "" {
		clientID = strings.TrimSpace(
			claims.AuthorizedParty,
		)
	}

	scopes := strings.Fields(
		claims.Scope,
	)

	return ports.AuthenticatedPrincipal{
		Subject:  claims.Subject,
		ClientID: clientID,
		Scopes:   scopes,
	}, nil
}

func discoverJWKSURI(
	ctx context.Context,
	issuerURL string,
) (string, error) {
	discoveryURL := strings.TrimRight(
		issuerURL,
		"/",
	) + "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		discoveryURL,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf(
			"create OIDC discovery request: %w",
			err,
		)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf(
			"request OIDC discovery document: %w",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(
			"OIDC discovery returned HTTP %d",
			resp.StatusCode,
		)
	}

	var document discoveryDocument

	if err := json.NewDecoder(
		resp.Body,
	).Decode(&document); err != nil {
		return "", fmt.Errorf(
			"decode OIDC discovery document: %w",
			err,
		)
	}

	document.JWKSURI = strings.TrimSpace(
		document.JWKSURI,
	)

	if document.JWKSURI == "" {
		return "", fmt.Errorf(
			"OIDC discovery document does not contain jwks_uri",
		)
	}

	return document.JWKSURI, nil
}

func containsAudience(
	audiences []string,
	expected string,
) bool {
	expected = strings.TrimSpace(expected)

	if expected == "" {
		return false
	}

	for _, audience := range audiences {
		if strings.TrimSpace(audience) == expected {
			return true
		}
	}

	return false
}
