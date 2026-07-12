package jscollect

import (
	"regexp"
	"strings"
)

// ScanResult holds everything discovered from scanning JS source code.
type ScanResult struct {
	SourceURL string

	// ScriptURLs are URLs found by heuristic string literal scanning.
	ScriptURLs []string

	// DynamicImports are specifiers found in import() calls.
	DynamicImports []string

	// WebpackChunks are identifiers/references to webpack chunks.
	WebpackChunks []string

	// ViteAssets are patterns found in Vite-specific API calls.
	ViteAssets []string

	// SourceMaps are sourceMappingURL values (URLs or inline data URIs).
	SourceMaps []SourceMapRef
}

// SourceMapRef represents a source map reference.
type SourceMapRef struct {
	URL        string
	Inline     bool
	InlineData string
}

// Scanner scans JS source for patterns.
type Scanner struct{}

// New creates a Scanner.
func New() *Scanner {
	return &Scanner{}
}

// Scan analyzes JS source and returns all discovered patterns.
func (s *Scanner) Scan(source, sourceURL string) *ScanResult {
	res := &ScanResult{
		SourceURL: sourceURL,
	}

	res.ScriptURLs = extractScriptURLs(source)
	res.DynamicImports = extractDynamicImports(source)
	res.WebpackChunks = extractWebpackChunks(source)
	res.ViteAssets = extractViteAssets(source)
	res.SourceMaps = extractSourceMaps(source)

	return res
}

// ---------------------------------------------------------------------------
// String-literal URL extraction (heuristic)
// ---------------------------------------------------------------------------

var urlSuffixes = []string{".js", ".mjs", ".cjs", ".jsx", ".ts", ".tsx",
	".css", ".json", ".html", ".svg", ".png", ".jpg", ".jpeg", ".gif",
	".webp", ".ico", ".woff", ".woff2", ".ttf", ".eot",
}

func extractScriptURLs(source string) []string {
	seen := make(map[string]struct{})
	var urls []string

	for _, quote := range []string{"\"", "'", "`"} {
		remaining := source
		for {
			idx := strings.Index(remaining, quote)
			if idx < 0 {
				break
			}
			remaining = remaining[idx+1:]
			end := strings.Index(remaining, quote)
			if end < 0 {
				break
			}
			str := strings.TrimSpace(remaining[:end])
			remaining = remaining[end+1:]

			if isLikelyURL(str) {
				if _, exists := seen[str]; !exists {
					seen[str] = struct{}{}
					urls = append(urls, str)
				}
			}
		}
	}
	return urls
}

