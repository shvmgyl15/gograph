package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// BuildBaselineGraphFromGitRef extracts a repository state at the given git ref
// into a temporary directory and builds a graph for it.
func BuildBaselineGraphFromGitRef(baselineRef string, buildGraph func(string) (*graph.Graph, error)) (*graph.Graph, error) {
	if strings.HasSuffix(baselineRef, ".json") {
		data, err := os.ReadFile(baselineRef)
		if err != nil {
			return nil, fmt.Errorf("error loading baseline graph %s: %w", baselineRef, err)
		}
		var g graph.Graph
		if err := json.Unmarshal(data, &g); err != nil {
			return nil, fmt.Errorf("error parsing baseline graph %s: %w", baselineRef, err)
		}
		return &g, nil
	}

	// Validate ref safely
	validRef := regexp.MustCompile(`^[a-zA-Z0-9_\-\.\/\~]+$`)
	if !validRef.MatchString(baselineRef) || strings.HasPrefix(baselineRef, "-") {
		return nil, fmt.Errorf("invalid or unsafe baseline ref: %q", baselineRef)
	}

	// Determine repository root to run git archive safely
	repoRootOut, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to determine repository root: %w", err)
	}
	repoRoot := strings.TrimSpace(string(repoRootOut))

	tmpDir, err := os.MkdirTemp("", "gograph-baseline-*")
	if err != nil {
		return nil, fmt.Errorf("error creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Execute git archive explicitly for the whole repository. BuildGraph filters files automatically.
	gitArgs := []string{"archive", "--format=tar", baselineRef}
	gitCmd := exec.Command("git", gitArgs...)
	gitCmd.Dir = repoRoot

	var gitStderr bytes.Buffer
	gitCmd.Stderr = &gitStderr

	tarCmd := exec.Command("tar", "-x", "-C", tmpDir)

	pr, pw := io.Pipe()
	gitCmd.Stdout = pw
	tarCmd.Stdin = pr

	if err := gitCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start git archive: %w", err)
	}
	if err := tarCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start tar: %w", err)
	}

	// Wait for git archive and close writer pipe so tar knows EOF
	gitErr := gitCmd.Wait()
	pw.Close()

	tarErr := tarCmd.Wait()

	if gitErr != nil {
		return nil, fmt.Errorf("for ref %q: git archive failed: %v: %s", baselineRef, gitErr, strings.TrimSpace(gitStderr.String()))
	}
	if tarErr != nil {
		return nil, fmt.Errorf("tar extraction failed: %w", tarErr)
	}

	return buildGraph(tmpDir)
}
