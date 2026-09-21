package oidc

import (
	"context"
	"fmt"
	"strings"

	coreoidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

type Verifier struct {
	verifier *coreoidc.IDTokenVerifier
	audience string
}

func NewVerifier(
	ctx context.Context,
	issuerURL string,
	clientID string,
	audience string,
) (*Verifier, error) {
	issuerURL = strings.TrimSpace(issuerURL)
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

	provider, err := coreoidc.NewProvider(
		ctx,
		issuerURL,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"initialize OIDC provider: %w",
			err,
		)
	}

	verifier := provider.Verifier(
		&coreoidc.Config{
			SkipClientIDCheck: true,
		},
	)

	return &Verifier{
		verifier: verifier,
		audience: audience,
	}, nil
}

func (v *Verifier) Validate(
	ctx context.Context,
	token string,
) (ports.AuthenticatedPrincipal, error) {
	if v == nil || v.verifier == nil {
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

	idToken, err := v.verifier.Verify(
		ctx,
		token,
	)
	if err != nil {
		return ports.AuthenticatedPrincipal{}, fmt.Errorf(
			"validate access token: %w",
			err,
		)
	}

	var claims struct {
		Subject  string   `json:"sub"`
		Audience []string `json:"aud"`
		ClientID string   `json:"client_id"`
		AZP      string   `json:"azp"`
		Scope    string   `json:"scope"`
	}

	if err := idToken.Claims(&claims); err != nil {
		return ports.AuthenticatedPrincipal{}, fmt.Errorf(
			"decode access token claims: %w",
			err,
		)
	}

	if claims.Subject == "" {
		return ports.AuthenticatedPrincipal{}, fmt.Errorf(
			"access token subject is required",
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
			claims.AZP,
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
