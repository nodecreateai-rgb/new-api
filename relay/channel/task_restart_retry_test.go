package channel

import (
	"errors"
	"testing"
)

func TestIsRestartWindowTransportError(t *testing.T) {
	for _, msg := range []string{
		`Post "http://dola2api:38472/v1/videos": dial tcp 10.0.1.10:38472: connect: connection refused`,
		`Post "http://dola2api:38472/v1/videos": read tcp 10.0.1.1:123->10.0.1.2:38472: read: connection reset by peer`,
		`Post "http://dola2api:38472/v1/videos": EOF`,
		`read response: unexpected EOF`,
		`dial tcp: lookup dola2api on 127.0.0.11:53: server misbehaving`,
	} {
		if !isRestartWindowTransportError(errors.New(msg)) {
			t.Fatalf("expected retryable restart transport error: %s", msg)
		}
	}
	for _, msg := range []string{
		`context deadline exceeded`,
		`x509: certificate signed by unknown authority`,
		`upstream returned HTTP 400`,
	} {
		if isRestartWindowTransportError(errors.New(msg)) {
			t.Fatalf("must not retry non-restart error: %s", msg)
		}
	}
}
