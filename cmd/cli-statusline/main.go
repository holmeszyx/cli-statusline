package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"cli-statusline/internal/statusline"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cli-statusline", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		width     int
		barWidth  int
		noColor   bool
		quotaMode string
		demo      bool
	)
	fs.IntVar(&width, "width", 0, "override terminal width (0 uses payload terminal_width)")
	fs.IntVar(&barWidth, "bar-width", 8, "progress bar character width")
	fs.BoolVar(&noColor, "no-color", false, "disable ANSI colors (also respects NO_COLOR)")
	fs.StringVar(&quotaMode, "quota-mode", "used", "quota display mode: used or remaining")
	fs.BoolVar(&demo, "demo", false, "render a sample payload for preview")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if os.Getenv("NO_COLOR") != "" {
		noColor = true
	}

	var payload statusline.Payload
	if demo {
		payload = samplePayload()
	} else {
		raw, err := io.ReadAll(stdin)
		if err == nil && len(bytes.TrimSpace(raw)) > 0 {
			_ = json.Unmarshal(raw, &payload)
		}
	}

	mode := statusline.QuotaModeUsed
	if strings.EqualFold(strings.TrimSpace(quotaMode), string(statusline.QuotaModeRemaining)) {
		mode = statusline.QuotaModeRemaining
	}

	out := statusline.Render(payload, statusline.Options{
		Width:     width,
		BarWidth:  barWidth,
		NoColor:   noColor,
		QuotaMode: mode,
	})
	if out != "" {
		_, _ = fmt.Fprintln(stdout, out)
	}
	return 0
}

func samplePayload() statusline.Payload {
	usedPct := 34.6
	remFrac := 0.72
	resetSec := int64(560580)
	return statusline.Payload{
		CWD: "/home/user/cli-statusline",
		Workspace: &statusline.WorkspaceInfo{
			CurrentDir: "/home/user/cli-statusline",
			ProjectDir: "/home/user/cli-statusline",
		},
		Model: &statusline.ModelInfo{
			ID:          "Gemini 4 Argon (Medium)",
			DisplayName: "Gemini 4 Argon (Medium)",
			Effort:      "medium",
		},
		ContextWindow: &statusline.ContextWindowInfo{
			ContextWindowSize: 1048576,
			UsedPercentage:    &usedPct,
		},
		Quota: map[string]statusline.QuotaBucket{
			"gemini-weekly": {
				RemainingFraction: &remFrac,
				ResetInSeconds:    &resetSec,
			},
		},
		AgentState:    "idle",
		VCS:           &statusline.VCSInfo{Type: "git", Branch: "main"},
		CycleMode:     "plan",
		ArtifactCount: 2,
		TerminalWidth: 120,
	}
}
