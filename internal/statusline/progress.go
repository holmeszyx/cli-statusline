package statusline

import "strings"

func renderProgressBar(pct float64, width int, invertSeverity bool, st styler) string {
	return renderProgressBarWithColor(pct, width, severityColor(clampPercent(pct), invertSeverity), st)
}

func renderProgressBarWithColor(pct float64, width int, barColor string, st styler) string {
	if width <= 0 {
		return ""
	}
	pct = clampPercent(pct)
	scaled := pct * float64(width) / 100.0
	filled := int(scaled)
	frac := scaled - float64(filled)

	var filledBuf strings.Builder
	var emptyBuf strings.Builder

	for i := 0; i < width; i++ {
		if i < filled {
			filledBuf.WriteRune('█')
			continue
		}
		if i == filled {
			switch {
			case frac >= 0.75:
				filledBuf.WriteRune('▓')
			case frac >= 0.50:
				filledBuf.WriteRune('▒')
			case frac >= 0.25:
				filledBuf.WriteRune('░')
			default:
				emptyBuf.WriteRune('·')
			}
			continue
		}
		emptyBuf.WriteRune('·')
	}

	return st.paint(filledBuf.String(), barColor) + st.paint(emptyBuf.String(), ansiFgGray)
}

func severityColor(pct float64, invert bool) string {
	if invert {
		switch {
		case pct <= 15:
			return ansiFgBrightRed
		case pct <= 40:
			return ansiFgBrightYel
		default:
			return ansiFgBrightGreen
		}
	}
	switch {
	case pct >= 85:
		return ansiFgBrightRed
	case pct >= 60:
		return ansiFgBrightYel
	default:
		return ansiFgBrightGreen
	}
}
