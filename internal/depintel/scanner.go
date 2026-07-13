package depintel

import (
	"fmt"
	"os"
	"path/filepath"
)

type Option func(*Scanner)

type Scanner struct {
	OSVLookup      bool
	AdvisoryLookup bool
	MinSeverity    Severity
	osvClient      *OSVClient
	advisoryDB     *AdvisoryDB
}

func WithOSVLookup(enabled bool) Option {
	return func(s *Scanner) {
		s.OSVLookup = enabled
	}
}

func WithAdvisoryLookup(enabled bool) Option {
	return func(s *Scanner) {
		s.AdvisoryLookup = enabled
	}
}

func WithMinSeverity(s Severity) Option {
	return func(sc *Scanner) {
		sc.MinSeverity = s
	}
}

func NewScanner(opts ...Option) *Scanner {
	s := &Scanner{
		OSVLookup:      true,
		AdvisoryLookup: true,
		MinSeverity:    SeverityUnknown,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Scanner) ensureOSV() {
	if s.osvClient == nil {
		s.osvClient = NewOSVClient()
	}
}

func (s *Scanner) ensureAdvisory() {
	if s.advisoryDB == nil {
		s.advisoryDB = NewAdvisoryDB()
	}
}

func (s *Scanner) ScanDir(dir string) (*DepResult, error) {
	files, err := FindDepFiles(dir)
	if err != nil {
		return nil, fmt.Errorf("find dep files: %w", err)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}

	result := &DepResult{
		Target: absDir,
	}

	for _, path := range files {
		pkgs, err := s.parseFile(path)
		if err != nil {
			continue
		}
		result.Packages = append(result.Packages, pkgs...)
	}

	if err := s.lookupVulnerabilities(result); err != nil {
		return result, err
	}

	return result, nil
}

func (s *Scanner) ScanFiles(paths []string) (*DepResult, error) {
	result := &DepResult{}

	for _, path := range paths {
		pkgs, err := s.parseFile(path)
		if err != nil {
			return nil, err
		}
		result.Packages = append(result.Packages, pkgs...)
	}

	if err := s.lookupVulnerabilities(result); err != nil {
		return result, err
	}

	return result, nil
}

func (s *Scanner) ScanFile(path string) (*DepResult, error) {
	return s.ScanFiles([]string{path})
}

func (s *Scanner) ScanText(text string, source string, eco Ecosystem) (*DepResult, error) {
	result := &DepResult{}

	switch source {
	case "package.json":
		pkgs, err := parsePackageJSON(parsedFile{Path: source, Content: text})
		if err != nil {
			return nil, err
		}
		result.Packages = pkgs
	case "go.mod":
		pkgs, err := parseGoMod(parsedFile{Path: source, Content: text})
		if err != nil {
			return nil, err
		}
		result.Packages = pkgs
	case "requirements.txt":
		pkgs, err := parseRequirementsTxt(parsedFile{Path: source, Content: text})
		if err != nil {
			return nil, err
		}
		result.Packages = pkgs
	default:
		return result, nil
	}

	if err := s.lookupVulnerabilities(result); err != nil {
		return result, err
	}

	return result, nil
}

func (s *Scanner) parseFile(path string) ([]Package, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	base := filepath.Base(path)
	parser, ok := parsers[base]
	if !ok {
		return nil, fmt.Errorf("no parser for %s", base)
	}

	return parser(parsedFile{Path: path, Content: string(content)})
}

func (s *Scanner) lookupVulnerabilities(result *DepResult) error {
	if len(result.Packages) == 0 {
		return nil
	}

	result.Vulnerabilities = nil

	if s.AdvisoryLookup {
		s.ensureAdvisory()
		advisoryVulns := s.advisoryDB.LookupBatch(result.Packages)
		for _, vulns := range advisoryVulns {
			result.Vulnerabilities = append(result.Vulnerabilities, vulns...)
		}
	}

	if s.OSVLookup {
		s.ensureOSV()
		osvVulns, err := s.osvClient.QueryBatch(result.Packages)
		if err != nil {
			return err
		}
		for _, vulns := range osvVulns {
			result.Vulnerabilities = append(result.Vulnerabilities, vulns...)
		}
	}

	result.Vulnerabilities = s.filterBySeverity(dedupVulns(result.Vulnerabilities))

	return nil
}

func (s *Scanner) filterBySeverity(vulns []Vulnerability) []Vulnerability {
	if s.MinSeverity == SeverityUnknown {
		return vulns
	}
	var filtered []Vulnerability
	for _, v := range vulns {
		if v.Severity >= s.MinSeverity {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

func dedupVulns(vulns []Vulnerability) []Vulnerability {
	seen := make(map[string]bool)
	var result []Vulnerability
	for _, v := range vulns {
		key := v.ID + "|" + v.AffectedPackage + "|" + v.AffectedVersion
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, v)
	}
	return result
}
