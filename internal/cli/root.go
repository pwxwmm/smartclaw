package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/auth"
	"github.com/instructkr/smartclaw/internal/hooks"
	"github.com/instructkr/smartclaw/internal/logger"
	"github.com/instructkr/smartclaw/internal/tui"
	"github.com/instructkr/smartclaw/internal/web"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "smartclaw [command] [args]",
	Short: "SmartClaw - AI-powered coding assistant",
	Long: `SmartClaw is a high-performance Go rewrite of Claude Code CLI.

Smart coding with AI-powered assistance:
  • 57+ built-in tools (file ops, code analysis, web, etc.)
  • 101 slash commands for productivity
  • Session persistence and management
  • MCP protocol integration
  • Skills and hooks system
  • Voice mode support
  • Permission modes for security

Examples:
  smartclaw repl              Start interactive REPL
  smartclaw prompt "Explain this code"   Send single prompt
  smartclaw --model claude-opus-4-6 repl Use specific model
  smartclaw /help             Show available commands in REPL`,
	Version: Version,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file (default is $HOME/.smartclaw/config.yaml)")
	rootCmd.PersistentFlags().StringP("model", "m", "claude-sonnet-4-5", "Model to use (smartclaw-sonnet-4-5, smartclaw-opus-4, smartclaw-haiku-3-5)")
	rootCmd.PersistentFlags().StringP("permission", "p", "ask", "Permission mode (ask, workspace-write, danger-full-access)")
	rootCmd.PersistentFlags().Bool("dangerously-skip-permissions", false, "Skip all permission checks (dangerous)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().Bool("json", false, "Output in JSON format")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().Int("max-tokens", 4096, "Maximum tokens for response")
	rootCmd.PersistentFlags().String("api-key", "", "Anthropic API key (or set ANTHROPIC_API_KEY)")
	rootCmd.PersistentFlags().String("url", "", "API base URL (for custom API endpoints)")
	rootCmd.PersistentFlags().Bool("openai", false, "Use OpenAI-compatible API format")
	rootCmd.PersistentFlags().Bool("show-thinking", true, "Show thinking/reasoning content from models like GLM-5")
	rootCmd.PersistentFlags().String("session", "", "Resume session by ID")
	rootCmd.PersistentFlags().String("system-prompt", "", "Custom system prompt")

	rootCmd.AddCommand(&cobra.Command{
		Use:   "test-colors",
		Short: "Test ANSI color rendering in TUI",
		Run: func(cmd *cobra.Command, args []string) {
			if err := tui.RunColorTestTUI(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "test-runtime",
		Short: "Test glamour at runtime",
		Run: func(cmd *cobra.Command, args []string) {
			if err := tui.RunColorTestTUI6(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "test-whitespace",
		Short: "Test ANSI with whitespace",
		Run: func(cmd *cobra.Command, args []string) {
			if err := tui.RunColorTestTUI5(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "test-markdown",
		Short: "Test markdown rendering with syntax highlighting",
		Run: func(cmd *cobra.Command, args []string) {
			if err := tui.RunColorTestTUI2(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "test-println",
		Short: "Test tea.Println for ANSI rendering",
		Run: func(cmd *cobra.Command, args []string) {
			if err := tui.RunColorTestTUI3(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "test-debug",
		Short: "Debug ANSI rendering in TUI",
		Run: func(cmd *cobra.Command, args []string) {
			if err := tui.RunColorTestTUI4(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	})

	var webPort int
	webCmd := &cobra.Command{
		Use:   "web",
		Short: "Start SmartClaw WebUI server",
		Long:  "Start the web-based interface for SmartClaw with streaming chat, file browser, code editor, dashboard, and voice input.",
		Run: func(cmd *cobra.Command, args []string) {
			apiKey := viper.GetString("api_key")
			if apiKey == "" {
				apiKey = os.Getenv("ANTHROPIC_API_KEY")
			}

			model := viper.GetString("model")
			baseURL := viper.GetString("base_url")

			var apiClient *api.Client
			if apiKey != "" {
				apiClient = api.NewClientWithModel(apiKey, baseURL, model)
				if viper.GetBool("openai") {
					apiClient.SetOpenAI(true)
				}
			}

			workDir, _ := os.Getwd()
			server := web.NewWebServer(webPort, workDir, apiClient)
			if err := server.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
	webCmd.Flags().IntVar(&webPort, "port", 8080, "WebUI server port")
	rootCmd.AddCommand(webCmd)

	viper.BindPFlag("model", rootCmd.PersistentFlags().Lookup("model"))
	viper.BindPFlag("permission", rootCmd.PersistentFlags().Lookup("permission"))
	viper.BindPFlag("dangerously_skip_permissions", rootCmd.PersistentFlags().Lookup("dangerously-skip-permissions"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json"))
	viper.BindPFlag("no_color", rootCmd.PersistentFlags().Lookup("no-color"))
	viper.BindPFlag("max_tokens", rootCmd.PersistentFlags().Lookup("max-tokens"))
	viper.BindPFlag("api_key", rootCmd.PersistentFlags().Lookup("api-key"))
	viper.BindPFlag("url", rootCmd.PersistentFlags().Lookup("url"))
	viper.BindPFlag("openai", rootCmd.PersistentFlags().Lookup("openai"))
	viper.BindPFlag("show_thinking", rootCmd.PersistentFlags().Lookup("show-thinking"))
	viper.BindPFlag("session", rootCmd.PersistentFlags().Lookup("session"))
	viper.BindPFlag("system_prompt", rootCmd.PersistentFlags().Lookup("system-prompt"))

	rootCmd.SetVersionTemplate(fmt.Sprintf("SmartClaw %s (commit: %s, built: %s)\n", Version, Commit, Date))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		viper.AddConfigPath(home + "/.smartclaw")
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
	}

	viper.SetEnvPrefix("SMART")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if viper.GetBool("verbose") {
			fmt.Fprintln(os.Stderr, "Debug: config not found, using defaults")
		}
	}

	initLogger()
	initAPIKey()
}

func initLogger() {
	logLevel := logger.LevelInfo
	if viper.GetBool("verbose") {
		logLevel = logger.LevelDebug
	}
	logger.SetLevel(logLevel)
}

func initAPIKey() {
	apiKey := viper.GetString("api_key")
	if apiKey != "" {
		auth.SetAPIKey(apiKey)
	}
}

func getModel() string {
	model := viper.GetString("model")
	if model == "" {
		model = "claude-opus-4-6"
	}
	return model
}

func getPermissionMode() string {
	mode := viper.GetString("permission")
	if mode == "" {
		if viper.GetBool("dangerously_skip_permissions") {
			return "danger-full-access"
		}
		return "ask"
	}
	return mode
}

type ClientConfig struct {
	APIKey       string
	Model        string
	MaxTokens    int
	SystemPrompt string
	BaseURL      string
	IsOpenAI     bool
}

func getClientConfig() (*ClientConfig, error) {
	apiKey := viper.GetString("api_key")
	if apiKey == "" {
		apiKey = auth.GetAPIKey()
	}

	baseURL := viper.GetString("url")
	if baseURL == "" {
		baseURL = auth.GetBaseURL()
	}

	if apiKey == "" {
		return nil, fmt.Errorf("no API key set. Set ANTHROPIC_API_KEY environment variable or use --api-key flag")
	}

	return &ClientConfig{
		APIKey:       apiKey,
		BaseURL:      baseURL,
		Model:        getModel(),
		MaxTokens:    viper.GetInt("max_tokens"),
		SystemPrompt: viper.GetString("system_prompt"),
		IsOpenAI:     viper.GetBool("openai"),
	}, nil
}

func createHookManager() *hooks.HookManager {
	workDir, _ := os.Getwd()
	return hooks.NewHookManager(workDir, "default-session")
}
