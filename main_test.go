package main

import (
	"encoding/csv"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStringSliceFlag(t *testing.T) {
	t.Parallel()

	var f stringSliceFlag
	if err := f.Set("/tmp/a"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := f.Set("/tmp/b"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	if got, want := f.String(), "/tmp/a,/tmp/b"; got != want {
		t.Fatalf("unexpected String(): got %q want %q", got, want)
	}
}

func TestGetGitInfoSuccess(t *testing.T) {
	t.Parallel()

	ensureGit(t)

	repoDir := t.TempDir()
	initGitRepoWithCommit(t, repoDir)
	runGit(t, repoDir, "remote", "add", "origin", "git@github.com:test/repo.git")

	remote, lastCommit, err := getGitInfo(repoDir)
	if err != nil {
		t.Fatalf("getGitInfo returned error: %v", err)
	}

	if remote != "git@github.com:test/repo.git" {
		t.Fatalf("unexpected remote: got %q", remote)
	}
	if lastCommit.IsZero() {
		t.Fatalf("last commit date must not be zero")
	}
}

func TestGetGitInfoNoRemote(t *testing.T) {
	t.Parallel()

	ensureGit(t)

	repoDir := t.TempDir()
	initGitRepoWithCommit(t, repoDir)

	remote, lastCommit, err := getGitInfo(repoDir)
	if err != nil {
		t.Fatalf("getGitInfo returned error: %v", err)
	}
	if remote != "" {
		t.Fatalf("expected empty remote, got %q", remote)
	}
	if lastCommit.IsZero() {
		t.Fatalf("last commit date must not be zero")
	}
}

func TestGetGitInfoNoCommit(t *testing.T) {
	t.Parallel()

	ensureGit(t)

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")

	remote, lastCommit, err := getGitInfo(repoDir)
	if err == nil {
		t.Fatalf("expected error for repo without commits")
	}
	if remote != "" {
		t.Fatalf("expected empty remote, got %q", remote)
	}
	if !lastCommit.IsZero() {
		t.Fatalf("expected zero lastCommit, got %v", lastCommit)
	}
}

func TestMainNoDirectoryFails(t *testing.T) {
	t.Parallel()

	binPath := buildBinary(t)
	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit when no --directory provided")
	}

	if !strings.Contains(string(output), "Please provide at least one --directory=/path") {
		t.Fatalf("unexpected output: %s", string(output))
	}
}

func TestMainGeneratesCSV(t *testing.T) {
	ensureGit(t)

	binPath := buildBinary(t)
	searchRoot := t.TempDir()
	workDir := t.TempDir()

	repoA := filepath.Join(searchRoot, "repo-a")
	repoB := filepath.Join(searchRoot, "repo-b")
	mustMkdir(t, repoA)
	mustMkdir(t, repoB)
	initGitRepoWithCommit(t, repoA)
	initGitRepoWithCommit(t, repoB)
	runGit(t, repoA, "remote", "add", "origin", "git@github.com:test/repo-a.git")
	runGit(t, repoB, "remote", "add", "origin", "git@github.com:test/repo-b.git")

	cmd := exec.Command(binPath, "--directory", searchRoot)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run failed: %v\n%s", err, string(output))
	}

	reportPath := filepath.Join(workDir, "git_projects_report.csv")
	rows := readCSV(t, reportPath)
	if len(rows) != 3 {
		t.Fatalf("unexpected row count: got %d want 3", len(rows))
	}

	wantHeader := []string{"Project name", "Path", "Remote repository", "Last commit date"}
	if strings.Join(rows[0], "|") != strings.Join(wantHeader, "|") {
		t.Fatalf("unexpected csv header: %v", rows[0])
	}

	seen := map[string]bool{}
	for _, row := range rows[1:] {
		seen[row[0]] = true
		if row[1] == "" || row[2] == "" || row[3] == "" {
			t.Fatalf("csv row has empty required fields: %v", row)
		}
		if _, err := time.Parse("2006-01-02 15:04:05", row[3]); err != nil {
			t.Fatalf("unexpected date format %q: %v", row[3], err)
		}
	}

	if !seen["repo-a"] || !seen["repo-b"] {
		t.Fatalf("expected repo-a and repo-b in csv rows, got: %v", rows[1:])
	}
}

func ensureGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available in PATH")
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	return wd
}

func buildBinary(t *testing.T) string {
	t.Helper()

	moduleDir := mustGetwd(t)
	binPath := filepath.Join(t.TempDir(), "go-git-searcher-test-bin")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = moduleDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, string(output))
	}

	return binPath
}

func initGitRepoWithCommit(t *testing.T, dir string) {
	t.Helper()

	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatalf("failed to create file in repo: %v", err)
	}

	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "initial commit")
}

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open csv: %v", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("failed to read csv rows: %v", err)
	}
	return rows
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create directory %s: %v", dir, err)
	}
}
