# Contributing to PFM-Go

## Development Environment Setup

### IDE Configuration

This project follows the **Google Go Style Guide**. Configure your IDE accordingly.

#### GoLand / IntelliJ IDEA

1. **Go Style**:
    - Settings → Editor → Code Style → Go
    - Import scheme: Google Go Style Guide

2. **goimports on save**:
    - Settings → Tools → File Watchers
    - Add goimports with scope: Project Files

3. **golangci-lint**:
    - Settings → Tools → External Tools
    - Add golangci-lint with: `golangci-lint run $FileDir$`

4. **File Templates**:
    - Settings → Editor → File and Code Templates
    - Add test file template with table-driven structure

#### VS Code

Add to `.vscode/settings.json` (not committed):

  ```json
  {
    "go.formatTool": "goimports",
    "go.lintTool": "golangci-lint",
    "go.lintOnSave": "workspace",
    "editor.formatOnSave": true,
    "go.useLanguageServer": true
  }

  Required Tools

  Install these tools globally:
  go install golang.org/x/tools/cmd/goimports@latest
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

  Code Style Rules

  - Tabs for indentation (not spaces)
  - gofmt format (automatic with goimports)
  - No line length limit (but be reasonable)
  - Imports order: stdlib, external, internal
  - See CLAUDE.md for complete style guide

  Development Workflow

  1. Pick an issue from GitHub
  2. Create a branch (optional, we use trunk-based development)
  3. Write tests first (TDD)
  4. Implement the feature
  5. Run: go test ./... -race -count=1
  6. Run: golangci-lint run ./...
  7. Commit with conventional format: feat(scope): description
  8. Push and issue closes automatically

  Testing

  - Unit tests: go test ./...
  - With race detector: go test ./... -race
  - Integration tests: go test -tags=integration ./...
  - Coverage: go test -cover ./...
