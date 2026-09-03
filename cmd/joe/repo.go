package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/ai-toolkit/tools"
)

var repoNameTool = llm.Tool{
	Name: "repo_name",
	Description: "Returns the name of the repository the agent is working on: " +
		"the name of the git repository's root directory, or the name of the " +
		"current directory when it is not inside a git repository.",
	Schema: tools.NewObjectBuilder().Build(),
}

func repoName(ctx context.Context, _ map[string]any) (string, error) {
	if root, err := gitRoot(ctx); err == nil {
		return filepath.Base(root), nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return filepath.Base(cwd), nil
}

func gitRoot(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}
