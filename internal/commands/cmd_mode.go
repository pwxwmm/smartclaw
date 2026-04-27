package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/instructkr/smartclaw/internal/voice"
)

func init() {
	Register(Command{
		Name:    "fast",
		Summary: "Fast mode",
	}, fastHandler)

	Register(Command{
		Name:    "lazy",
		Summary: "Lazy mode",
	}, lazyHandler)

	Register(Command{
		Name:    "desktop",
		Summary: "Desktop mode",
	}, desktopHandler)

	Register(Command{
		Name:    "mobile",
		Summary: "Mobile mode",
	}, mobileHandler)

	Register(Command{
		Name:    "chrome",
		Summary: "Chrome integration",
	}, chromeHandler)

	Register(Command{
		Name:    "voice",
		Summary: "Voice mode control",
	}, voiceHandler)
}

func fastHandler(args []string) error {
	fmt.Println("Fast mode enabled")
	fmt.Println("  Using fastest available model")
	return nil
}

func lazyHandler(args []string) error {
	fmt.Println("Lazy mode enabled")
	fmt.Println("  Delays tool execution for batching")
	return nil
}

func desktopHandler(args []string) error {
	fmt.Println("Desktop mode")
	fmt.Println("⚠️  Desktop features not fully implemented")
	return nil
}

func mobileHandler(args []string) error {
	fmt.Println("Mobile mode")
	fmt.Println("  Optimized for mobile UI")
	return nil
}

func chromeHandler(args []string) error {
	fmt.Println("Chrome integration")
	fmt.Println("⚠️  Requires Chrome browser")
	return nil
}

func voiceHandler(args []string) error {
	if len(args) == 0 {
		return showVoiceStatus()
	}

	mode := args[0]
	switch mode {
	case "on":
		cmdCtx.VoiceManager.SetMode(voice.VoiceModeAlwaysOn)
		fmt.Println("✓ Voice mode enabled (always on)")
	case "off":
		cmdCtx.VoiceManager.SetMode(voice.VoiceModeDisabled)
		fmt.Println("✓ Voice mode disabled")
	case "ptt", "push-to-talk":
		cmdCtx.VoiceManager.SetMode(voice.VoiceModePushToTalk)
		fmt.Println("✓ Voice mode enabled (push-to-talk)")
		fmt.Println("  Press Space to start recording")
	case "keyterm":
		if len(args) > 1 {
			terms := args[1:]
			cmdCtx.VoiceManager.SetKeyterms(terms)
			fmt.Printf("✓ Keyterms set: %v\n", terms)
		}
	case "test":
		return testVoice()
	default:
		fmt.Printf("Unknown voice command: %s\n", mode)
		fmt.Println("\nUsage: /voice [on|off|ptt|keyterm <terms>|test]")
	}
	return nil
}

func showVoiceStatus() error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Voice Configuration        │")
	fmt.Println("└─────────────────────────────────────┘")

	config := cmdCtx.VoiceManager.GetConfig()
	modeStr := "disabled"
	switch config.Mode {
	case voice.VoiceModePushToTalk:
		modeStr = "push-to-talk"
	case voice.VoiceModeAlwaysOn:
		modeStr = "always-on"
	}

	fmt.Printf("  Mode:          %s\n", modeStr)
	fmt.Printf("  Language:      %s\n", config.Language)
	fmt.Printf("  Sample Rate:   %d Hz\n", config.SampleRate)
	fmt.Printf("  Model:         %s\n", config.Model)
	fmt.Printf("  Keyterms:      %v\n", cmdCtx.VoiceManager.GetKeyterms())
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  /voice on            - Enable always-on mode")
	fmt.Println("  /voice ptt          - Enable push-to-talk mode")
	fmt.Println("  /voice off          - Disable voice")
	fmt.Println("  /voice keyterm <w1> - Set keyterms")
	fmt.Println("  /voice test         - Test microphone")
	return nil
}

func testVoice() error {
	fmt.Println("Testing voice recording...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := cmdCtx.VoiceManager.StartPushToTalk(ctx)
	if err != nil {
		fmt.Printf("✗ Failed to start recording: %v\n", err)
		return nil
	}

	fmt.Println("Recording... (press Ctrl+C to stop)")
	time.Sleep(2 * time.Second)

	result, err := cmdCtx.VoiceManager.StopPushToTalk(ctx)
	if err != nil {
		fmt.Printf("✗ Recording failed: %v\n", err)
		return nil
	}

	fmt.Printf("✓ Recorded: %s\n", result.Text)
	return nil
}
