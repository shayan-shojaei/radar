package main

import (
	"fmt"
	"net/url"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shayan-shojaei/radar/internal/config"
	"github.com/shayan-shojaei/radar/internal/openapi"
	"github.com/shayan-shojaei/radar/internal/specsstore"
	"github.com/shayan-shojaei/radar/internal/tui"
	"github.com/shayan-shojaei/radar/internal/updater"
	"github.com/spf13/cobra"
)

// Injected at build time via -ldflags.
var (
	version = "dev"
	goos    = "unknown"
	goarch  = "unknown"
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
			// Fall back to the spec URL's origin when the spec defines no servers.
			if baseURL == "" {
				if u, err := url.Parse(specURL); err == nil && u.Host != "" {
					baseURL = u.Scheme + "://" + u.Host
				}
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
	cmd.AddCommand(versionCmd())
	cmd.AddCommand(updateCmd())
	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the current radar version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("radar %s (%s/%s)\n", version, goos, goarch)
		},
	}
}

func updateCmd() *cobra.Command {
	var checkOnly bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update radar to the latest release",
		RunE: func(cmd *cobra.Command, args []string) error {
			latest, err := updater.LatestVersion()
			if err != nil {
				return fmt.Errorf("fetch latest version: %w", err)
			}

			if !updater.IsNewer(latest, version) {
				fmt.Printf("Already up to date (%s)\n", version)
				return nil
			}

			fmt.Printf("New version available: %s (current: %s)\n", latest, version)

			if checkOnly {
				os.Exit(1) // non-zero signals "update available" for scripting
			}

			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("locate current binary: %w", err)
			}

			fmt.Printf("Downloading radar %s (%s/%s)...\n", latest, goos, goarch)
			if err := updater.Replace(exe, latest, goos, goarch); err != nil {
				return fmt.Errorf("update: %w", err)
			}

			fmt.Printf("Updated to %s — restart radar to use the new version.\n", latest)
			return nil
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "Only check for updates, do not download")
	return cmd
}
