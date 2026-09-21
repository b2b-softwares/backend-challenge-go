package ports

import "context"

type AuthenticatedPrincipal struct {
	Subject  string
	ClientID string
	Scopes   []string
}

type TokenValidator interface {
	Validate(ctx context.Context, token string) (AuthenticatedPrincipal, error)
}
