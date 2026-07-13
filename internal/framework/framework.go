package framework

// Framework describes a detected web framework or bundler.
type Framework struct {
	Name       string   `json:"name"`
	Version    string   `json:"version,omitempty"`
	Confidence float64  `json:"confidence"` // 0.0 – 1.0
	Signals    []Signal `json:"signals,omitempty"`
}

// Signal is a single piece of evidence that contributed to detection.
type Signal struct {
	Description string  `json:"description"`
	File        string  `json:"file,omitempty"`
	Weight      float64 `json:"weight"`
}

// Result holds the full output of framework detection.
type Result struct {
	Frameworks []Framework `json:"frameworks"`
	Errors     []error     `json:"errors,omitempty"`
}

// KnownFrameworks lists all frameworks this detector recognises.
var KnownFrameworks = []string{
	"react", "next", "angular", "vue", "nuxt",
	"astro", "remix", "svelte", "webpack", "vite",
}

// detector is the interface each framework-specific checker implements.
type detector interface {
	Name() string
	Check(dir string, files []FileMeta) (*Framework, error)
}

// FileMeta holds metadata about a file in the target directory.
type FileMeta struct {
	Path    string
	Content string
}
