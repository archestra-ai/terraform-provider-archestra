package provider

import "fmt"

// mustAs asserts v is a T, panicking with the actual type otherwise. Test fakes
// use it on decoded JSON, where a wrong shape is a test bug, not a case to handle.
func mustAs[T any](v any) T {
	out, ok := v.(T)
	if !ok {
		var want T
		panic(fmt.Sprintf("expected %T, got %T", want, v))
	}
	return out
}
