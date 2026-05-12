package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shayan-shojaei/radar/internal/config"
	"github.com/shayan-shojaei/radar/internal/openapi"
	"github.com/shayan-shojaei/radar/internal/specsstore"
	"github.com/shayan-shojaei/radar/internal/tui"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var specURL string

	cmd := &cobra.Command{
		Use:   "radar [spec-url]",
		Short: "An interactive terminal API explorer for OpenAPI / Swagger specs",
		Long: `radar fetches an OpenAPI or Swagger specification and launches an
interactive TUI where you can browse endpoints, craft requests,
and inspect responses. Session data is saved locally using age encryption.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				specURL = args[0]
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// F5: if no URL provided, launch the spec picker.
			if specURL == "" {
				saved, _ := specsstore.Load(cfg.StorageDir)
				recent, _ := specsstore.LoadRecent(cfg.StorageDir)
				chosen, err := tui.RunSpecPicker(saved, recent, cfg.StorageDir)
				if err != nil {
					return fmt.Errorf("spec picker: %w", err)
				}
				if chosen == "" {
					return nil // user quit without selecting
				}
				specURL = chosen
			}

			fmt.Fprintf(os.Stderr, "Fetching spec from %s...\n", specURL)
			endpoints, baseURL, err := openapi.Parse(specURL)
			if err != nil {
				return fmt.Errorf("parse spec: %w", err)
			}
			if len(endpoints) == 0 {
				return fmt.Errorf("no endpoints found in spec")
			}
			fmt.Fprintf(os.Stderr, "Loaded %d endpoints\n", len(endpoints))

			// Record this URL in the recent list.
			specsstore.AddRecent(cfg.StorageDir, specURL) //nolint:errcheck

			passphrase := os.Getenv("RADAR_PASSPHRASE")
			model := tui.New(endpoints, baseURL, specURL, cfg, passphrase)
			p := tea.NewProgram(model, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("TUI error: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&specURL, "url", "", "URL or file path to the OpenAPI / Swagger spec")
	return cmd
}
