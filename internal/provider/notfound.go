package provider

import "net/http"

// statusCoder is implemented by every oapi-codegen `*WithResponse` type.
type statusCoder interface {
	StatusCode() int
}

// IsNotFound reports whether the API response is HTTP 404. Use it on the
// typed `*WithResponse` value after the err / JSON200 checks. When true,
// callers should `resp.State.RemoveResource(ctx)` (Read/Update) or
// no-op-return (Delete) — the row is gone, state should follow.
//
// Why not just `JSON404 != nil`: not every endpoint declares a typed 404
// response schema, so the field can be missing even when StatusCode() is
// 404 (body shape mismatch). StatusCode() is universal across endpoints.
func IsNotFound(resp statusCoder) bool {
	return resp != nil && resp.StatusCode() == http.StatusNotFound
}
