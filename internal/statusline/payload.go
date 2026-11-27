package statusline

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

type Payload struct {
	CWD                 string                 `json:"cwd"`
	SessionID           string                 `json:"session_id"`
	ConversationID      string                 `json:"conversation_id"`
	ConversationTitle   string                 `json:"conversation_title"`
	Model               *ModelInfo             `json:"model"`
	Workspace           *WorkspaceInfo         `json:"workspace"`
	Version             string                 `json:"version"`
	ContextWindow       *ContextWindowInfo     `json:"context_window"`
	Exceeds200kTokens   *bool                  `json:"exceeds_200k_tokens"`
	Agent               *AgentInfo             `json:"agent"`
	Product             string                 `json:"product"`
	Quota               map[string]QuotaBucket `json:"quota"`
	AgentState          string                 `json:"agent_state"`
	VCS                 *VCSInfo               `json:"vcs"`
	CycleMode           string                 `json:"cycle_mode"`
	Sandbox             *SandboxInfo           `json:"sandbox"`
	ArtifactCount       int                    `json:"artifact_count"`
	TaskCount           int                    `json:"task_count"`
	TerminalWidth       int                    `json:"terminal_width"`
	PendingInputCount   int                    `json:"pending_input_count"`
	ToolConfirmPending  bool                   `json:"tool_confirmation_pending"`
}

type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Effort      string `json:"effort"`
}

type WorkspaceInfo struct {
	CurrentDir string `json:"current_dir"`
	ProjectDir string `json:"project_dir"`
}

type ContextWindowInfo struct {
	TotalInputTokens    int64         `json:"total_input_tokens"`
	TotalOutputTokens   int64         `json:"total_output_tokens"`
	ContextWindowSize   int64         `json:"context_window_size"`
	UsedPercentage      *float64      `json:"used_percentage"`
	RemainingPercentage *float64      `json:"remaining_percentage"`
	CurrentUsage        *CurrentUsage `json:"current_usage"`
}

type CurrentUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

type AgentInfo struct {
	Name string `json:"name"`
}

type QuotaBucket struct {
	RemainingFraction *float64 `json:"remaining_fraction"`
	RemainingAmount   *float64 `json:"remaining_amount"`
	ResetTime         string   `json:"reset_time"`
	ResetInSeconds    *int64   `json:"reset_in_seconds"`
	Disabled          bool     `json:"disabled"`
}

type VCSInfo struct {
	Type   string `json:"type"`
	Branch string `json:"branch"`
	Dirty  bool   `json:"dirty"`
}

type SandboxInfo struct {
	Enabled bool `json:"enabled"`
}

type QuotaEntry struct {
	ID                string
	RemainingFraction *float64
	RemainingAmount   *float64
	ResetInSeconds    int64
}

func (p Payload) WorkDirPath() string {
	if p.Workspace != nil && strings.TrimSpace(p.Workspace.CurrentDir) != "" {
		return strings.TrimSpace(p.Workspace.CurrentDir)
	}
	if strings.TrimSpace(p.CWD) != "" {
		return strings.TrimSpace(p.CWD)
	}
	if p.Workspace != nil && strings.TrimSpace(p.Workspace.ProjectDir) != "" {
		return strings.TrimSpace(p.Workspace.ProjectDir)
	}
	return ""
}

func (p Payload) WorkDirName() string {
	raw := p.WorkDirPath()
	if raw == "" {
		return ""
	}

	trimmed := strings.TrimRight(raw, "/\\")
	if trimmed == "" {
		return "/"
	}

	idx := strings.LastIndexAny(trimmed, "/\\")
	if idx == -1 {
		return trimmed
	}
	return trimmed[idx+1:]
}

func (p Payload) ModelNameAndEffort() (string, string) {
	if p.Model == nil {
		return "", ""
	}
	name := strings.TrimSpace(p.Model.DisplayName)
	if name == "" {
		name = strings.TrimSpace(p.Model.ID)
	}
	effort := strings.TrimSpace(p.Model.Effort)

	if name != "" && effort != "" {
		suffix := "(" + strings.ToLower(effort) + ")"
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, suffix) {
			name = strings.TrimSpace(name[:len(name)-len(suffix)])
		}
	}
	return name, effort
}

func (p Payload) ContextUsedPercent() (float64, bool) {
	if p.ContextWindow == nil {
		return 0, false
	}
	cw := p.ContextWindow
	if cw.UsedPercentage != nil {
		return clampPercent(*cw.UsedPercentage), true
	}
	if cw.RemainingPercentage != nil {
		return clampPercent(100 - *cw.RemainingPercentage), true
	}
	if cw.ContextWindowSize > 0 && cw.CurrentUsage != nil {
		used := cw.CurrentUsage.InputTokens + cw.CurrentUsage.OutputTokens +
			cw.CurrentUsage.CacheCreationInputTokens + cw.CurrentUsage.CacheReadInputTokens
		pct := float64(used) / float64(cw.ContextWindowSize) * 100
		return clampPercent(pct), true
	}
	if cw.ContextWindowSize > 0 {
		return 0, true
	}
	return 0, false
}

