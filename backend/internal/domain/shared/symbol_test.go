package shared_test

import (
	"testing"

	"github.com/binancetracker/binancetracker/internal/domain/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSymbol(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"lower case is upcased", "btc", "BTC", false},
		{"trims whitespace", "  eth ", "ETH", false},
		{"alphanumeric ok", "1INCH", "1INCH", false},
		{"empty rejected", "", "", true},
		{"hyphen rejected", "BTC-USD", "", true},
		{"slash rejected", "BTC/USDT", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := shared.NewSymbol(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.String())
		})
	}
}

func TestSymbolEquality(t *testing.T) {
	t.Parallel()
	a, _ := shared.NewSymbol("btc")
	b, _ := shared.NewSymbol("BTC")
	assert.True(t, a.Equals(b))
	assert.False(t, a.IsZero())
}
