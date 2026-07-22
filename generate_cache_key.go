package plush

import (
	"strings"
	"sync"

	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/helpers/meta"
)

var (
	// Pre-computed character lookup table (much faster than regex)
	charTable [256]byte

	// String pools for reducing allocations
	stringPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 0, 1024) // Larger initial capacity for file paths
			return &buf
		},
	}
)

func init() {
	// Initialize character lookup table once at startup
	for i := 0; i < 256; i++ {
		char := byte(i)
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_' || char == '.' {
			charTable[i] = char // Keep valid characters
		} else {
			charTable[i] = '_' // Replace invalid with underscore
		}
	}
}

// Ultra-fast sanitization optimized for long file paths
func sanitizeCacheKey(input string) string {
	if input == "" {
		return ""
	}
	if cacheKeyAlreadySanitized(input) {
		return input
	}

	// Get buffer from pool with larger capacity for file paths
	bufPtr := stringPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]
	defer func() {
		*bufPtr = buf
		stringPool.Put(bufPtr)
	}()

	// Ensure buffer has enough capacity to avoid reallocations
	if cap(buf) < len(input) {
		buf = make([]byte, 0, len(input)+128)
	}

	lastWasUnderscore := false

	// Process each byte using lookup table - single pass
	for i := 0; i < len(input); i++ {
		char := input[i]
		sanitized := charTable[char]

		// Handle consecutive underscores in same pass
		if sanitized == '_' {
			if !lastWasUnderscore {
				buf = append(buf, '_')
				lastWasUnderscore = true
			}
		} else {
			buf = append(buf, sanitized)
			lastWasUnderscore = false
		}
	}

	// Trim trailing underscore if needed
	if len(buf) > 0 && buf[len(buf)-1] == '_' {
		buf = buf[:len(buf)-1]
	}

	return string(buf)
}

func cacheKeyAlreadySanitized(input string) bool {
	if input == "" {
		return true
	}
	lastWasUnderscore := false
	for i := 0; i < len(input); i++ {
		char := input[i]
		if charTable[char] != char {
			return false
		}
		if char == '_' {
			if lastWasUnderscore {
				return false
			}
			lastWasUnderscore = true
			continue
		}
		lastWasUnderscore = false
	}
	return !lastWasUnderscore
}

// Enhanced file path cleaning for better performance
func cleanFilePath(filename string) string {
	if filename == "" {
		return ""
	}
	var cleanPath string
	if strings.ContainsRune(filename, '\\') {
		cleanPath = strings.ReplaceAll(filename, "\\", "/")
	} else {
		cleanPath = filename
	}
	absolutePath := strings.HasPrefix(cleanPath, "/")
	cleanPath = strings.Trim(cleanPath, "/")
	if cleanPath == "" {
		if absolutePath {
			return "/"
		}
		return ""
	}

	parts := strings.Split(cleanPath, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		part = sanitizeCacheKey(part)
		if part != "" {
			out = append(out, part)
		}
	}
	cleanPath = strings.Join(out, "/")
	if absolutePath && cleanPath != "" {
		return "/" + cleanPath
	}
	return cleanPath
}

// Enhanced URL cleaning with optimized performance
func cleanRequestURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	// Fast path for simple paths
	if rawURL[0] == '/' {
		return cleanURLPath(rawURL)
	}

	// Handle URLs with scheme using rune iteration (faster than url.Parse for most cases)
	return cleanFullURL(rawURL)
}

// Fast path for URL paths (starts with /)
func cleanURLPath(path string) string {
	// Find query/fragment positions
	queryPos := strings.IndexByte(path, '?')
	fragmentPos := strings.IndexByte(path, '#')

	// Determine where to cut
	cutPos := len(path)
	if queryPos != -1 && (fragmentPos == -1 || queryPos < fragmentPos) {
		cutPos = queryPos
	} else if fragmentPos != -1 {
		cutPos = fragmentPos
	}

	// Extract clean path
	cleanPath := strings.TrimLeft(path[1:cutPos], "/")
	if cleanPath == "" {
		return ""
	}

	return sanitizeCacheKey(cleanPath)
}

// Clean full URLs using rune iteration (avoids url.Parse overhead)
func cleanFullURL(rawURL string) string {
	var hostStart, hostEnd, pathStart, pathEnd int
	var foundSlashes bool
	slashCount := 0

	// Parse URL components in single pass
parseURL:
	for i, r := range rawURL {
		switch {
		case !foundSlashes && r == ':':
			// End of scheme
			continue

		case !foundSlashes && r == '/':
			slashCount++
			if slashCount == 2 {
				// Found "//" - start of host
				hostStart = i + 1
				foundSlashes = true
			}
			continue

		case foundSlashes && hostEnd == 0:
			if r == '/' {
				// End of host, start of path
				hostEnd = i
				pathStart = i + 1
			} else if r == '?' || r == '#' {
				// Query or fragment - end of host, no path
				hostEnd = i
				break parseURL
			}
			continue

		case hostEnd > 0 && pathEnd == 0:
			if r == '?' || r == '#' {
				// End of path
				pathEnd = i
				break parseURL
			}
		}
	}

	// Handle case where URL ends without query/fragment
	if foundSlashes && hostEnd == 0 {
		hostEnd = len(rawURL)
	}
	if hostEnd > 0 && pathStart > 0 && pathEnd == 0 {
		pathEnd = len(rawURL)
	}

	// Extract and sanitize components
	var parts []string

	// Add host if present
	if hostEnd > hostStart {
		host := rawURL[hostStart:hostEnd]
		if host != "" {
			parts = append(parts, sanitizeCacheKey(host))
		}
	}

	// Add path if present
	if pathEnd > pathStart {
		path := rawURL[pathStart:pathEnd]
		if path != "" && path != "/" {
			parts = append(parts, sanitizeCacheKey(path))
		}
	}

	// Fallback: if no host/path found, sanitize the whole thing
	if len(parts) == 0 {
		// Handle edge cases like "localhost", "example.com", etc.
		return sanitizeCacheKey(rawURL)
	}

	return strings.Join(parts, "_")
}

func generateCacheKeyFromCleanFilename(cleanFilename string, ctx hctx.Context) string {
	currentURL := ctx.Value(meta.TemplateCurrentUrlKey)
	if currentURL == nil {
		return cleanFilename
	}
	if url, ok := currentURL.(string); ok && url != "" {
		cleanURL := cleanRequestURL(url)
		if cleanURL != "" {
			return cleanFilename + "|url:" + cleanURL
		}
	}
	return cleanFilename
}

func GenerateASTKey(filename string) string {
	cleanFilename := cleanFilePath(filename)
	return GenerateASTKeyFromCleanFilename(cleanFilename)
}

func GenerateASTKeyFromCleanFilename(cleanFilename string) string {
	return "ast:" + cleanFilename
}

func generateFullKeyFromCleanFilename(cleanFilename string, ctx hctx.Context) string {
	return "full:" + generateCacheKeyFromCleanFilename(cleanFilename, ctx)
}