func isLikelyURL(s string) bool {
	if len(s) < 4 {
		return false
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return true
	}
	if strings.HasPrefix(s, "//") && len(s) > 4 {
		return true
	}
	if strings.HasPrefix(s, "/") && strings.Contains(s[1:], ".") && !strings.Contains(s, " ") {
		return true
	}
	if strings.Count(s, ".") >= 1 && strings.Contains(s, "/") && !strings.Contains(s, " ") {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Dynamic imports: import('...')
// ---------------------------------------------------------------------------

var dynamicImportRE = regexp.MustCompile(`import\s*\(\s*['"]([^'"]+)['"]\s*\)`)

func extractDynamicImports(source string) []string {
	matches := dynamicImportRE.FindAllStringSubmatch(source, -1)
	var imports []string
	seen := make(map[string]struct{})
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		spec := strings.TrimSpace(m[1])
		if spec == "" {
			continue
		}
		if _, exists := seen[spec]; !exists {
			seen[spec] = struct{}{}
			imports = append(imports, spec)
		}
	}
	return imports
}

// ---------------------------------------------------------------------------
// Webpack chunks
// ---------------------------------------------------------------------------

var (
	webpackChunkRE   = regexp.MustCompile(`__webpack_public_path__\s*=\s*['"]([^'"]+)['"]`)
	webpackChunkName = regexp.MustCompile(`['"]chunk-([a-f0-9]+)['"]`)
	webpackImportRE  = regexp.MustCompile(`import\s*\(\s*/*\s*webpackChunkName\s*:\s*['"]([^'"]+)['"]`)
	webpackPrefetch  = regexp.MustCompile(`__webpack_chunk_load__\(['"]([^'"]+)['"]\)`)
)

func extractWebpackChunks(source string) []string {
	var chunks []string
	seen := make(map[string]struct{})

	// __webpack_public_path__ = "..."
	for _, m := range webpackChunkRE.FindAllStringSubmatch(source, -1) {
		if len(m) >= 2 {
			chunks = append(chunks, m[1])
		}
	}

	// "chunk-xxxxx" patterns
	for _, m := range webpackChunkName.FindAllStringSubmatch(source, -1) {
		if len(m) >= 2 {
			id := m[1]
			if _, exists := seen[id]; !exists {
				seen[id] = struct{}{}
				chunks = append(chunks, "chunk-"+id)
			}
		}
	}

	// webpackChunkName hints in comments
	for _, m := range webpackImportRE.FindAllStringSubmatch(source, -1) {
		if len(m) >= 2 {
			name := strings.TrimSpace(m[1])
			if name != "" {
				if _, exists := seen[name]; !exists {
					seen[name] = struct{}{}
					chunks = append(chunks, name)
				}
			}
		}
	}

	// __webpack_chunk_load__("...")
	for _, m := range webpackPrefetch.FindAllStringSubmatch(source, -1) {
		if len(m) >= 2 {
			spec := strings.TrimSpace(m[1])
			if spec != "" {
				if _, exists := seen[spec]; !exists {
					seen[spec] = struct{}{}
					chunks = append(chunks, spec)
				}
			}
		}
	}

	return chunks
}

// ---------------------------------------------------------------------------
// Vite assets
// ---------------------------------------------------------------------------

var (
	viteStaticImportRE = regexp.MustCompile(`new\s+URL\s*\(\s*['"]([^'"]+)['"]\s*,\s*import\.meta\.url\s*\)`)
	viteGlobRE         = regexp.MustCompile(`import\.meta\.glob\s*\(\s*['"]([^'"]+)['"]`)
)

func extractViteAssets(source string) []string {
	var assets []string
	seen := make(map[string]struct{})

	// new URL('./...', import.meta.url)
	for _, m := range viteStaticImportRE.FindAllStringSubmatch(source, -1) {
		if len(m) >= 2 {
			asset := strings.TrimSpace(m[1])
			if asset != "" {
				if _, exists := seen[asset]; !exists {
					seen[asset] = struct{}{}
					assets = append(assets, asset)
				}
			}
		}
	}

	// import.meta.glob('...')
	for _, m := range viteGlobRE.FindAllStringSubmatch(source, -1) {
		if len(m) >= 2 {
			glob := strings.TrimSpace(m[1])
			if glob != "" {
				if _, exists := seen[glob]; !exists {
					seen[glob] = struct{}{}
					assets = append(assets, glob)
				}
			}
		}
	}

	return assets
}

// ---------------------------------------------------------------------------
// Source maps
// ---------------------------------------------------------------------------

var (
	sourceMapURLRE = regexp.MustCompile(`(?m)^[ \t]*//[#@]\s*sourceMappingURL=(.+)$`)
	sourceMapBlock = regexp.MustCompile(`/\*\s*[#@]\s*sourceMappingURL=(.+?)\s*\*/`)
)

func extractSourceMaps(source string) []SourceMapRef {
	var refs []SourceMapRef
	seen := make(map[string]struct{})

	for _, m := range sourceMapURLRE.FindAllStringSubmatch(source, -1) {
		if len(m) >= 2 {
			val := strings.TrimSpace(m[1])
			if val == "" {
				continue
			}
			if _, exists := seen[val]; !exists {
				seen[val] = struct{}{}
				ref := SourceMapRef{URL: val}
				if strings.HasPrefix(val, "data:") {
					ref.Inline = true
					if idx := strings.Index(val, "base64,"); idx >= 0 {
						ref.InlineData = val[idx+7:]
					}
				}
				refs = append(refs, ref)
			}
		}
	}

	for _, m := range sourceMapBlock.FindAllStringSubmatch(source, -1) {
		if len(m) >= 2 {
			val := strings.TrimSpace(m[1])
			if val == "" {
				continue
			}
			if _, exists := seen[val]; !exists {
				seen[val] = struct{}{}
				ref := SourceMapRef{URL: val}
				if strings.HasPrefix(val, "data:") {
					ref.Inline = true
					if idx := strings.Index(val, "base64,"); idx >= 0 {
						ref.InlineData = val[idx+7:]
					}
				}
				refs = append(refs, ref)
			}
		}
	}

	return refs
}
