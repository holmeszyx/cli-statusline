package statusline

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func detectGitBranch(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	cur := filepath.Clean(dir)
	for {
		gitPath := filepath.Join(cur, ".git")
		info, err := os.Stat(gitPath)
		if err == nil {
			gitDir := gitPath
			if !info.IsDir() {
				gitDir = resolveGitDirFile(cur, gitPath)
			}
			if gitDir != "" {
				if branch := readGitHeadBranch(gitDir, dir); branch != "" {
					return branch
				}
			}
			return ""
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
}

func resolveGitDirFile(baseDir, gitFile string) string {
	raw, err := os.ReadFile(gitFile)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(raw))
	const prefix = "gitdir:"
	if !strings.HasPrefix(strings.ToLower(line), prefix) {
		return ""
	}
	target := strings.TrimSpace(line[len(prefix):])
	if target == "" {
		return ""
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(baseDir, target)
	}
	return filepath.Clean(target)
}

func readGitHeadBranch(gitDir, workDir string) string {
	raw, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(raw))
	if line == "" {
		return ""
	}
	if strings.HasPrefix(line, "ref:") {
		ref := strings.TrimSpace(strings.TrimPrefix(line, "ref:"))
		if ref == "refs/heads/.invalid" {
			return gitBranchFromCLI(workDir)
		}
		if strings.HasPrefix(ref, "refs/heads/") {
			return strings.TrimPrefix(ref, "refs/heads/")
		}
		if strings.HasPrefix(ref, "refs/tags/") {
			return strings.TrimPrefix(ref, "refs/tags/")
		}
		return ref
	}
	if isHexHash(line) {
		if len(line) > 7 {
			return line[:7]
		}
		return line
	}
	return ""
}

func isHexHash(s string) bool {
	if len(s) < 7 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !isHex {
			return false
		}
	}
	return true
}

func gitBranchFromCLI(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" || branch == "HEAD" {
		return ""
	}
	return branch
}
