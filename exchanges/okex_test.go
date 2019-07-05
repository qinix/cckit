package exchanges

import "testing"

func TestGetMarkets(t *testing.T) {
	t.Log(Okex.getMarkets())
}
