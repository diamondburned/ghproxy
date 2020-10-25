package proxy

import "net/http"

type CookieInterceptFunc = func(setCookie string) string

// CookieInterceptor intercepts header writes to modify cookies.
type CookieInterceptor struct {
	changer CookieInterceptFunc
}

func NewCookieInterceptor(changer CookieInterceptFunc) CookieInterceptor {
	return CookieInterceptor{
		changer: changer,
	}
}

func (ci CookieInterceptor) NewWriter(w http.ResponseWriter) http.ResponseWriter {
	return cookieInterceptorWriter{
		ResponseWriter: w,
		interceptor:    ci,
	}
}

type cookieInterceptorWriter struct {
	http.ResponseWriter
	interceptor CookieInterceptor
}

func (w cookieInterceptorWriter) WriteHeader(statusCode int) {
	// Copy all headers.
	var actualHeader = w.ResponseWriter.Header()

	if cookies, ok := actualHeader["Set-Cookie"]; ok {
		// Intercept all Set-Cookie headers.
		for i, value := range cookies {
			cookies[i] = w.interceptor.changer(value)
		}
		actualHeader["Set-Cookie"] = cookies
	}

	// Finalize.
	w.ResponseWriter.WriteHeader(statusCode)
}
