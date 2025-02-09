package endpoint

import (
	"net/http"

	"github.com/MunifTanjim/stremthru/internal/config"
	"github.com/MunifTanjim/stremthru/internal/context"
)

type MiddlewareFunc func(http.HandlerFunc) http.HandlerFunc

func withRequestContext(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, context.SetRequestContext(r))
	})
}

func Middleware(middlewares ...MiddlewareFunc) MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return withRequestContext(next)
	}
}

func ProxyAuthContext(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.GetRequestContext(r)

		token, hasToken := extractProxyAuthToken(r)
		_, username, password, ok := parseBasicAuthToken(token)

		ctx.IsProxyAuthorized = hasToken && ok && config.ProxyAuthPassword.GetPassword(username) == password
		ctx.ProxyAuthUser = username
		ctx.ProxyAuthPassword = password

		next.ServeHTTP(w, r)
	})
}

func ProxyAuthRequired(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.GetRequestContext(r)

		if !ctx.IsProxyAuthorized {
			w.Header().Add("Proxy-Authenticate", "Basic")
			SendError(w, ErrorProxyAuthRequired(r))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func StoreContext(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		store, err := getStore(r)
		if err != nil {
			SendError(w, err)
			return
		}
		ctx := context.GetRequestContext(r)
		ctx.Store = store
		ctx.StoreAuthToken = getStoreAuthToken(r)
		w.Header().Add("X-StremThru-Store-Name", r.Header.Get("X-StremThru-Store-Name"))
		next.ServeHTTP(w, r)
	})
}

func StoreRequired(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.GetRequestContext(r)

		if ctx.Store == nil {
			SendError(w, ErrorBadRequest(r, "missing store"))
			return
		}

		if ctx.StoreAuthToken == "" {
			w.Header().Add("WWW-Authenticate", "Bearer realm=\"store:"+string(ctx.Store.GetName())+"\"")
			SendError(w, ErrorUnauthorized(r))
			return
		}

		next.ServeHTTP(w, r)
	})
}
