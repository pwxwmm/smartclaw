package tools

import (
	"context"
	"encoding/base64"
	"io/ioutil"
	"path/filepath"
	"strings"
)

type ImageTool struct{}

func (t *ImageTool) Name() string        { return "image" }
func (t *ImageTool) Description() string { return "Process and analyze images" }

func (t *ImageTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type": "string",
				"enum": []string{"read", "encode", "analyze", "resize", "convert"},
			},
			"path": map[string]interface{}{"type": "string"},
			"format": map[string]interface{}{
				"type": "string",
				"enum": []string{"base64", "data_url", "bytes"},
			},
		},
		"required": []string{"operation", "path"},
	}
}

func (t *ImageTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	operation, _ := input["operation"].(string)
	path, _ := input["path"].(string)

	if path == "" {
		return nil, ErrRequiredField("path")
	}

	switch operation {
	case "read":
		return t.readImage(path)
	case "encode":
		format, _ := input["format"].(string)
		return t.encodeImage(path, format)
	case "analyze":
		return t.analyzeImage(path)
	default:
		return nil, &Error{Code: "INVALID_OPERATION", Message: operation}
	}
}

func (t *ImageTool) readImage(path string) (interface{}, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, &Error{Code: "READ_ERROR", Message: err.Error()}
	}

	ext := strings.ToLower(filepath.Ext(path))
	mimeType := t.getMimeType(ext)

	return map[string]interface{}{
		"path":      path,
		"size":      len(data),
		"mime_type": mimeType,
		"extension": ext,
	}, nil
}

func (t *ImageTool) encodeImage(path, format string) (interface{}, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, &Error{Code: "READ_ERROR", Message: err.Error()}
	}

	ext := strings.ToLower(filepath.Ext(path))
	mimeType := t.getMimeType(ext)

	encoded := base64.StdEncoding.EncodeToString(data)

	switch format {
	case "data_url":
		return map[string]interface{}{
			"data_url": "data:" + mimeType + ";base64," + encoded,
			"path":     path,
		}, nil
	default:
		return map[string]interface{}{
			"base64": encoded,
			"path":   path,
			"size":   len(data),
		}, nil
	}
}

func (t *ImageTool) analyzeImage(path string) (interface{}, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, &Error{Code: "READ_ERROR", Message: err.Error()}
	}

	ext := strings.ToLower(filepath.Ext(path))

	return map[string]interface{}{
		"path":      path,
		"size":      len(data),
		"extension": ext,
		"mime_type": t.getMimeType(ext),
		"analysis":  "Image analysis requires vision model",
	}, nil
}

func (t *ImageTool) getMimeType(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".bmp":
		return "image/bmp"
	default:
		return "application/octet-stream"
	}
}

type PDFTool struct{}

func (t *PDFTool) Name() string        { return "pdf" }
func (t *PDFTool) Description() string { return "Read and extract text from PDF files" }

func (t *PDFTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type": "string",
				"enum": []string{"read", "extract", "info", "pages"},
			},
			"path":  map[string]interface{}{"type": "string"},
			"page":  map[string]interface{}{"type": "integer"},
			"start": map[string]interface{}{"type": "integer"},
			"end":   map[string]interface{}{"type": "integer"},
		},
		"required": []string{"operation", "path"},
	}
}

func (t *PDFTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	operation, _ := input["operation"].(string)
	path, _ := input["path"].(string)

	if path == "" {
		return nil, ErrRequiredField("path")
	}

	switch operation {
	case "read":
		return t.readPDF(path, input)
	case "info":
		return t.pdfInfo(path)
	case "pages":
		return t.getPages(path)
	default:
		return nil, &Error{Code: "INVALID_OPERATION", Message: operation}
	}
}

func (t *PDFTool) readPDF(path string, input map[string]interface{}) (interface{}, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, &Error{Code: "READ_ERROR", Message: err.Error()}
	}

	start, _ := input["start"].(int)
	end, _ := input["end"].(int)

	if start == 0 {
		start = 1
	}
	if end == 0 {
		end = -1
	}

	return map[string]interface{}{
		"path":       path,
		"size":       len(data),
		"start_page": start,
		"end_page":   end,
		"text":       "PDF text extraction requires external library",
		"note":       "Install pdftotext for text extraction",
	}, nil
}

func (t *PDFTool) pdfInfo(path string) (interface{}, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, &Error{Code: "READ_ERROR", Message: err.Error()}
	}

	return map[string]interface{}{
		"path": path,
		"size": len(data),
		"info": "PDF info extraction requires external library",
	}, nil
}

func (t *PDFTool) getPages(path string) (interface{}, error) {
	return map[string]interface{}{
		"path":  path,
		"pages": "Page count requires external library",
	}, nil
}

type AudioTool struct{}

func (t *AudioTool) Name() string        { return "audio" }
func (t *AudioTool) Description() string { return "Process audio files" }

func (t *AudioTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type": "string",
				"enum": []string{"read", "info", "transcribe"},
			},
			"path": map[string]interface{}{"type": "string"},
		},
		"required": []string{"operation", "path"},
	}
}

func (t *AudioTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	operation, _ := input["operation"].(string)
	path, _ := input["path"].(string)

	if path == "" {
		return nil, ErrRequiredField("path")
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, &Error{Code: "READ_ERROR", Message: err.Error()}
	}

	ext := strings.ToLower(filepath.Ext(path))

	return map[string]interface{}{
		"operation": operation,
		"path":      path,
		"size":      len(data),
		"extension": ext,
		"mime_type": t.getMimeType(ext),
	}, nil
}

func (t *AudioTool) getMimeType(ext string) string {
	switch ext {
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".m4a":
		return "audio/mp4"
	case ".flac":
		return "audio/flac"
	default:
		return "application/octet-stream"
	}
}
