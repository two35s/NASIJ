package framework

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// Detect runs all framework detectors against the given FileMeta slice.
func Detect(dir string, files []FileMeta) *Result {
	all := make(map[string][]Signal) // framework → signals
	for _, d := range allDetectors() {
		sigs := d.detect(dir, files)
		for fw, s := range sigs {
			all[fw] = append(all[fw], s...)
		}
	}
	res := &Result{}
	for name, sigs := range all {
		c := confidence(sigs)
		if c > 0 {
			res.Frameworks = append(res.Frameworks, Framework{
				Name:       name,
				Confidence: c,
				Signals:    sigs,
			})
		}
	}
	return res
}

// DetectFromPaths walks dir, reads files up to maxSize, then calls Detect.
func DetectFromPaths(dir string, maxSize int64) *Result {
	files := walkFiles(dir, maxSize)
	return Detect(dir, files)
}

func walkFiles(dir string, maxSize int64) []FileMeta {
	var files []FileMeta
	_ = filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || strings.HasPrefix(fi.Name(), ".") {
			return nil
		}
		m := FileMeta{Path: path}
		if fi.Size() <= maxSize {
			b, _ := os.ReadFile(path)
			m.Content = string(b)
		}
		files = append(files, m)
		return nil
	})
	return files
}

func confidence(sigs []Signal) float64 {
	var s float64
	for _, sg := range sigs {
		s += sg.Weight
	}
	if s > 1.0 {
		return 1.0
	}
	return s
}

func sig(desc, file string, weight float64) Signal {
	return Signal{Description: desc, File: file, Weight: weight}
}

// ---------------------------------------------------------------------------
// Detector interface
// ---------------------------------------------------------------------------

type detectorIfc interface {
	detect(dir string, files []FileMeta) map[string][]Signal
}

func allDetectors() []detectorIfc {
	return []detectorIfc{
		&pkgDetector{},
		&configDetector{},
		&extDetector{},
		&sourceDetector{},
	}
}

// ---------------------------------------------------------------------------
// 1. package.json detector
// ---------------------------------------------------------------------------

type pkgDetector struct{}

type pkgJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

var depFramework = map[string]string{
	"react": "react", "react-dom": "react",
	"next": "next",
	"@angular/core": "angular", "@angular/platform-browser": "angular",
	"@angular/router": "angular", "@angular/forms": "angular",
	"@angular/common": "angular", "@angular/http": "angular",
	"vue": "vue",
	"nuxt": "nuxt", "nuxt3": "nuxt",
	"astro": "astro",
	"@remix-run/react": "remix", "@remix-run/node": "remix",
	"@remix-run/serve": "remix",
	"svelte": "svelte",
	"webpack": "webpack",
	"vite": "vite",
}

func depWeight(name string) float64 {
	switch name {
	case "react", "react-dom", "@angular/core", "vue", "astro", "svelte":
		return 0.5
	case "next", "nuxt", "nuxt3", "@remix-run/react":
		return 0.45
	case "@angular/platform-browser", "@angular/router", "@angular/forms":
		return 0.4
	case "webpack", "vite", "@remix-run/node", "@remix-run/serve":
		return 0.35
	default:
		if strings.HasPrefix(name, "@angular/") {
			return 0.35
		}
		return 0
	}
}

func (d *pkgDetector) detect(dir string, files []FileMeta) map[string][]Signal {
	var pkg *pkgJSON
	for _, f := range files {
		if filepath.Base(f.Path) == "package.json" {
			var p pkgJSON
			if err := json.Unmarshal([]byte(f.Content), &p); err == nil {
				pkg = &p
			}
			break
		}
	}
	if pkg == nil {
		b, err := os.ReadFile(filepath.Join(dir, "package.json"))
		if err != nil {
			return nil
		}
		var p pkgJSON
		if err := json.Unmarshal(b, &p); err != nil {
			return nil
		}
		pkg = &p
	}
	if pkg == nil {
		return nil
	}

	result := make(map[string][]Signal)
	allDeps := make(map[string]string)
	for k, v := range pkg.Dependencies {
		allDeps[k] = v
	}
	for k, v := range pkg.DevDependencies {
		allDeps[k] = v
	}
	for name, ver := range allDeps {
		fw, ok := depFramework[name]
		if !ok {
			continue
		}
		desc := "dependency: " + name
		if ver != "" {
			desc += "@" + ver
		}
		result[fw] = append(result[fw], sig(desc, "package.json", depWeight(name)))
	}
	return result
}

// ---------------------------------------------------------------------------
// 2. Config-file detector
// ---------------------------------------------------------------------------

type configDetector struct{}

