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
