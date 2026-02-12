package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "subscriptions",
	Short: "Subscriptions microservice",
	Long:  "A subscriptions microservice for managing plan and email subscriptions over HTTP and gRPC.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
