package secrets

import "fmt"

type Severity int

const (
	SeverityInfo     Severity = 0
	SeverityLow      Severity = 1
	SeverityMedium   Severity = 2
	SeverityHigh     Severity = 3
	SeverityCritical Severity = 4
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

type SecretType string

const (
	TypeAWS             SecretType = "aws"
	TypeAzure           SecretType = "azure"
	TypeGCP             SecretType = "gcp"
	TypeJWT             SecretType = "jwt"
	TypeOAuth           SecretType = "oauth"
	TypeAPIKey          SecretType = "api_key"
	TypeToken           SecretType = "token"
	TypeGeneric         SecretType = "generic"
	TypeHighEntropy     SecretType = "high_entropy"
	TypePrivateKey      SecretType = "private_key"
	TypePassword        SecretType = "password"
	TypeConnectionString SecretType = "connection_string"
)

type Finding struct {
	SecretType SecretType `json:"secret_type"`
	Severity   Severity   `json:"severity"`
	Provider   string     `json:"provider"`
	Key        string     `json:"key"`
	Value      string     `json:"value,omitempty"`
	Match      string     `json:"match"`
	Source     string     `json:"source"`
	Context    string     `json:"context"`
	Entropy    float64    `json:"entropy"`
	Line       int        `json:"line,omitempty"`
}

func (f *Finding) String() string {
	return fmt.Sprintf("[%s] %s: %s (%s)", f.Severity, f.Provider, f.Key, f.Source)
}

type ScanResult struct {
	Target   string    `json:"target"`
	Findings []Finding `json:"findings"`
}

func (r *ScanResult) CountBySeverity() map[Severity]int {
	counts := make(map[Severity]int)
	for _, f := range r.Findings {
		counts[f.Severity]++
	}
	return counts
}

func (r *ScanResult) CountByType() map[SecretType]int {
	counts := make(map[SecretType]int)
	for _, f := range r.Findings {
		counts[f.SecretType]++
	}
	return counts
}
