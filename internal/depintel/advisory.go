package depintel

import (
	"strings"
	"sync"
)

type Advisory struct {
	ID              string   `json:"id"`
	PackageName     string   `json:"package_name"`
	Ecosystem       Ecosystem `json:"ecosystem"`
	Severity        Severity `json:"severity"`
	Summary         string   `json:"summary"`
	FixedIn         string   `json:"fixed_in,omitempty"`
	IntroducedIn    string   `json:"introduced_in,omitempty"`
}

type AdvisoryDB struct {
	mu       sync.RWMutex
	advisories []Advisory
}

func NewAdvisoryDB() *AdvisoryDB {
	db := &AdvisoryDB{}
	db.loadDefaults()
	return db
}

func (db *AdvisoryDB) Add(a Advisory) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.advisories = append(db.advisories, a)
}

func (db *AdvisoryDB) Lookup(pkg Package) []Vulnerability {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var vulns []Vulnerability
	for _, a := range db.advisories {
		if a.PackageName != pkg.Name && !strings.HasPrefix(pkg.Name, a.PackageName) {
			continue
		}
		if a.Ecosystem != "" && a.Ecosystem != pkg.Ecosystem {
			continue
		}

		if pkg.Version == "" {
			continue
		}

		if a.FixedIn != "" {
			if compareVersions(pkg.Version, a.FixedIn) < 0 {
				vulns = append(vulns, Vulnerability{
					ID:              a.ID,
					Summary:         a.Summary,
					Severity:        a.Severity,
					FixedVersion:    a.FixedIn,
					AffectedPackage: pkg.Name,
					AffectedVersion: pkg.Version,
					Ecosystem:       pkg.Ecosystem,
					Source:          "advisory-db",
				})
			}
		}
	}
	return vulns
}

func (db *AdvisoryDB) LookupBatch(pkgs []Package) map[Package][]Vulnerability {
	result := make(map[Package][]Vulnerability)
	for _, pkg := range pkgs {
		vulns := db.Lookup(pkg)
		if len(vulns) > 0 {
			result[pkg] = vulns
		}
	}
	return result
}

func compareVersions(a, b string) int {
	va := parseVersionParts(a)
	vb := parseVersionParts(b)
	minLen := len(va)
	if len(vb) < minLen {
		minLen = len(vb)
	}
	for i := 0; i < minLen; i++ {
		if va[i] < vb[i] {
			return -1
		}
		if va[i] > vb[i] {
			return 1
		}
	}
	if len(va) < len(vb) {
		return -1
	}
	if len(va) > len(vb) {
		return 1
	}
	return 0
}

func parseVersionParts(v string) []int {
	v = stripVersionPrefix(v)
	parts := strings.Split(v, ".")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n := 0
		for i := 0; i < len(p); i++ {
			if p[i] >= '0' && p[i] <= '9' {
				n = n*10 + int(p[i]-'0')
			} else {
				break
			}
		}
		nums = append(nums, n)
	}
	return nums
}

