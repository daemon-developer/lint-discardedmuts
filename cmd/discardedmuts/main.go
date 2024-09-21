package main

import (
	"github.com/daemon-developer/lint-discardedmuts/pkg/discardedmuts" // Update with actual import path
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(discardedmuts.DiscardedModificationAnalyzer)
}
