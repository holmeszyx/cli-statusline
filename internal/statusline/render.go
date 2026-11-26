package statusline

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type QuotaDisplayMode string

const (
	QuotaModeUsed      QuotaDisplayMode = "used"
	QuotaModeRemaining QuotaDisplayMode = "remaining"
)

type Options struct {
	Width     int
	BarWidth  int
	NoColor   bool
	QuotaMode QuotaDisplayMode
}

const (
	defaultBarWidth = 8
	narrowBarWidth  = 6
	wideThreshold   = 100
	mediumThreshold = 60
)

func Render(p Payload, opts Options) string {
	st := styler{noColor: opts.NoColor}
	width := opts.Width
	if width <= 0 {
		width = p.TerminalWidth
	}
	barWidth := opts.BarWidth
	if barWidth <= 0 {
		barWidth = defaultBarWidth
	}
	quotaMode := opts.QuotaMode
	if quotaMode != QuotaModeRemaining {
		quotaMode = QuotaModeUsed
	}

	if width > 0 && width < mediumThreshold {
		compactBar := barWidth
		if compactBar > narrowBarWidth {
			compactBar = narrowBarWidth
		}
		return renderNarrow(p, compactBar, quotaMode, st)
	}
	if width > 0 && width < wideThreshold {
		return renderMedium(p, barWidth, quotaMode, st)
	}
	wide := renderWide(p, barWidth, quotaMode, st)
	if width > 0 && VisibleWidth(wide) > width {
		return renderMedium(p, barWidth, quotaMode, st)
	}
	return wide
}

func renderWide(p Payload, barWidth int, quotaMode QuotaDisplayMode, st styler) string {
	sep := st.paint(" │ ", ansiFgGray)
	parts := make([]string, 0, 5)

	cliStatus := renderCLIStatus(p, false, st)
	if cliStatus != "" {
		parts = append(parts, cliStatus)
	}
	if dir := renderWorkDir(p, false, st); dir != "" {
		parts = append(parts, dir)
	}
	if ctx := renderContext(p, barWidth, false, st); ctx != "" {
		parts = append(parts, ctx)
	}
	if quota := renderQuota(p, barWidth, quotaMode, false, 2, st); quota != "" {
		parts = append(parts, quota)
	}
	if artifacts := renderArtifacts(p, st); artifacts != "" {
		parts = append(parts, artifacts)
	}
	return strings.Join(parts, sep)
}

func renderMedium(p Payload, barWidth int, quotaMode QuotaDisplayMode, st styler) string {
	dot := st.paint(" · ", ansiFgGray)
	pipe := st.paint(" │ ", ansiFgGray)

	line1Parts := make([]string, 0, 2)
	if cliStatus := renderCLIStatus(p, false, st); cliStatus != "" {
		line1Parts = append(line1Parts, cliStatus)
	}
	if dir := renderWorkDir(p, false, st); dir != "" {
		line1Parts = append(line1Parts, dir)
	}

	line2Parts := make([]string, 0, 3)
	if ctx := renderContext(p, barWidth, false, st); ctx != "" {
		line2Parts = append(line2Parts, ctx)
	}
	if quota := renderQuota(p, barWidth, quotaMode, false, 2, st); quota != "" {
		line2Parts = append(line2Parts, quota)
	}
	if artifacts := renderArtifacts(p, st); artifacts != "" {
		line2Parts = append(line2Parts, artifacts)
	}

	top := st.paint("╭─ ", ansiFgGray) + strings.Join(line1Parts, pipe)
	if len(line2Parts) == 0 {
		return top
	}
	bottom := st.paint("╰─ ", ansiFgGray) + strings.Join(line2Parts, dot)
	return top + "\n" + bottom
}

func renderNarrow(p Payload, barWidth int, quotaMode QuotaDisplayMode, st styler) string {
	dot := st.paint(" · ", ansiFgGray)

	line1Parts := make([]string, 0, 2)
	if cliStatus := renderCLIStatus(p, true, st); cliStatus != "" {
		line1Parts = append(line1Parts, cliStatus)
	}
	if dir := renderWorkDir(p, true, st); dir != "" {
		line1Parts = append(line1Parts, dir)
	}

	line2Parts := make([]string, 0, 3)
	if ctx := renderContext(p, barWidth, true, st); ctx != "" {
		line2Parts = append(line2Parts, ctx)
	}
	if quota := renderQuota(p, barWidth, quotaMode, true, 1, st); quota != "" {
		line2Parts = append(line2Parts, quota)
	}
	if artifacts := renderArtifacts(p, st); artifacts != "" {
		line2Parts = append(line2Parts, artifacts)
	}

	line1 := strings.Join(line1Parts, dot)
	if len(line2Parts) == 0 {
		return line1
	}
	return line1 + "\n" + strings.Join(line2Parts, dot)
}

