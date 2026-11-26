package statusline

import (
	"math"
	"sort"
	"strings"
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

func (p Payload) WorkDirName() string {
	raw := ""
	if p.Workspace != nil && strings.TrimSpace(p.Workspace.CurrentDir) != "" {
		raw = p.Workspace.CurrentDir
	} else if strings.TrimSpace(p.CWD) != "" {
		raw = p.CWD
	} else if p.Workspace != nil && strings.TrimSpace(p.Workspace.ProjectDir) != "" {
		raw = p.Workspace.ProjectDir
	}

	raw = strings.TrimSpace(raw)
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
		fi, fj := entries[i].RemainingFraction, entries[j].RemainingFraction
		if fi != nil && fj != nil {
			if math.Abs(*fi-*fj) > 1e-9 {
				return *fi < *fj
			}
		} else if fi != nil {
			return true
		} else if fj != nil {
			return false
		}
		return entries[i].ID < entries[j].ID
	})
	return entries
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
