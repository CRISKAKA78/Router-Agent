package api

import "net/http"

// ServeContent owns HTTP range/conditional semantics. Translate its pre-body
// errors to the same JSON envelope without buffering successful file bytes.
type contentWriter struct {
	http.ResponseWriter
	failed bool
}

func (w *contentWriter) WriteHeader(status int) {
	if status >= 400 {
		w.failed = true
		w.Header().Del("Content-Disposition")
		w.Header().Del("Content-Length")
		code := "invalid_request"
		if status >= 500 {
			code = "internal_error"
		}
		if status == 416 {
			code = "range_not_satisfiable"
		}
		write(w.ResponseWriter, response{status: status, code: code})
		return
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *contentWriter) Write(b []byte) (int, error) {
	if w.failed {
		return len(b), nil
	}
	return w.ResponseWriter.Write(b)
}
