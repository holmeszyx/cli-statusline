package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		stdin    string
		wantCode int
		wantSub  string
	}{
		{
			name:     "demo mode plain",
			args:     []string{"-demo", "-no-color"},
			wantCode: 0,
			wantSub:  "dir cli-statusline",
		},
		{
			name:     "valid stdin json",
			args:     []string{"-no-color", "-width", "120"},
			stdin:    `{"cwd":"/tmp/demo-app","cycle_mode":"auto","model":{"display_name":"Gemini 3 Pro","effort":"high"},"context_window":{"used_percentage":25}}`,
			wantCode: 0,
			wantSub:  "✦ auto · Gemini 3 Pro · high │ dir demo-app │ ctx ██······ 25%",
		},
		{
			name:     "malformed stdin degrades gracefully",
			args:     []string{"-no-color"},
			stdin:    `{bad json`,
			wantCode: 0,
			wantSub:  "▸ default",
		},
		{
			name:     "remaining quota mode",
			args:     []string{"-demo", "-no-color", "-quota-mode", "remaining"},
			wantCode: 0,
			wantSub:  "quota █████▓·· 72%",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out, errBuf bytes.Buffer
			code := run(tc.args, strings.NewReader(tc.stdin), &out, &errBuf)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr=%q)", code, tc.wantCode, errBuf.String())
			}
			if !strings.Contains(out.String(), tc.wantSub) {
				t.Fatalf("stdout = %q, want substring %q", out.String(), tc.wantSub)
			}
		})
	}
}
