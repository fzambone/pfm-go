package main

import "fmt"

// Version information injected at build via ldflags.
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	fmt.Printf("PFM-Go v%s (commit: %s, built: %s)\n", Version, GitCommit, BuildTime)
}