func renderCLIStatus(p Payload, compact bool, st styler) string {
	dot := st.paint(" · ", ansiFgGray)
	parts := make([]string, 0, 3)

	stateDot := renderStateDot(p.AgentState, st)
	mode := renderCycleMode(p.EffectiveCycleMode(), compact, st)
	parts = append(parts, stateDot+" "+mode)

	modelName, effort := p.ModelNameAndEffort()
	if modelName != "" {
		parts = append(parts, st.paint(modelName, ansiFgBrightMag, ansiBold))
	}
	if effort != "" {
		parts = append(parts, renderEffort(effort, st))
	}

	return strings.Join(parts, dot)
}

func renderStateDot(state string, st styler) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "", "idle":
		return st.paint("●", ansiFgBrightGreen)
	case "working", "thinking":
		return st.paint("◐", ansiFgBrightCyan)
	case "tool_use":
		return st.paint("◆", ansiFgBrightMag)
	case "reviewing", "authenticating", "initializing":
		return st.paint("◎", ansiFgBrightYel)
	case "error":
		return st.paint("✕", ansiFgBrightRed, ansiBold)
	default:
		return st.paint("●", ansiFgWhite)
	}
}

func renderCycleMode(mode string, compact bool, st styler) string {
	switch strings.ToLower(mode) {
	case "", "default":
		if compact {
			return st.paint("default", ansiFgWhite)
		}
		return st.paint("▸ default", ansiFgWhite)
	case "plan":
		if compact {
			return st.paint("plan", ansiFgBrightCyan, ansiBold)
		}
		return st.paint("◈ plan", ansiFgBrightCyan, ansiBold)
	case "auto":
		if compact {
			return st.paint("auto", ansiFgBrightGreen, ansiBold)
		}
		return st.paint("✦ auto", ansiFgBrightGreen, ansiBold)
	case "accept-edits":
		if compact {
			return st.paint("edits", ansiFgBrightYel, ansiBold)
		}
		return st.paint("✎ accept-edits", ansiFgBrightYel, ansiBold)
	default:
		if compact {
			return st.paint(mode, ansiFgBrightWhite, ansiBold)
		}
		return st.paint("▸ "+mode, ansiFgBrightWhite, ansiBold)
	}
}

func renderEffort(effort string, st styler) string {
	lower := strings.ToLower(effort)
	switch lower {
	case "low":
		return st.paint(lower, ansiFgCyan)
	case "medium":
		return st.paint(lower, ansiFgBrightCyan)
	case "high":
		return st.paint(lower, ansiFgBrightYel, ansiBold)
	case "max", "xhigh":
		return st.paint(lower, ansiFgBrightRed, ansiBold)
	default:
		return st.paint(lower, ansiFgBrightCyan)
	}
}

func renderWorkDir(p Payload, compact bool, st styler) string {
	name := p.WorkDirName()
	if name == "" {
		return ""
	}
	var out string
	if compact {
		out = st.paint(name, ansiFgBrightBlue, ansiBold)
	} else {
		out = st.paint("dir ", ansiFgGray) + st.paint(name, ansiFgBrightBlue, ansiBold)
	}
	if branch := resolveBranch(p); branch != "" {
		if p.VCS != nil && p.VCS.Dirty {
			out += st.paint(" · ", ansiFgGray) + st.paint(branch, ansiFgBrightRed) + st.paint("*", ansiFgBrightYel)
		} else {
			out += st.paint(" · ", ansiFgGray) + st.paint(branch, ansiFgCyan)
		}
	}
	return out
}

func resolveBranch(p Payload) string {
	if p.VCS == nil {
		return ""
	}
	if branch := strings.TrimSpace(p.VCS.Branch); branch != "" {
		return branch
	}
	if strings.EqualFold(strings.TrimSpace(p.VCS.Type), "git") {
		return detectGitBranch(p.WorkDirPath())
	}
	return ""
}

