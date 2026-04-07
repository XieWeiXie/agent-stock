package cli

import (
	"fmt"
	"strings"

	"agent-stock/internal/provider"
)

func parseMarket(s string) (provider.Market, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "ab":
		return provider.MarketAB, nil
	case "hk":
		return provider.MarketHK, nil
	case "us":
		return provider.MarketUS, nil
	default:
		return "", fmt.Errorf("invalid market: %s (allowed: ab|hk|us)", s)
	}
}
