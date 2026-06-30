package chat

import (
	"fmt"
	"strconv"
)

func formatTokens(tokens int) string {
	switch {
	case tokens >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(tokens)/1_000_000)
	case tokens >= 1_000:
		return fmt.Sprintf("%.2fK", float64(tokens)/1_000)
	default:
		return strconv.Itoa(tokens)
	}
}
