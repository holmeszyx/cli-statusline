package statusline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func floatPtr(v float64) *float64 { return &v }
func int64Ptr(v int64) *int64     { return &v }
func boolPtr(v bool) *bool        { return &v }

func TestWorkDirName(t *testing.T) {
	tests := []struct {
		name    string
		payload Payload
		want    string
	}{
		{
			name: "workspace current_dir unix",
			payload: Payload{
				CWD:       "/fallback/dir",
				Workspace: &WorkspaceInfo{CurrentDir: "/home/user/my-project"},
			},
			want: "my-project",
		},
		{
			name: "workspace current_dir windows with trailing slash",
			payload: Payload{
				Workspace: &WorkspaceInfo{CurrentDir: `F:\GoProject\cli-statusline\`},
			},
			want: "cli-statusline",
		},
		{
			name: "fallback to cwd",
			payload: Payload{
				CWD: `C:\Users\dev\app`,
			},
			want: "app",
		},
		{
			name: "fallback to project_dir",
			payload: Payload{
				Workspace: &WorkspaceInfo{ProjectDir: "/repo/root/"},
			},
			want: "root",
		},
		{
			name: "root directory",
			payload: Payload{
				CWD: "/",
			},
			want: "/",
		},
		{
			name:    "empty",
			payload: Payload{},
			want:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.payload.WorkDirName(); got != tc.want {
				t.Fatalf("WorkDirName() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestModelNameAndEffort(t *testing.T) {
	tests := []struct {
		name       string
		model      *ModelInfo
		wantName   string
		wantEffort string
	}{
		{
			name:       "nil model",
			model:      nil,
			wantName:   "",
			wantEffort: "",
		},
		{
			name: "strips duplicated effort suffix",
			model: &ModelInfo{
				ID:          "Gemini 4 Argon (Medium)",
				DisplayName: "Gemini 4 Argon (Medium)",
				Effort:      "medium",
			},
			wantName:   "Gemini 4 Argon",
			wantEffort: "medium",
		},
		{
			name: "keeps distinct name and effort",
			model: &ModelInfo{
				ID:          "gemini-3-pro",
				DisplayName: "Gemini 3 Pro",
				Effort:      "high",
			},
			wantName:   "Gemini 3 Pro",
			wantEffort: "high",
		},
		{
			name: "falls back to id",
			model: &ModelInfo{
				ID:     "gemini-flash",
				Effort: "low",
			},
			wantName:   "gemini-flash",
			wantEffort: "low",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := Payload{Model: tc.model}
			gotName, gotEffort := p.ModelNameAndEffort()
			if gotName != tc.wantName || gotEffort != tc.wantEffort {
				t.Fatalf("ModelNameAndEffort() = (%q, %q), want (%q, %q)", gotName, gotEffort, tc.wantName, tc.wantEffort)
			}
		})
	}
}

func TestContextUsedPercent(t *testing.T) {
	tests := []struct {
		name    string
		cw      *ContextWindowInfo
		wantPct float64
		wantOK  bool
	}{
		{
			name:   "nil context window",
			cw:     nil,
			wantOK: false,
		},
		{
			name: "direct used percentage",
			cw: &ContextWindowInfo{
				UsedPercentage: floatPtr(14.24),
			},
			wantPct: 14.24,
			wantOK:  true,
		},
		{
			name: "computed from remaining percentage",
			cw: &ContextWindowInfo{
				RemainingPercentage: floatPtr(75.5),
			},
			wantPct: 24.5,
			wantOK:  true,
		},
		{
			name: "computed from current usage",
			cw: &ContextWindowInfo{
				ContextWindowSize: 1000,
				CurrentUsage: &CurrentUsage{
					InputTokens:          200,
					OutputTokens:         50,
					CacheReadInputTokens: 50,
				},
			},
			wantPct: 30,
			wantOK:  true,
		},
		{
			name: "clamps out of bounds",
			cw: &ContextWindowInfo{
				UsedPercentage: floatPtr(142),
			},
			wantPct: 100,
			wantOK:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := Payload{ContextWindow: tc.cw}
			got, ok := p.ContextUsedPercent()
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && got != tc.wantPct {
				t.Fatalf("pct = %v, want %v", got, tc.wantPct)
			}
		})
	}
}

func TestRenderProgressBar(t *testing.T) {
	st := styler{noColor: true}
	tests := []struct {
		name string
		pct  float64
		w    int
		want string
	}{
		{name: "zero", pct: 0, w: 8, want: "········"},
		{name: "quarter of one block", pct: 4, w: 8, want: "░·······"},
		{name: "half", pct: 50, w: 8, want: "████····"},
		{name: "full", pct: 100, w: 8, want: "████████"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := renderProgressBar(tc.pct, tc.w, false, st)
			if got != tc.want {
				t.Fatalf("renderProgressBar(%v, %d) = %q, want %q", tc.pct, tc.w, got, tc.want)
			}
		})
	}
}

func TestRenderOfficialPayload(t *testing.T) {
	raw := `{
		"cwd": "/home/user/my-project",
		"model": {
			"id": "Gemini 4 Argon (Medium)",
			"display_name": "Gemini 4 Argon (Medium)",
			"effort": "medium"
		},
		"workspace": {
			"current_dir": "/home/user/my-project",
			"project_dir": "/home/user/my-project"
		},
		"context_window": {
			"context_window_size": 1048576,
			"used_percentage": 14.24,
			"remaining_percentage": 85.76
		},
		"quota": {
			"gemini-weekly": {
				"remaining_fraction": 0.9378,
				"reset_time": "2026-07-06T07:50:32Z",
				"reset_in_seconds": 560580
			}
		},
		"agent_state": "idle",
		"cycle_mode": "plan",
		"terminal_width": 120
	}`

	var p Payload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	got := Render(p, Options{NoColor: true})
	for _, want := range []string{
		"◈ plan",
		"Gemini 4 Argon",
		"medium",
		"dir my-project",
		"ctx █······· 14.2%",
		"quota ░······· 6.2% (6d)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Render() = %q, expected substring %q", got, want)
		}
	}
	if strings.Contains(got, "\n") {
		t.Errorf("expected single line for width 120, got %q", got)
	}
}

func TestRenderResponsiveLayouts(t *testing.T) {
	p := Payload{
		Workspace: &WorkspaceInfo{CurrentDir: "/code/cli-statusline"},
		Model: &ModelInfo{
			DisplayName: "Gemini 3 Pro",
			Effort:      "high",
		},
		ContextWindow: &ContextWindowInfo{
			UsedPercentage: floatPtr(68.5),
		},
		Quota: map[string]QuotaBucket{
			"rolling_5h": {
				RemainingFraction: floatPtr(0.20),
				ResetInSeconds:    int64Ptr(7200),
			},
			"gemini-weekly": {
				RemainingFraction: floatPtr(0.65),
				ResetInSeconds:    int64Ptr(172800),
			},
			"disabled-bucket": {
				RemainingFraction: floatPtr(0.05),
				Disabled:          true,
			},
		},
		AgentState: "working",
		CycleMode:  "auto",
	}

	tests := []struct {
		name      string
		width     int
		quotaMode QuotaDisplayMode
		wantLines int
		mustHave  []string
		mustOmit  []string
	}{
		{
			name:      "wide single line",
			width:     140,
			quotaMode: QuotaModeUsed,
			wantLines: 1,
			mustHave:  []string{"◐ ✦ auto", "Gemini 3 Pro", "high", "dir cli-statusline", "ctx █████░·· 68.5%", "5h ██████░· 80% (2h)", "7d ██▓····· 35% (2d)"},
			mustOmit:  []string{"disabled"},
		},
		{
			name:      "medium two-line frame",
			width:     80,
			quotaMode: QuotaModeRemaining,
			wantLines: 2,
			mustHave:  []string{"╭─ ", "╰─ ", "5h █▒······ 20% (2h)", "7d █████··· 65% (2d)"},
		},
		{
			name:      "narrow compact two-line",
			width:     50,
			quotaMode: QuotaModeUsed,
			wantLines: 2,
			mustHave:  []string{"◐ auto", "cli-statusline", "ctx ████·· 69%", "quota ████▓· 80%"},
			mustOmit:  []string{"╭─", "(2h)", "7d"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Render(p, Options{
				Width:     tc.width,
				NoColor:   true,
				QuotaMode: tc.quotaMode,
			})
			lines := strings.Split(got, "\n")
			if len(lines) != tc.wantLines {
				t.Fatalf("got %d lines (%q), want %d", len(lines), got, tc.wantLines)
			}
			for _, s := range tc.mustHave {
				if !strings.Contains(got, s) {
					t.Errorf("output %q missing %q", got, s)
				}
			}
			for _, s := range tc.mustOmit {
				if strings.Contains(got, s) {
					t.Errorf("output %q should not contain %q", got, s)
				}
			}
		})
	}
}

func TestRenderANSIAndWidthFallback(t *testing.T) {
	p := Payload{
		Workspace: &WorkspaceInfo{CurrentDir: "/code/very-long-workspace-directory-name"},
		Model: &ModelInfo{
			DisplayName: "Gemini 4 Neon",
			Effort:      "max",
		},
		ContextWindow: &ContextWindowInfo{
			UsedPercentage: floatPtr(92.0),
		},
		Quota: map[string]QuotaBucket{
			"gemini-weekly": {
				RemainingFraction: floatPtr(0.10),
				ResetInSeconds:    int64Ptr(3600),
			},
		},
		CycleMode:     "accept-edits",
		TerminalWidth: 102,
	}

	colored := Render(p, Options{NoColor: false})
	if !strings.Contains(colored, "\033[") {
		t.Fatalf("expected ANSI escape sequences in colored output: %q", colored)
	}
	plain := StripANSI(colored)
	if !strings.Contains(plain, "\n") {
		t.Fatalf("expected automatic fallback to 2-line layout when wide line exceeds terminal_width 102, got %q", plain)
	}
	for _, line := range strings.Split(plain, "\n") {
		if VisibleWidth(line) > 102 {
			t.Errorf("line width %d exceeds 102: %q", VisibleWidth(line), line)
		}
	}
}

func TestRenderArtifacts(t *testing.T) {
	tests := []struct {
		name          string
		artifactCount int
		width         int
		wantSub       string
		wantAbsent    bool
	}{
		{
			name:          "omitted when zero",
			artifactCount: 0,
			width:         120,
			wantSub:       "A ",
			wantAbsent:    true,
		},
		{
			name:          "wide shows A count",
			artifactCount: 3,
			width:         120,
			wantSub:       "│ A 3",
		},
		{
			name:          "medium shows A count on metrics line",
			artifactCount: 2,
			width:         80,
			wantSub:       "· A 2",
		},
		{
			name:          "narrow shows A count on metrics line",
			artifactCount: 5,
			width:         50,
			wantSub:       "· A 5",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := Payload{
				Workspace:     &WorkspaceInfo{CurrentDir: "/code/app"},
				ContextWindow: &ContextWindowInfo{UsedPercentage: floatPtr(20)},
				ArtifactCount: tc.artifactCount,
			}
			got := Render(p, Options{Width: tc.width, NoColor: true})
			has := strings.Contains(got, tc.wantSub)
			if tc.wantAbsent && has {
				t.Fatalf("Render() = %q, should not contain %q", got, tc.wantSub)
			}
			if !tc.wantAbsent && !has {
				t.Fatalf("Render() = %q, expected substring %q", got, tc.wantSub)
			}
		})
	}
}

func TestRenderContextExceeds200kColor(t *testing.T) {
	tests := []struct {
		name       string
		exceeds    *bool
		wantPrefix string
	}{
		{
			name:       "normal below 200k uses gray label and green bar",
			exceeds:    boolPtr(false),
			wantPrefix: ansiFgGray + "ctx " + ansiReset + ansiFgBrightGreen,
		},
		{
			name:       "nil exceeds uses normal color",
			exceeds:    nil,
			wantPrefix: ansiFgGray + "ctx " + ansiReset + ansiFgBrightGreen,
		},
		{
			name:       "exceeds 200k uses bright magenta indicator",
			exceeds:    boolPtr(true),
			wantPrefix: ansiFgBrightMag + "ctx " + ansiReset + ansiFgBrightMag,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := Payload{
				ContextWindow:     &ContextWindowInfo{UsedPercentage: floatPtr(25)},
				Exceeds200kTokens: tc.exceeds,
			}
			got := renderContext(p, 8, false, styler{noColor: false})
			if !strings.HasPrefix(got, tc.wantPrefix) {
				t.Fatalf("renderContext() = %q, want prefix %q", got, tc.wantPrefix)
			}
		})
	}
}

func TestRenderGitBranchDetection(t *testing.T) {
	t.Run("detects branch from .git/HEAD in parent dir", func(t *testing.T) {
		root := t.TempDir()
		repoDir := filepath.Join(root, "my-repo")
		subDir := filepath.Join(repoDir, "cmd", "app")
		if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, ".git", "HEAD"), []byte("ref: refs/heads/feat/status-bar\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		p := Payload{
			Workspace: &WorkspaceInfo{CurrentDir: subDir},
			VCS:       &VCSInfo{Type: "git"},
		}
		got := Render(p, Options{Width: 120, NoColor: true})
		if !strings.Contains(got, "dir app · feat/status-bar") {
			t.Fatalf("Render() = %q, expected %q", got, "dir app · feat/status-bar")
		}
	})

	t.Run("detects branch from worktree .git file", func(t *testing.T) {
		root := t.TempDir()
		wtDir := filepath.Join(root, "wt-repo")
		gitMeta := filepath.Join(root, "meta", "worktrees", "wt1")
		if err := os.MkdirAll(wtDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(gitMeta, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wtDir, ".git"), []byte("gitdir: ../meta/worktrees/wt1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gitMeta, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		p := Payload{
			Workspace: &WorkspaceInfo{CurrentDir: wtDir},
			VCS:       &VCSInfo{Type: "git"},
		}
		got := Render(p, Options{Width: 120, NoColor: true})
		if !strings.Contains(got, "dir wt-repo · main") {
			t.Fatalf("Render() = %q, expected %q", got, "dir wt-repo · main")
		}
	})

	t.Run("detached HEAD shows short hash", func(t *testing.T) {
		repoDir := filepath.Join(t.TempDir(), "detached-repo")
		if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, ".git", "HEAD"), []byte("a1b2c3d4e5f60718293a4b5c6d7e8f9012345678\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		p := Payload{
			Workspace: &WorkspaceInfo{CurrentDir: repoDir},
			VCS:       &VCSInfo{Type: "git"},
		}
		got := Render(p, Options{Width: 120, NoColor: true})
		if !strings.Contains(got, "dir detached-repo · a1b2c3d") {
			t.Fatalf("Render() = %q, expected %q", got, "dir detached-repo · a1b2c3d")
		}
	})

	t.Run("skips git detection when vcs is not git", func(t *testing.T) {
		repoDir := filepath.Join(t.TempDir(), "non-git-vcs")
		if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		p := Payload{
			Workspace: &WorkspaceInfo{CurrentDir: repoDir},
			VCS:       &VCSInfo{Type: "jj"},
		}
		got := Render(p, Options{Width: 120, NoColor: true})
		if strings.Contains(got, "· main") {
			t.Fatalf("Render() = %q, should not detect git branch when vcs.type is jj", got)
		}
	})

	t.Run("prefers explicit payload branch and formats with dot", func(t *testing.T) {
		p := Payload{
			Workspace: &WorkspaceInfo{CurrentDir: "/code/my-app"},
			VCS:       &VCSInfo{Type: "git", Branch: "develop", Dirty: true},
		}
		got := Render(p, Options{Width: 120, NoColor: true})
		if !strings.Contains(got, "dir my-app · develop*") {
			t.Fatalf("Render() = %q, expected %q", got, "dir my-app · develop*")
		}
	})
}

func TestActiveQuotasOrder(t *testing.T) {
	tests := []struct {
		name    string
		quota   map[string]QuotaBucket
		wantIDs []string
	}{
		{
			name: "fixed order 5h weekly monthly regardless of remaining fraction",
			quota: map[string]QuotaBucket{
				"gemini-monthly": {RemainingFraction: floatPtr(0.05)},
				"gemini-weekly":  {RemainingFraction: floatPtr(0.15)},
				"rolling_5h":     {RemainingFraction: floatPtr(0.90)},
			},
			wantIDs: []string{"rolling_5h", "gemini-weekly", "gemini-monthly"},
		},
		{
			name: "5h before weekly even when weekly has lower remaining",
			quota: map[string]QuotaBucket{
				"gemini-weekly": {RemainingFraction: floatPtr(0.10)},
				"rolling_5h":    {RemainingFraction: floatPtr(0.85)},
			},
			wantIDs: []string{"rolling_5h", "gemini-weekly"},
		},
		{
			name: "5h before monthly when weekly absent",
			quota: map[string]QuotaBucket{
				"monthly": {RemainingFraction: floatPtr(0.10)},
				"5h":      {RemainingFraction: floatPtr(0.90)},
			},
			wantIDs: []string{"5h", "monthly"},
		},
		{
			name: "weekly before monthly when 5h absent",
			quota: map[string]QuotaBucket{
				"30d": {RemainingFraction: floatPtr(0.10)},
				"7d":  {RemainingFraction: floatPtr(0.80)},
			},
			wantIDs: []string{"7d", "30d"},
		},
		{
			name: "alternate names and extra bucket after fixed buckets",
			quota: map[string]QuotaBucket{
				"custom-extra": {RemainingFraction: floatPtr(0.01)},
				"30d":          {RemainingFraction: floatPtr(0.20)},
				"7d":           {RemainingFraction: floatPtr(0.40)},
				"five_hour":    {RemainingFraction: floatPtr(0.95)},
			},
			wantIDs: []string{"five_hour", "7d", "30d", "custom-extra"},
		},
		{
			name: "amount-only bucket preserves fixed order",
			quota: map[string]QuotaBucket{
				"gemini-monthly": {RemainingFraction: floatPtr(0.10)},
				"gemini-weekly":  {RemainingAmount: floatPtr(50)},
				"rolling_5h":     {RemainingAmount: floatPtr(100)},
			},
			wantIDs: []string{"rolling_5h", "gemini-weekly", "gemini-monthly"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := Payload{Quota: tc.quota}
			got := p.ActiveQuotas()
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("ActiveQuotas() len = %d, want %d (%v)", len(got), len(tc.wantIDs), got)
			}
			for i, wantID := range tc.wantIDs {
				if got[i].ID != wantID {
					t.Fatalf("ActiveQuotas()[%d].ID = %q, want %q (all=%v)", i, got[i].ID, wantID, got)
				}
			}
		})
	}
}

func TestRenderQuotaFixedOrder(t *testing.T) {
	allThree := Payload{
		Workspace:     &WorkspaceInfo{CurrentDir: "/code/app"},
		ContextWindow: &ContextWindowInfo{UsedPercentage: floatPtr(20)},
		Quota: map[string]QuotaBucket{
			"gemini-monthly": {
				RemainingFraction: floatPtr(0.10),
				ResetInSeconds:    int64Ptr(864000),
			},
			"gemini-weekly": {
				RemainingFraction: floatPtr(0.50),
				ResetInSeconds:    int64Ptr(172800),
			},
			"rolling_5h": {
				RemainingFraction: floatPtr(0.90),
				ResetInSeconds:    int64Ptr(7200),
			},
		},
	}

	tests := []struct {
		name        string
		payload     Payload
		width       int
		wantOrdered []string
		mustOmit    []string
	}{
		{
			name:    "wide renders 5h weekly monthly in fixed order",
			payload: allThree,
			width:   160,
			wantOrdered: []string{
				"5h ▓······· 10% (2h)",
				"7d ████···· 50% (2d)",
				"30d ███████· 90% (10d)",
			},
		},
		{
			name:    "medium renders 5h weekly monthly in fixed order",
			payload: allThree,
			width:   80,
			wantOrdered: []string{
				"5h ▓······· 10% (2h)",
				"7d ████···· 50% (2d)",
				"30d ███████· 90% (10d)",
			},
		},
		{
			name:    "narrow keeps first bucket in fixed order (5h)",
			payload: allThree,
			width:   50,
			wantOrdered: []string{
				"quota ▒····· 10%",
			},
			mustOmit: []string{"7d", "30d"},
		},
		{
			name: "wide without monthly keeps 5h before weekly even when weekly is higher usage",
			payload: Payload{
				Workspace:     &WorkspaceInfo{CurrentDir: "/code/app"},
				ContextWindow: &ContextWindowInfo{UsedPercentage: floatPtr(20)},
				Quota: map[string]QuotaBucket{
					"gemini-weekly": {
						RemainingFraction: floatPtr(0.05),
						ResetInSeconds:    int64Ptr(172800),
					},
					"rolling_5h": {
						RemainingFraction: floatPtr(0.95),
						ResetInSeconds:    int64Ptr(7200),
					},
				},
			},
			width: 140,
			wantOrdered: []string{
				"5h ░······· 5% (2h)",
				"7d ███████▒ 95% (2d)",
			},
			mustOmit: []string{"30d"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Render(tc.payload, Options{Width: tc.width, NoColor: true})
			lastIdx := -1
			for _, sub := range tc.wantOrdered {
				idx := strings.Index(got, sub)
				if idx == -1 {
					t.Fatalf("Render() = %q, missing %q", got, sub)
				}
				if idx <= lastIdx {
					t.Fatalf("Render() = %q, expected %q after previous segment (idx=%d, lastIdx=%d)", got, sub, idx, lastIdx)
				}
				lastIdx = idx
			}
			for _, sub := range tc.mustOmit {
				if strings.Contains(got, sub) {
					t.Fatalf("Render() = %q, should not contain %q", got, sub)
				}
			}
		})
	}
}

func TestModelQuotas(t *testing.T) {
	tests := []struct {
		name    string
		model   *ModelInfo
		quota   map[string]QuotaBucket
		wantIDs []string
	}{
		{
			name:  "gemini neon selects only gemini series over 3p and claude",
			model: &ModelInfo{ID: "Gemini 4 Neon (High)", DisplayName: "Gemini 4 Neon (High)"},
			quota: map[string]QuotaBucket{
				"gemini-5h":     {RemainingFraction: floatPtr(0.80)},
				"gemini-weekly": {RemainingFraction: floatPtr(0.60)},
				"3p-5h":         {RemainingFraction: floatPtr(0.20)},
				"3p-weekly":     {RemainingFraction: floatPtr(0.10)},
				"claude-5h":     {RemainingFraction: floatPtr(0.30)},
			},
			wantIDs: []string{"gemini-5h", "gemini-weekly"},
		},
		{
			name:  "gemini neon keeps gemini 5h weekly monthly in fixed order",
			model: &ModelInfo{DisplayName: "Gemini 4 Neon"},
			quota: map[string]QuotaBucket{
				"gemini-monthly": {RemainingFraction: floatPtr(0.90)},
				"gemini-weekly":  {RemainingFraction: floatPtr(0.70)},
				"gemini-5h":      {RemainingFraction: floatPtr(0.50)},
				"3p-5h":          {RemainingFraction: floatPtr(0.10)},
				"3p-weekly":      {RemainingFraction: floatPtr(0.10)},
			},
			wantIDs: []string{"gemini-5h", "gemini-weekly", "gemini-monthly"},
		},
		{
			name:  "gemini neon excludes argon-specific bucket when standard gemini exists",
			model: &ModelInfo{DisplayName: "Gemini 4 Neon (High)"},
			quota: map[string]QuotaBucket{
				"gemini-5h":           {RemainingFraction: floatPtr(0.80)},
				"gemini-weekly":       {RemainingFraction: floatPtr(0.60)},
				"gemini-argon-weekly": {RemainingFraction: floatPtr(0.40)},
				"3p-5h":               {RemainingFraction: floatPtr(0.20)},
			},
			wantIDs: []string{"gemini-5h", "gemini-weekly"},
		},
		{
			name:  "gemini argon matches gemini and argon-specific buckets",
			model: &ModelInfo{DisplayName: "Gemini 4 Argon (Medium)"},
			quota: map[string]QuotaBucket{
				"gemini-5h":           {RemainingFraction: floatPtr(0.80)},
				"gemini-argon-weekly": {RemainingFraction: floatPtr(0.40)},
				"3p-5h":               {RemainingFraction: floatPtr(0.20)},
			},
			wantIDs: []string{"gemini-5h", "gemini-argon-weekly"},
		},
		{
			name:  "claude sonnet selects 3p series over gemini",
			model: &ModelInfo{DisplayName: "Claude Sonnet 4.6"},
			quota: map[string]QuotaBucket{
				"gemini-5h":     {RemainingFraction: floatPtr(0.80)},
				"gemini-weekly": {RemainingFraction: floatPtr(0.60)},
				"3p-5h":         {RemainingFraction: floatPtr(0.20)},
				"3p-weekly":     {RemainingFraction: floatPtr(0.10)},
			},
			wantIDs: []string{"3p-5h", "3p-weekly"},
		},
		{
			name:  "claude sonnet matches shared claude-gpt pool",
			model: &ModelInfo{DisplayName: "Claude Sonnet 4.6"},
			quota: map[string]QuotaBucket{
				"gemini-5h":         {RemainingFraction: floatPtr(0.80)},
				"claude-gpt-5h":     {RemainingFraction: floatPtr(0.50)},
				"claude-gpt-weekly": {RemainingFraction: floatPtr(0.40)},
			},
			wantIDs: []string{"claude-gpt-5h", "claude-gpt-weekly"},
		},
		{
			name:  "gpt model matches shared claude-gpt pool and excludes gemini",
			model: &ModelInfo{DisplayName: "GPT-5"},
			quota: map[string]QuotaBucket{
				"gemini-5h":         {RemainingFraction: floatPtr(0.80)},
				"claude-gpt-5h":     {RemainingFraction: floatPtr(0.50)},
				"claude-gpt-weekly": {RemainingFraction: floatPtr(0.40)},
			},
			wantIDs: []string{"claude-gpt-5h", "claude-gpt-weekly"},
		},
		{
			name:  "gemini neon hides unrelated 3p buckets when no gemini bucket exists",
			model: &ModelInfo{DisplayName: "Gemini 4 Neon"},
			quota: map[string]QuotaBucket{
				"3p-5h":     {RemainingFraction: floatPtr(0.20)},
				"3p-weekly": {RemainingFraction: floatPtr(0.10)},
			},
			wantIDs: nil,
		},
		{
			name:  "generic buckets kept alongside matching model buckets",
			model: &ModelInfo{DisplayName: "Gemini 3 Pro"},
			quota: map[string]QuotaBucket{
				"rolling_5h":    {RemainingFraction: floatPtr(0.80)},
				"gemini-weekly": {RemainingFraction: floatPtr(0.60)},
				"claude-weekly": {RemainingFraction: floatPtr(0.20)},
			},
			wantIDs: []string{"rolling_5h", "gemini-weekly"},
		},
		{
			name:  "nil model keeps all active quotas",
			model: nil,
			quota: map[string]QuotaBucket{
				"gemini-5h": {RemainingFraction: floatPtr(0.80)},
				"3p-5h":     {RemainingFraction: floatPtr(0.20)},
			},
			wantIDs: []string{"3p-5h", "gemini-5h"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := Payload{Model: tc.model, Quota: tc.quota}
			got := p.ModelQuotas()
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("ModelQuotas() len = %d, want %d (%v)", len(got), len(tc.wantIDs), got)
			}
			for i, wantID := range tc.wantIDs {
				if got[i].ID != wantID {
					t.Fatalf("ModelQuotas()[%d].ID = %q, want %q (all=%v)", i, got[i].ID, wantID, got)
				}
			}
		})
	}
}

func TestRenderModelSeriesQuotaFilter(t *testing.T) {
	p := Payload{
		Workspace: &WorkspaceInfo{CurrentDir: "/code/app"},
		Model: &ModelInfo{
			ID:          "Gemini 4 Neon (High)",
			DisplayName: "Gemini 4 Neon (High)",
			Effort:      "high",
		},
		ContextWindow: &ContextWindowInfo{UsedPercentage: floatPtr(25)},
		Quota: map[string]QuotaBucket{
			"gemini-5h": {
				RemainingFraction: floatPtr(0.80),
				ResetInSeconds:    int64Ptr(7200),
			},
			"gemini-weekly": {
				RemainingFraction: floatPtr(0.60),
				ResetInSeconds:    int64Ptr(172800),
			},
			"3p-5h": {
				RemainingFraction: floatPtr(0.10),
				ResetInSeconds:    int64Ptr(3600),
			},
			"3p-weekly": {
				RemainingFraction: floatPtr(0.20),
				ResetInSeconds:    int64Ptr(86400),
			},
		},
	}

	t.Run("default renders only current model series with short labels", func(t *testing.T) {
		got := Render(p, Options{Width: 160, NoColor: true})
		for _, want := range []string{"5h █▒······ 20% (2h)", "7d ███····· 40% (2d)"} {
			if !strings.Contains(got, want) {
				t.Fatalf("Render() = %q, missing %q", got, want)
			}
		}
		for _, omit := range []string{"90%", "80%", "3p-5h", "3p-weekly"} {
			if strings.Contains(got, omit) {
				t.Fatalf("Render() = %q, should not contain %q", got, omit)
			}
		}
	})

	t.Run("all-quotas renders all series and disambiguates colliding labels", func(t *testing.T) {
		got := Render(p, Options{Width: 220, NoColor: true, AllQuotas: true})
		for _, want := range []string{
			"3p-5h ███████· 90% (1h)",
			"gemini-5h █▒······ 20% (2h)",
			"3p-weekly ██████░· 80% (1d)",
			"gemini-weekly ███····· 40% (2d)",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("Render(AllQuotas:true) = %q, missing %q", got, want)
			}
		}
	})
}
