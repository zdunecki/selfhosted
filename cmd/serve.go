package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zdunecki/selfhosted/pkg/server"
)

var (
	servePort      int
	serveNoBrowser bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the SelfHosted web UI (API + frontend)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return server.StartWithOptions(servePort, !serveNoBrowser)
	},
}

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 8080, "HTTP port to listen on")
	serveCmd.Flags().BoolVar(&serveNoBrowser, "no-browser", true, "Do not open the system browser")
	rootCmd.AddCommand(serveCmd)
}

