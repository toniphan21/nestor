//go:build noselfupdate

package main

import "github.com/spf13/cobra"

const upgradeCommand = "brew upgrade nestor"

// selfUpdateCmd returns nil in builds (e.g. the Homebrew formula) where
// updates are managed by another package manager.
func selfUpdateCmd() *cobra.Command { return nil }
