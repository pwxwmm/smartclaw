package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/hooks"
	"github.com/instructkr/smartclaw/internal/runtime"
	"github.com/instructkr/smartclaw/internal/tools"
)

var promptCmd = &cobra.Command{
	Use:   "prompt [text...]",
	Short: "Send a single prompt and exit",
	Long: `Send a single prompt to Claude and stream the response.

Examples:
  smartclaw prompt "Explain this code"
  smartclaw prompt --model claude-sonnet-4-5 "Write a function"
  smartclaw prompt --json "List files" > output.json`,
	Args: cobra.MinimumNArgs(1),
	Run:  runPrompt,
}

func init() {
	rootCmd.AddCommand(promptCmd)
	promptCmd.Flags().Bool("stream", true, "Stream response")
	promptCmd.Flags().Bool("tools", true, "Enable tool use")
}

func runPrompt(cmd *cobra.Command, args []string) {
	prompt := strings.Join(args, " ")
	model := viper.GetString("model")
	useJSON := viper.GetBool("json")
	enableTools, _ := cmd.Flags().GetBool("tools")
	isOpenAI := viper.GetBool("openai")

	cfg, err := getClientConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if !useJSON {
		fmt.Printf("Model: %s\n", model)
		fmt.Printf("Prompt: %s\n\n", prompt)
		fmt.Println("--- Response ---")
	}

	client := api.NewClientWithModel(cfg.APIKey, cfg.BaseURL, cfg.Model)
	client.SetOpenAI(isOpenAI)

	hookManager := createHookManager()
	hookManager.LoadConfig()

	engine := runtime.NewQueryEngineWithHooks(client, runtime.QueryConfig{
		Model:        cfg.Model,
		MaxTokens:    cfg.MaxTokens,
		SystemPrompt: cfg.SystemPrompt,
	}, hookManager)

	if enableTools {
		prepareTools(engine)
	}

	result, err := engine.Query(context.Background(), prompt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if useJSON {
		outputJSON(result)
	} else {
		outputText(result)
	}

	if !useJSON {
		fmt.Printf("\n--- Stats ---\n")
		fmt.Printf("Input tokens: %d\n", result.Usage.InputTokens)
		fmt.Printf("Output tokens: %d\n", result.Usage.OutputTokens)
		fmt.Printf("Cost: $%.6f\n", result.Cost)
	}
}

func prepareTools(engine *runtime.QueryEngine) {
	registry := tools.GetRegistry()
	for _, tool := range registry.All() {
		engine.AddTool(tool)
	}
}

func outputText(result *runtime.QueryResult) {
	switch content := result.Message.Content.(type) {
	case string:
		fmt.Println(content)
	default:
		fmt.Printf("%v\n", content)
	}
}

func outputJSON(result *runtime.QueryResult) {
	output := map[string]any{
		"content":       result.Message.Content,
		"input_tokens":  result.Usage.InputTokens,
		"output_tokens": result.Usage.OutputTokens,
		"cost":          result.Cost,
		"stop_reason":   result.StopReason,
	}

	jsonOutput, err := jsonMarshal(output)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error encoding JSON:", err)
		os.Exit(1)
	}
	fmt.Println(string(jsonOutput))
}

func jsonMarshal(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

func createHookManagerForPrompt() *hooks.HookManager {
	workDir, _ := os.Getwd()
	return hooks.NewHookManager(workDir, "prompt-session")
}
