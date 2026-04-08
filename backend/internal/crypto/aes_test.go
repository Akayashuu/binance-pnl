package crypto_test

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/binancetracker/binancetracker/internal/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newKey(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(b)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	t.Parallel()
	c, err := crypto.NewAESGCM(newKey(t))
	require.NoError(t, err)

	const secret = "binance-secret-very-long-12345"
	ct, err := c.Encrypt(secret)
	require.NoError(t, err)
	assert.NotEqual(t, secret, ct)

	pt, err := c.Decrypt(ct)
	require.NoError(t, err)
	assert.Equal(t, secret, pt)
}

func TestRejectsBadKey(t *testing.T) {
	t.Parallel()
	_, err := crypto.NewAESGCM("not-base64!")
	assert.Error(t, err)

	tooShort := base64.StdEncoding.EncodeToString([]byte("short"))
	_, err = crypto.NewAESGCM(tooShort)
	assert.ErrorIs(t, err, crypto.ErrInvalidKey)
}