func stripVersionPrefix(v string) string {
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

func (db *AdvisoryDB) loadDefaults() {
	db.advisories = []Advisory{
		// Lodash — Prototype Pollution
		{ID: "CVE-2020-8203", PackageName: "lodash", Ecosystem: EcosystemNPM, Severity: SeverityHigh, Summary: "Prototype pollution in lodash < 4.17.20", FixedIn: "4.17.20"},
		{ID: "CVE-2019-10744", PackageName: "lodash", Ecosystem: EcosystemNPM, Severity: SeverityMedium, Summary: "Prototype pollution in lodash < 4.17.15", FixedIn: "4.17.15"},
		{ID: "CVE-2018-16487", PackageName: "lodash", Ecosystem: EcosystemNPM, Severity: SeverityMedium, Summary: "Prototype pollution in lodash < 4.17.11", FixedIn: "4.17.11"},
		// Express — various
		{ID: "CVE-2022-24999", PackageName: "qs", Ecosystem: EcosystemNPM, Severity: SeverityHigh, Summary: "qs prototype pollution < 6.7.3, 6.6.1, 6.5.3, 6.4.1, 6.3.3, 6.2.4", FixedIn: "6.7.3"},
		{ID: "GHSA-fr57-f8f5-h4vw", PackageName: "express", Ecosystem: EcosystemNPM, Severity: SeverityMedium, Summary: "Express open redirect < 4.17.3", FixedIn: "4.17.3"},
		{ID: "CVE-2024-29041", PackageName: "express", Ecosystem: EcosystemNPM, Severity: SeverityMedium, Summary: "Express path traversal < 4.19.2", FixedIn: "4.19.2"},
		// Axios — SSRF
		{ID: "CVE-2023-45857", PackageName: "axios", Ecosystem: EcosystemNPM, Severity: SeverityHigh, Summary: "Axios SSRF via absolute URL < 1.6.0", FixedIn: "1.6.0"},
		{ID: "CVE-2024-39338", PackageName: "axios", Ecosystem: EcosystemNPM, Severity: SeverityMedium, Summary: "Axios request-smuggling via Content-Type bypass", FixedIn: "1.7.4"},
		// next.js
		{ID: "CVE-2024-34351", PackageName: "next", Ecosystem: EcosystemNPM, Severity: SeverityHigh, Summary: "Next.js SSRF < 14.1.1", FixedIn: "14.1.1"},
		{ID: "CVE-2024-47831", PackageName: "next", Ecosystem: EcosystemNPM, Severity: SeverityHigh, Summary: "Next.js Server-Side DoS < 14.2.10", FixedIn: "14.2.10"},
		// React
		{ID: "GHSA-6g9j-2wmw-4f3f", PackageName: "react-dom", Ecosystem: EcosystemNPM, Severity: SeverityMedium, Summary: "React DOM XSS in < 16.13.1, 17.0.2, 18.2.0", FixedIn: "18.2.0"},
		// jQuery
		{ID: "CVE-2020-11023", PackageName: "jquery", Ecosystem: EcosystemNPM, Severity: SeverityMedium, Summary: "jQuery XSS < 3.5.0", FixedIn: "3.5.0"},
		{ID: "CVE-2020-11022", PackageName: "jquery", Ecosystem: EcosystemNPM, Severity: SeverityMedium, Summary: "jQuery XSS via HTML parsing < 3.5.0", FixedIn: "3.5.0"},
		// minimatch / glob-parent
		{ID: "CVE-2021-26945", PackageName: "minimatch", Ecosystem: EcosystemNPM, Severity: SeverityLow, Summary: "minimatch ReDoS < 3.0.5", FixedIn: "3.0.5"},
		// nth-check
		{ID: "CVE-2021-3803", PackageName: "nth-check", Ecosystem: EcosystemNPM, Severity: SeverityHigh, Summary: "nth-check ReDoS < 2.0.1", FixedIn: "2.0.1"},
		// trim-newlines
		{ID: "CVE-2021-33623", PackageName: "trim-newlines", Ecosystem: EcosystemNPM, Severity: SeverityHigh, Summary: "trim-newlines ReDoS < 3.0.1, 4.0.1", FixedIn: "3.0.1"},
		// semver-regex
		{ID: "CVE-2021-43307", PackageName: "semver-regex", Ecosystem: EcosystemNPM, Severity: SeverityLow, Summary: "semver-regex ReDoS < 3.1.4", FixedIn: "3.1.4"},
		// shelljs
		{ID: "CVE-2022-0144", PackageName: "shelljs", Ecosystem: EcosystemNPM, Severity: SeverityHigh, Summary: "shelljs command injection < 0.8.5", FixedIn: "0.8.5"},
		// path-parse
		{ID: "GHSA-c6rc-g2vg-9h3q", PackageName: "path-parse", Ecosystem: EcosystemNPM, Severity: SeverityMedium, Summary: "path-parse prototype pollution < 1.0.7", FixedIn: "1.0.7"},
		// async
		{ID: "CVE-2021-43138", PackageName: "async", Ecosystem: EcosystemNPM, Severity: SeverityMedium, Summary: "async prototype pollution < 2.6.4, 3.2.2", FixedIn: "3.2.2"},
		// undici
		{ID: "CVE-2024-30260", PackageName: "undici", Ecosystem: EcosystemNPM, Severity: SeverityHigh, Summary: "undici request smuggling < 6.6.1", FixedIn: "6.6.1"},
		// cookie
		{ID: "CVE-2024-47764", PackageName: "cookie", Ecosystem: EcosystemNPM, Severity: SeverityMedium, Summary: "cookie < 0.7.0 DoS", FixedIn: "0.7.0"},
		// cross-spawn
		{ID: "CVE-2024-21538", PackageName: "cross-spawn", Ecosystem: EcosystemNPM, Severity: SeverityCritical, Summary: "cross-spawn command injection < 7.0.5", FixedIn: "7.0.5"},
	}
}