var configPatterns = []struct {
	re     *regexp.Regexp
	fw     string
	desc   string
	weight float64
}{
	{regexp.MustCompile(`^next\.config\.(js|mjs|ts)$`), "next", "Next.js config", 0.5},
	{regexp.MustCompile(`^angular\.json$`), "angular", "Angular CLI config", 0.5},
	{regexp.MustCompile(`^nuxt\.config\.(js|ts|mjs)$`), "nuxt", "Nuxt config", 0.5},
	{regexp.MustCompile(`^astro\.config\.(mjs|js|ts)$`), "astro", "Astro config", 0.5},
	{regexp.MustCompile(`^remix\.config\.(js|ts)$`), "remix", "Remix config", 0.5},
	{regexp.MustCompile(`^svelte\.config\.(js|ts)$`), "svelte", "Svelte config", 0.3},
	{regexp.MustCompile(`^webpack\.config\.(js|ts|mjs|cjs)$`), "webpack", "Webpack config", 0.5},
	{regexp.MustCompile(`^vite\.config\.(ts|js|mjs)$`), "vite", "Vite config", 0.5},
	{regexp.MustCompile(`tsconfig\.json$`), "angular", "tsconfig.json (Angular hint)", 0.1},
}

func (d *configDetector) detect(dir string, files []FileMeta) map[string][]Signal {
	result := make(map[string][]Signal)
	for _, f := range files {
		base := filepath.Base(f.Path)
		for _, cp := range configPatterns {
			if cp.re.MatchString(base) {
				result[cp.fw] = append(result[cp.fw], sig(cp.desc, f.Path, cp.weight))
			}
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 3. File-extension detector
// ---------------------------------------------------------------------------

type extDetector struct{}

var extPatterns = []struct {
	ext    string
	fw     string
	desc   string
	weight float64
}{
	{".vue", "vue", "Vue SFC", 0.4},
	{".svelte", "svelte", "Svelte component", 0.4},
	{".astro", "astro", "Astro component", 0.4},
	{".jsx", "react", "JSX file", 0.2},
	{".tsx", "react", "TSX file", 0.2},
}

func (d *extDetector) detect(dir string, files []FileMeta) map[string][]Signal {
	result := make(map[string][]Signal)
	seen := make(map[string]bool)
	for _, f := range files {
		ext := filepath.Ext(f.Path)
		for _, ep := range extPatterns {
			if ext == ep.ext {
				key := ep.fw + ":" + ep.desc
				if !seen[key] {
					seen[key] = true
					result[ep.fw] = append(result[ep.fw], sig(ep.desc, f.Path, ep.weight))
				}
			}
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 4. Source-code detector
// ---------------------------------------------------------------------------

type sourceDetector struct{}

var sourcePatterns = map[string][]struct {
	re     *regexp.Regexp
	desc   string
	weight float64
}{
	"react": {
		{regexp.MustCompile(`from\s+['"]react['"]`), "import from react", 0.3},
		{regexp.MustCompile(`from\s+['"]react-dom['"]`), "import from react-dom", 0.3},
		{regexp.MustCompile(`React\.(createElement|Component|PureComponent|Fragment)\b`), "React global API", 0.25},
		{regexp.MustCompile(`useState|useEffect|useContext|useRef|useMemo|useCallback|useReducer`), "React hook usage", 0.25},
		{regexp.MustCompile(`ReactDOM\.(render|createRoot|hydrate)\b`), "ReactDOM API", 0.25},
		{regexp.MustCompile(`export\s+default\s+(function|class|const)\s+\w+\s*(?:<[A-Z]|\()`), "React component export", 0.15},
	},
	"next": {
		{regexp.MustCompile(`from\s+['"]next['"]`), "import from next", 0.3},
		{regexp.MustCompile(`from\s+['"]next/(\w+)['"]`), "import from next/*", 0.3},
		{regexp.MustCompile(`getStaticProps|getServerSideProps|getStaticPaths`), "Next data-fetching", 0.3},
		{regexp.MustCompile(`useRouter\s*\(\s*\)`), "next/router usage", 0.2},
		{regexp.MustCompile(`next\.(js|ts|mjs)`), "Next config in source", 0.1},
	},
	"angular": {
		{regexp.MustCompile(`from\s+['"]@angular/core['"]`), "import @angular/core", 0.3},
		{regexp.MustCompile(`@Component\s*\(\s*\{`), "@Component decorator", 0.35},
		{regexp.MustCompile(`@Injectable\s*\(\s*\)`), "@Injectable decorator", 0.25},
		{regexp.MustCompile(`@NgModule\s*\(\s*\{`), "@NgModule decorator", 0.3},
		{regexp.MustCompile(`from\s+['"]@angular/(platform-browser|router|forms|http|common)['"]`), "import @angular/*", 0.25},
		{regexp.MustCompile(`ng-app\s*=|ng-controller\s*=`), "AngularJS directive", 0.2},
	},
	"vue": {
		{regexp.MustCompile(`from\s+['"]vue['"]`), "import from vue", 0.3},
		{regexp.MustCompile(`createApp\s*\(`), "Vue createApp", 0.3},
		{regexp.MustCompile(`Vue\.(component|directive|mixin|use)\s*\(`), "Vue global API", 0.25},
		{regexp.MustCompile(`new\s+Vue\s*\(\s*\{`), "Vue 2 constructor", 0.3},
		{regexp.MustCompile(`v-(if|for|model|bind|on|show|html|text|slot)\s*[=:]`), "Vue directive", 0.2},
	},
	"nuxt": {
		{regexp.MustCompile(`from\s+['"]nuxt['"]`), "import from nuxt", 0.3},
		{regexp.MustCompile(`from\s+['"]#app['"]`), "Nuxt #app import", 0.25},
		{regexp.MustCompile(`definePageMeta\s*\(\s*\{`), "Nuxt definePageMeta", 0.25},
		{regexp.MustCompile(`useFetch\s*\(\s*['"\/]`), "Nuxt useFetch", 0.2},
		{regexp.MustCompile(`useState\s*\(\s*['"][^'"]+['"]\s*\)`), "Nuxt useState", 0.2},
	},
	"astro": {
		{regexp.MustCompile(`---\s*[\s\S]*?---`), "Astro frontmatter", 0.35},
		{regexp.MustCompile(`Astro\.(props|request|response|slots|glob|redirect|url)\b`), "Astro global", 0.3},
		{regexp.MustCompile(`from\s+['"]astro['"]`), "import from astro", 0.25},
	},
	"remix": {
		{regexp.MustCompile(`from\s+['"]@remix-run/\w+['"]`), "import @remix-run/*", 0.35},
		{regexp.MustCompile(`useLoaderData\s*\(\s*\)`), "useLoaderData", 0.25},
		{regexp.MustCompile(`useActionData\s*\(\s*\)`), "useActionData", 0.25},
		{regexp.MustCompile(`json\s*\(\s*\{`), "Remix json helper", 0.15},
	},
	"svelte": {
		{regexp.MustCompile(`<script[^>]*>[\s\S]*?export\s+(let|const|function|class)\s+\w+`), "Svelte export declaration", 0.3},
		{regexp.MustCompile(`\$\s*:`), "Svelte reactive $:", 0.25},
		{regexp.MustCompile(`on:click|on:submit|on:input|bind:value`), "Svelte event binding", 0.2},
		{regexp.MustCompile(`\{#each\s+|\{#if\s+|\{#await\s+`), "Svelte block", 0.25},
		{regexp.MustCompile(`from\s+['"]svelte['"]`), "import from svelte", 0.25},
	},
	"webpack": {
		{regexp.MustCompile(`__webpack_public_path__`), "webpack public path", 0.3},
		{regexp.MustCompile(`__webpack_require__`), "webpack require", 0.3},
		{regexp.MustCompile(`webpackChunkName`), "webpack chunk name", 0.2},
		{regexp.MustCompile(`__webpack_chunk_load__`), "webpack chunk load", 0.25},
	},
	"vite": {
		{regexp.MustCompile(`import\.meta\.env\b`), "import.meta.env", 0.25},
		{regexp.MustCompile(`import\.meta\.glob\b`), "import.meta.glob", 0.25},
		{regexp.MustCompile(`import\.meta\.hot\b`), "import.meta.hot (HMR)", 0.2},
		{regexp.MustCompile(`new\s+URL\s*\(\s*['"][^'"]+['"]\s*,\s*import\.meta\.url\s*\)`), "Vite asset URL", 0.25},
		{regexp.MustCompile(`__VITE_ASSET__`), "Vite asset placeholder", 0.2},
	},
}

func (d *sourceDetector) detect(dir string, files []FileMeta) map[string][]Signal {
	result := make(map[string][]Signal)
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.Path))
		if !isSourceExt(ext) || f.Content == "" {
			continue
		}
		for fw, patterns := range sourcePatterns {
			for _, p := range patterns {
				if p.re.MatchString(f.Content) {
					result[fw] = append(result[fw], sig(p.desc, f.Path, p.weight))
				}
			}
		}
	}
	return result
}

func isSourceExt(ext string) bool {
	switch ext {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".vue", ".svelte", ".astro", ".html", ".htm":
		return true
	}
	return false
}
