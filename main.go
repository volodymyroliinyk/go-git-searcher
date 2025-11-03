package main

import (
    "context"
    "encoding/csv"
    "errors"
    "flag"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "sort"
    "strings"
    "time"
)

type GitProject struct {
    Path           string
    ProjectName    string
    RemoteRepo     string
    LastCommitDate time.Time
}

// Add this helper to force output
func printAndFlush(s string) {
    fmt.Print(s)

    // os.Stdout is of type *os.File and already supports the Sync() method
    // Therefore, type assertion is not needed.
    if err := os.Stdout.Sync(); err != nil {
        // Synchronization error handling is desirable, but often ignored
        // fmt.Printf("Error syncing stdout: %v\n", err)
    }
}

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
    return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(value string) error {
    *s = append(*s, value)
    return nil
}

func main() {
    var directories stringSliceFlag
    flag.Var(&directories, "directory", "Path to a directory to search (can be repeated)")

    flag.Parse()

    if len(directories) == 0 {
        fmt.Println("❌ Please provide at least one --directory=/path")
        os.Exit(1)
    }

    var gitProjects []GitProject

    for _, rootDir := range directories {
        rootDir = strings.TrimSpace(rootDir)
        fmt.Printf("🔍 Scanning: %s\n", rootDir)

        err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
            if info != nil && info.IsDir() {
                printAndFlush(fmt.Sprintf("Entering: %s\n", path))
            }

            if err != nil {
                // Log the error (for example, "permission denied") and return nil
                // to continue traversing other parts of the tree.
                fmt.Printf("🚫 Skipped due to error in %s: %v\n", path, err)
                return nil // We continue the detour
            }
            if info.IsDir() && strings.HasSuffix(path, "/.git") {
                printAndFlush(path)
                projectPath := filepath.Dir(path)
                projectName := filepath.Base(projectPath)
                remoteRepo, lastCommitDate, err := getGitInfo(projectPath)
                if err != nil {
                    fmt.Printf("!❌ [%s] Skipping project due to error: %v\n", projectPath, err) // Mark the gap
                    return nil
                }
                gitProjects = append(gitProjects, GitProject{
                    Path:           projectPath,
                    ProjectName:    projectName,
                    RemoteRepo:     remoteRepo,
                    LastCommitDate: lastCommitDate,
                })

                printAndFlush("+")
            } else if info.IsDir() {
                printAndFlush(path)
                printAndFlush(".")
                return nil
            }
            return nil
        })

        if err != nil {
            fmt.Printf("🚫 Error while scanning '%s': %v\n", rootDir, err)
            continue
        }
    }

    // Sorting
    sort.Slice(gitProjects, func(i, j int) bool {
        if gitProjects[i].RemoteRepo != "" && gitProjects[j].RemoteRepo != "" {
            if gitProjects[i].RemoteRepo == gitProjects[j].RemoteRepo {
                return gitProjects[i].LastCommitDate.After(gitProjects[j].LastCommitDate)
            }

            return gitProjects[i].RemoteRepo < gitProjects[j].RemoteRepo
        }

        return gitProjects[i].ProjectName < gitProjects[j].ProjectName
    })

    // Create CSV
    csvFile, err := os.Create("git_projects_report.csv")
    if err != nil {
        fmt.Printf("❌ Failed to create CSV file: %v\n", err)

        return
    }
    defer csvFile.Close()

    writer := csv.NewWriter(csvFile)
    defer writer.Flush()

    writer.Write([]string{"Project name", "Path", "Remote repository", "Last commit date"})

    for _, project := range gitProjects {
        writer.Write([]string{
            project.ProjectName,
            project.Path,
            project.RemoteRepo,
            project.LastCommitDate.Format("2006-01-02 15:04:05"),
        })
    }

    fmt.Println("✅ Report saved to 'git_projects_report.csv'")

    return
}

func getGitInfo(projectPath string) (string, time.Time, error) {
    // Set a timeout of 10 seconds for git operations
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    var devNull *os.File
    // Try to open /dev/null once
    // Important: devNull open error handling is separate from git errors
    if dn, err := os.Open(os.DevNull); err == nil {
        devNull = dn
        defer devNull.Close()
    }

    // --- 1. Getting Remote Repo ---
    cmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
    cmd.Dir = projectPath
    // If we managed to open /dev/null, redirect stderr there
    if devNull != nil {
        cmd.Stderr = devNull
    }

    remoteRepoBytes, err := cmd.Output()
    remoteRepo := ""

    if err != nil {
        // Check the timeout
        if errors.Is(ctx.Err(), context.DeadlineExceeded) {
            return "", time.Time{}, fmt.Errorf("Git remote operation timed out after 10s")
        }
        // Log in, but continue
        fmt.Printf("⚠️ [%s] Failed to get remote repo: %v\n", projectPath, err)
    } else {
        remoteRepo = strings.TrimSpace(string(remoteRepoBytes))
    }

    // --- 2. Get the date of the last commit ---
    cmd = exec.CommandContext(ctx, "git", "log", "-1", "--format=%cd", "--date=iso")
    cmd.Dir = projectPath
    if devNull != nil {
        cmd.Stderr = devNull
    }

    lastCommitBytes, err := cmd.Output()
    if err != nil {
        // Check if a timeout was the cause
        if errors.Is(ctx.Err(), context.DeadlineExceeded) {
            return remoteRepo, time.Time{}, fmt.Errorf("Git log operation timed out after 10s")
        }
        return remoteRepo, time.Time{}, fmt.Errorf("Failed to get last commit date: %w", err)
    }

    lastCommitDateStr := strings.TrimSpace(string(lastCommitBytes))
    // Fix: git log --date=iso outputs "2024-05-15 15:00:00 +0300",
    // your format is correct.
    lastCommitDate, err := time.Parse("2006-01-02 15:04:05 -0700", lastCommitDateStr)
    if err != nil {
        return remoteRepo, time.Time{}, fmt.Errorf("Failed to parse commit date '%s': %v", lastCommitDateStr, err)
    }

    return remoteRepo, lastCommitDate, nil
}
