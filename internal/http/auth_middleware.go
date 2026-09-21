package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

type principalContextKey struct{}

func withPrincipal(
	ctx context.Context,
	principal ports.AuthenticatedPrincipal,
) context.Context {
	return context.WithValue(
		ctx,
		principalContextKey{},
		principal,
	)
}

func PrincipalFromContext(
	ctx context.Context,
) (ports.AuthenticatedPrincipal, bool) {
	principal, ok := ctx.Value(
		principalContextKey{},
	).(ports.AuthenticatedPrincipal)

	return principal, ok
}

func AuthMiddleware(
	validator ports.TokenValidator,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if validator == nil {
				http.Error(
					w,
					"authentication is not configured",
					http.StatusInternalServerError,
				)
				return
			}

			header := strings.TrimSpace(
				r.Header.Get("Authorization"),
			)

			if header == "" {
				http.Error(
					w,
					"missing Authorization header",
					http.StatusUnauthorized,
				)
				return
			}

			const prefix = "Bearer "

			if !strings.HasPrefix(header, prefix) {
				http.Error(
					w,
					"invalid Authorization header",
					http.StatusUnauthorized,
				)
				return
			}

			token := strings.TrimSpace(
				strings.TrimPrefix(header, prefix),
			)

			if token == "" {
				http.Error(
					w,
					"bearer token is required",
					http.StatusUnauthorized,
				)
				return
			}

			principal, err := validator.Validate(
				r.Context(),
				token,
			)
			if err != nil {
				http.Error(
					w,
					"invalid access token",
					http.StatusUnauthorized,
				)
				return
			}

			ctx := withPrincipal(
				r.Context(),
				principal,
			)

			next.ServeHTTP(
				w,
				r.WithContext(ctx),
			)
		})
	}
}
