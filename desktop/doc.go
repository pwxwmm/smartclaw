//go:build desktop
// +build desktop

// Package desktop provides a Wails-based desktop application for SmartClaw.
//
// # Build Tag Requirement
//
// This package uses the "desktop" build tag to isolate Wails dependencies
// (which require CGO and platform-specific tooling) from the main build.
// Without the tag, these files are excluded from compilation.
//
// To build the desktop application:
//
//	go build -tags desktop -o smartclaw-desktop ./desktop/
//
// To run the normal CLI without desktop support:
//
//	go build -o smartclaw ./cmd/smartclaw/
//
// # Prerequisites
//
// Building with the "desktop" tag requires:
//   - Wails CLI v2: go install github.com/wailsapp/wails/v2/cmd/wails@latest
//   - CGO enabled (CC compiler available)
//   - Platform-specific WebView2 (Windows), WebKit (macOS/Linux)
//
// # Architecture
//
// The desktop package connects the Wails frontend to the SmartClaw engine
// through Go backend bindings (see app.go). The frontend reuses the existing
// Web UI static files from internal/web/static/ via Wails' asset handler.
package desktop