func renderContext(p Payload, barWidth int, compact bool, st styler) string {
	pct, ok := p.ContextUsedPercent()
	if !ok {
		return st.paint("ctx ", ansiFgGray) + st.paint("--", ansiFgGray)
	}
	pctText := formatPercent(pct, compact)
	if p.Exceeds200k() {
		bar := renderProgressBarWithColor(pct, barWidth, ansiFgBrightMag, st)
		return st.paint("ctx ", ansiFgBrightMag) + bar + " " + st.paint(pctText, ansiFgBrightMag, ansiBold)
	}
	bar := renderProgressBar(pct, barWidth, false, st)
	return st.paint("ctx ", ansiFgGray) + bar + " " + st.paint(pctText, ansiFgBrightWhite, ansiBold)
}

func renderArtifacts(p Payload, st styler) string {
	if p.ArtifactCount <= 0 {
		return ""
	}
	return st.paint("A ", ansiFgGray) + st.paint(strconv.Itoa(p.ArtifactCount), ansiFgBrightCyan, ansiBold)
}

func renderQuota(p Payload, barWidth int, mode QuotaDisplayMode, compact bool, maxBuckets int, st styler) string {
	entries := p.ActiveQuotas()
	if len(entries) == 0 {
		return ""
	}
	if maxBuckets > 0 && len(entries) > maxBuckets {
		entries = entries[:maxBuckets]
	}

	multi := len(entries) > 1
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		label := "quota"
		if multi {
			label = shortQuotaLabel(e.ID)
		}

		if e.RemainingFraction != nil {
			remPct := clampPercent(*e.RemainingFraction * 100)
			displayPct := clampPercent(100 - remPct)
			invert := false
			if mode == QuotaModeRemaining {
				displayPct = remPct
				invert = true
			}
			bar := renderProgressBar(displayPct, barWidth, invert, st)
			pctText := formatPercent(displayPct, compact)
			seg := st.paint(label+" ", ansiFgGray) + bar + " " + st.paint(pctText, ansiFgBrightWhite, ansiBold)
			if !compact && e.ResetInSeconds > 0 {
				if reset := formatReset(e.ResetInSeconds); reset != "" {
					seg += st.paint(" ("+reset+")", ansiFgGray)
				}
			}
			parts = append(parts, seg)
			continue
		}

		if e.RemainingAmount != nil {
			amt := fmt.Sprintf("%.0f", math.Round(*e.RemainingAmount))
			parts = append(parts, st.paint(label+" ", ansiFgGray)+st.paint(amt, ansiFgBrightWhite, ansiBold))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, st.paint(" · ", ansiFgGray))
}

func formatPercent(pct float64, compact bool) string {
	if compact || pct >= 99.95 || pct == math.Trunc(pct) {
		return fmt.Sprintf("%.0f%%", math.Round(pct))
	}
	return fmt.Sprintf("%.1f%%", pct)
}

func shortQuotaLabel(id string) string {
	lower := strings.ToLower(strings.TrimSpace(id))
	switch {
	case strings.Contains(lower, "5h"):
		return "5h"
	case strings.Contains(lower, "week") || strings.Contains(lower, "7d"):
		return "7d"
	case strings.Contains(lower, "day") || strings.Contains(lower, "daily") || strings.Contains(lower, "24h"):
		return "1d"
	case strings.Contains(lower, "month") || strings.Contains(lower, "30d"):
		return "30d"
	}
	trimmed := lower
	for _, prefix := range []string{"gemini-", "gemini_", "claude-", "claude_", "rolling-", "rolling_"} {
		trimmed = strings.TrimPrefix(trimmed, prefix)
	}
	runes := []rune(trimmed)
	if len(runes) > 6 {
		return string(runes[:6])
	}
	if len(runes) == 0 {
		return "quota"
	}
	return trimmed
}

func formatReset(seconds int64) string {
	if seconds <= 0 {
		return ""
	}
	if seconds >= 86400 {
		return fmt.Sprintf("%dd", seconds/86400)
	}
	if seconds >= 3600 {
		return fmt.Sprintf("%dh", seconds/3600)
	}
	if seconds >= 60 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	return "<1m"
}
