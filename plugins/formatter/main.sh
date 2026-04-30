#!/bin/sh
set -e

input=$(cat)

file=$(echo "$input" | grep -o '"file"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*:.*"\([^"]*\)".*/\1/')
language=$(echo "$input" | grep -o '"language"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*:.*"\([^"]*\)".*/\1/')

if [ -z "$file" ]; then
    echo '{"error": "missing required field: file"}'
    exit 1
fi

if [ ! -f "$file" ]; then
    echo "{\"error\": \"file not found: $file\"}"
    exit 1
fi

if [ -z "$language" ]; then
    case "$file" in
        *.go)   language="go" ;;
        *.js)   language="js" ;;
        *.py)   language="py" ;;
        *)      language="unknown" ;;
    esac
fi

original_size=$(wc -c < "$file" | tr -d ' ')

case "$language" in
    go)
        if command -v gofmt >/dev/null 2>&1; then
            gofmt -w "$file"
        else
            echo '{"error": "gofmt not found"}'
            exit 1
        fi
        ;;
    js)
        if command -v prettier >/dev/null 2>&1; then
            prettier --write "$file" >/dev/null 2>&1
        else
            echo '{"error": "prettier not found"}'
            exit 1
        fi
        ;;
    py)
        if command -v autopep8 >/dev/null 2>&1; then
            autopep8 --in-place "$file"
        else
            echo '{"error": "autopep8 not found"}'
            exit 1
        fi
        ;;
    *)
        echo "{\"error\": \"unsupported language: $language\"}"
        exit 1
        ;;
esac

formatted_size=$(wc -c < "$file" | tr -d ' ')

printf '{"formatted": true, "file": "%s", "original_size": %s, "formatted_size": %s}' "$file" "$original_size" "$formatted_size"
