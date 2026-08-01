// FORK-CUSTOM: Protect provider credential redaction semantics.
package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeProviderSyncError(t *testing.T) {
	err := errors.New("upstream rejected Authorization Bearer secret-value")
	message := sanitizeProviderSyncError(err, "secret-value")
	assert.Equal(t, "upstream rejected Authorization Bearer [redacted]", message)
}
