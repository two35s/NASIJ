package depintel

import "strings"

type Ecosystem string

const (
	EcosystemNPM     Ecosystem = "npm"
	EcosystemGo      Ecosystem = "Go"
	EcosystemPyPI    Ecosystem = "PyPI"
	EcosystemRubyGems Ecosystem = "RubyGems"
	EcosystemCrates  Ecosystem = "crates.io"
	EcosystemPackagist Ecosystem = "Packagist"
	EcosystemMaven   Ecosystem = "Maven"
	EcosystemNuGet   Ecosystem = "NuGet"
)

type Severity int

const (
	SeverityUnknown  Severity = 0
	SeverityLow      Severity = 1
	SeverityMedium   Severity = 2
	SeverityHigh     Severity = 3
	SeverityCritical Severity = 4
)

func (s Severity) String() string {
	switch s {
	case SeverityCritical:
		return "CRITICAL"
	case SeverityHigh:
		return "HIGH"
	case SeverityMedium:
		return "MEDIUM"
	case SeverityLow:
		return "LOW"
	case SeverityUnknown:
		return "UNKNOWN"
	default:
		return "UNKNOWN"
	}
}

type Package struct {
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	Ecosystem Ecosystem `json:"ecosystem"`
	Source    string    `json:"source"`
}

type Vulnerability struct {
	ID              string   `json:"id"`
	Aliases         []string `json:"aliases,omitempty"`
	Summary         string   `json:"summary,omitempty"`
	Severity        Severity `json:"severity"`
	FixedVersion    string   `json:"fixed_version,omitempty"`
	AffectedPackage string   `json:"affected_package"`
	AffectedVersion string   `json:"affected_version"`
	Ecosystem       Ecosystem `json:"ecosystem"`
	Source          string   `json:"source"`
}

type DepResult struct {
	Target          string          `json:"target"`
	Packages        []Package       `json:"packages"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
}

func (r *DepResult) VulnerableCount() int {
	return len(r.Vulnerabilities)
}

func (r *DepResult) PackageCount() int {
	return len(r.Packages)
}

func ParseVersion(ver string) string {
	ver = strings.TrimPrefix(ver, "^")
	ver = strings.TrimPrefix(ver, "~> ")
	ver = strings.TrimPrefix(ver, "~>")
	ver = strings.TrimPrefix(ver, "~")
	ver = strings.TrimPrefix(ver, ">= ")
	ver = strings.TrimPrefix(ver, ">=")
	ver = strings.TrimPrefix(ver, "<=")
	ver = strings.TrimPrefix(ver, "!=")
	ver = strings.TrimPrefix(ver, "~=")
	ver = strings.TrimPrefix(ver, "> ")
	ver = strings.TrimPrefix(ver, ">")
	ver = strings.TrimPrefix(ver, "< ")
	ver = strings.TrimPrefix(ver, "<")
	ver = strings.TrimPrefix(ver, "= ")
	ver = strings.TrimPrefix(ver, "=")
	ver = strings.TrimPrefix(ver, "v")
	ver = strings.TrimSpace(ver)
	if idx := strings.Index(ver, " "); idx > 0 {
		ver = ver[:idx]
	}
	if idx := strings.Index(ver, ","); idx > 0 {
		ver = ver[:idx]
	}
	return ver
}