func (p Payload) ActiveQuotas() []QuotaEntry {
	if len(p.Quota) == 0 {
		return nil
	}
	entries := make([]QuotaEntry, 0, len(p.Quota))
	for id, b := range p.Quota {
		if b.Disabled {
			continue
		}
		if b.RemainingFraction == nil && b.RemainingAmount == nil {
			continue
		}
		var reset int64
		if b.ResetInSeconds != nil {
			reset = *b.ResetInSeconds
		}
		entries = append(entries, QuotaEntry{
			ID:                id,
			RemainingFraction: b.RemainingFraction,
			RemainingAmount:   b.RemainingAmount,
			ResetInSeconds:    reset,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		ri, rj := quotaOrderRank(entries[i].ID), quotaOrderRank(entries[j].ID)
		if ri != rj {
			return ri < rj
		}
		return entries[i].ID < entries[j].ID
	})
	return entries
}

const (
	quotaRank5h      = 0
	quotaRankWeekly  = 1
	quotaRankMonthly = 2
	quotaRankOther   = 3
)

func quotaOrderRank(id string) int {
	lower := strings.ToLower(strings.TrimSpace(id))
	switch {
	case strings.Contains(lower, "5h") || strings.Contains(lower, "5_hour") || strings.Contains(lower, "5-hour") || strings.Contains(lower, "five_hour") || strings.Contains(lower, "five-hour"):
		return quotaRank5h
	case strings.Contains(lower, "week") || strings.Contains(lower, "7d"):
		return quotaRankWeekly
	case strings.Contains(lower, "month") || strings.Contains(lower, "30d"):
		return quotaRankMonthly
	default:
		return quotaRankOther
	}
}

var quotaWindowTokens = map[string]struct{}{
	"5h": {}, "7d": {}, "30d": {}, "24h": {}, "1d": {},
	"5hr": {}, "5hour": {}, "5": {}, "7": {}, "30": {}, "24": {},
	"hour": {}, "hours": {}, "five": {}, "seven": {},
	"day": {}, "days": {}, "daily": {},
	"week": {}, "weeks": {}, "weekly": {},
	"month": {}, "months": {}, "monthly": {},
	"rolling": {}, "window": {}, "quota": {}, "limit": {}, "rate": {}, "session": {},
}

var knownQuotaProviders = map[string]string{
	"gemini":    "gemini",
	"google":    "gemini",
	"claude":    "claude",
	"anthropic": "claude",
	"gpt":       "gpt",
	"openai":    "gpt",
	"codex":     "gpt",
	"3p":        "3p",
	"deepseek":  "deepseek",
	"llama":     "llama",
	"qwen":      "qwen",
	"mistral":   "mistral",
	"grok":      "grok",
}

func (p Payload) ModelQuotas() []QuotaEntry {
	all := p.ActiveQuotas()
	if len(all) == 0 {
		return nil
	}
	modelProviders, modelTokens := p.modelSeriesTokens()
	if len(modelTokens) == 0 {
		return all
	}

	exact := make([]QuotaEntry, 0, len(all))
	providerFallback := make([]QuotaEntry, 0, len(all))
	hasExactScoped := false

	for _, e := range all {
		providers, qualifiers := classifyQuotaBucket(e.ID)
		if len(providers) == 0 && len(qualifiers) == 0 {
			exact = append(exact, e)
			providerFallback = append(providerFallback, e)
			continue
		}

		providerMatched := len(providers) == 0 || hasAnyToken(providers, modelProviders)
		if !providerMatched {
			continue
		}

		if len(providers) > 0 {
			providerFallback = append(providerFallback, e)
		}
		if hasAllTokens(qualifiers, modelTokens) {
			exact = append(exact, e)
			hasExactScoped = true
		}
	}

	if hasExactScoped {
		return exact
	}
	if len(providerFallback) > 0 {
		return providerFallback
	}
	return exact
}

func (p Payload) modelSeriesTokens() (map[string]bool, map[string]bool) {
	if p.Model == nil {
		return nil, nil
	}
	tokens := make(map[string]bool)
	providers := make(map[string]bool)

	for _, raw := range []string{p.Model.DisplayName, p.Model.ID} {
		for _, tok := range tokenizeAlphaNum(raw) {
			tokens[tok] = true
			if prov, ok := knownQuotaProviders[tok]; ok {
				providers[prov] = true
				continue
			}
			switch tok {
			case "sonnet", "opus", "haiku":
				providers["claude"] = true
			case "o1", "o3", "o4":
				providers["gpt"] = true
			}
		}
	}

	if len(tokens) > 0 && !providers["gemini"] {
		providers["3p"] = true
	}
	return providers, tokens
}

func classifyQuotaBucket(id string) ([]string, []string) {
	tokens := tokenizeAlphaNum(id)
	hasThird := false
	hasParty := false
	for _, tok := range tokens {
		if tok == "third" {
			hasThird = true
		} else if tok == "party" {
			hasParty = true
		}
	}
	isThirdParty := hasThird && hasParty

	var providers []string
	var qualifiers []string
	if isThirdParty {
		providers = append(providers, "3p")
	}

	for _, tok := range tokens {
		if isThirdParty && (tok == "third" || tok == "party") {
			continue
		}
		if _, isWindow := quotaWindowTokens[tok]; isWindow {
			continue
		}
		if prov, ok := knownQuotaProviders[tok]; ok {
			providers = append(providers, prov)
			continue
		}
		qualifiers = append(qualifiers, tok)
	}
	return providers, qualifiers
}

func tokenizeAlphaNum(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func hasAnyToken(needles []string, haystack map[string]bool) bool {
	for _, n := range needles {
		if haystack[n] {
			return true
		}
	}
	return false
}

func hasAllTokens(needles []string, haystack map[string]bool) bool {
	for _, n := range needles {
		if !haystack[n] {
			return false
		}
	}
	return true
}

func (p Payload) EffectiveCycleMode() string {
	mode := strings.TrimSpace(p.CycleMode)
	if mode == "" {
		return "default"
	}
	return mode
}

func (p Payload) Exceeds200k() bool {
	return p.Exceeds200kTokens != nil && *p.Exceeds200kTokens
}

func clampPercent(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
