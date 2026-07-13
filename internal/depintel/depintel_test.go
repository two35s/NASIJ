package depintel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePackageJSON(t *testing.T) {
	content := `{
		"dependencies": {
			"react": "^18.2.0",
			"lodash": "4.17.21",
			"axios": "~1.6.0"
		},
		"devDependencies": {
			"typescript": "^5.0.0",
			"jest": "29.5.0"
		}
	}`
	pkgs, err := parsePackageJSON(parsedFile{Path: "package.json", Content: content})
	require.NoError(t, err)
	require.Len(t, pkgs, 5)

	find := func(name string) *Package {
		for _, p := range pkgs {
			if p.Name == name {
				return &p
			}
		}
		return nil
	}

	p := find("react")
	require.NotNil(t, p)
	assert.Equal(t, "18.2.0", p.Version)
	assert.Equal(t, EcosystemNPM, p.Ecosystem)

	p = find("lodash")
	require.NotNil(t, p)
	assert.Equal(t, "4.17.21", p.Version)

	p = find("typescript")
	require.NotNil(t, p)
	assert.Equal(t, "5.0.0", p.Version)
	assert.Equal(t, EcosystemNPM, p.Ecosystem)
}

func TestParsePackageJSON_NoDeps(t *testing.T) {
	pkgs, err := parsePackageJSON(parsedFile{Path: "package.json", Content: `{}`})
	require.NoError(t, err)
	assert.Empty(t, pkgs)
}

func TestParseGoMod(t *testing.T) {
	content := `module github.com/example/project

go 1.21

require (
	github.com/gorilla/mux v1.8.1
	github.com/stretchr/testify v1.8.4
	github.com/rs/zerolog v1.31.0
)

require github.com/spf13/cobra v1.8.0
`
	pkgs, err := parseGoMod(parsedFile{Path: "go.mod", Content: content})
	require.NoError(t, err)
	require.Len(t, pkgs, 4)

	names := make(map[string]string)
	for _, p := range pkgs {
		names[p.Name] = p.Version
	}
	assert.Equal(t, "v1.8.1", names["github.com/gorilla/mux"])
	assert.Equal(t, "v1.8.4", names["github.com/stretchr/testify"])
	assert.Equal(t, "v1.31.0", names["github.com/rs/zerolog"])
	assert.Equal(t, "v1.8.0", names["github.com/spf13/cobra"])
}

func TestParseGoMod_ExcludeIndirect(t *testing.T) {
	content := `module example

go 1.21

require (
	github.com/foo/bar v1.0.0
	github.com/baz/qux v2.0.0 // indirect
)`
	pkgs, err := parseGoMod(parsedFile{Path: "go.mod", Content: content})
	require.NoError(t, err)
	require.Len(t, pkgs, 2)
}

func TestParseGoMod_NoDeps(t *testing.T) {
	content := `module example

go 1.21
`
	pkgs, err := parseGoMod(parsedFile{Content: content})
	require.NoError(t, err)
	assert.Empty(t, pkgs)
}

func TestParseRequirementsTxt(t *testing.T) {
	content := `flask==2.3.0
requests>=2.31.0
numpy~=1.24.0
Django<5.0.0
# comment
# -r other.txt
`
	pkgs, err := parseRequirementsTxt(parsedFile{Path: "requirements.txt", Content: content})
	require.NoError(t, err)
	require.Len(t, pkgs, 4)

	names := make(map[string]string)
	for _, p := range pkgs {
		names[p.Name] = p.Version
	}
	assert.Equal(t, "2.3.0", names["flask"])
	assert.Equal(t, "2.31.0", names["requests"])
	assert.Equal(t, "1.24.0", names["numpy"])
	assert.Equal(t, "5.0.0", names["Django"])
}

func TestParseRequirementsTxt_NoDeps(t *testing.T) {
	content := `# empty`
	pkgs, err := parseRequirementsTxt(parsedFile{Content: content})
	require.NoError(t, err)
	assert.Empty(t, pkgs)
}

func TestParseGemfile(t *testing.T) {
	content := `source "https://rubygems.org"

gem "rails", "~> 7.0.0"
gem "pg"
gem "puma", ">= 5.0"
gem "rspec", "3.12"
`
	pkgs, err := parseGemfile(parsedFile{Path: "Gemfile", Content: content})
	require.NoError(t, err)
	require.Len(t, pkgs, 4)

	names := make(map[string]string)
	for _, p := range pkgs {
		names[p.Name] = p.Version
	}
	assert.Equal(t, "7.0.0", names["rails"])
	assert.Equal(t, "", names["pg"])
	assert.Equal(t, "5.0", names["puma"])
	assert.Equal(t, "3.12", names["rspec"])
}

func TestParseGemfileLock(t *testing.T) {
	content := `GEM
  remote: https://rubygems.org/
  specs:
    actionpack (7.0.4)
      actionview (= 7.0.4)
    actionview (7.0.4)
    nokogiri (1.14.0)

PLATFORMS
  arm64-darwin

DEPENDENCIES
  rails (~> 7.0.0)

BUNDLED WITH
   2.4.0
`
	pkgs, err := parseGemfileLock(parsedFile{Path: "Gemfile.lock", Content: content})
	require.NoError(t, err)
	require.Len(t, pkgs, 3)

	names := make(map[string]string)
	for _, p := range pkgs {
		names[p.Name] = p.Version
	}
	assert.Equal(t, "7.0.4", names["actionpack"])
	assert.Equal(t, "7.0.4", names["actionview"])
	assert.Equal(t, "1.14.0", names["nokogiri"])
}

func TestParseCargoToml(t *testing.T) {
	content := `[package]
name = "example"
version = "0.1.0"

[dependencies]
serde = "1.0"
tokio = { version = "1.35", features = ["full"] }
reqwest = "0.11.20"

[dev-dependencies]
criterion = "0.5"

[build-dependencies]
cc = "1.0"
`
	pkgs, err := parseCargoToml(parsedFile{Path: "Cargo.toml", Content: content})
	require.NoError(t, err)
	require.Len(t, pkgs, 5)

	names := make(map[string]string)
	for _, p := range pkgs {
		names[p.Name] = p.Version
	}
	assert.Equal(t, "1.0", names["serde"])
	assert.Equal(t, "1.35", names["tokio"])
	assert.Equal(t, "0.11.20", names["reqwest"])
	assert.Equal(t, "0.5", names["criterion"])
	assert.Equal(t, "1.0", names["cc"])
}

func TestParseCargoLock(t *testing.T) {
	content := `[[package]]
name = "serde"
version = "1.0.188"

[[package]]
name = "tokio"
version = "1.35.0"
`
	pkgs, err := parseCargoLock(parsedFile{Path: "Cargo.lock", Content: content})
	require.NoError(t, err)
	require.Len(t, pkgs, 2)

	names := make(map[string]string)
	for _, p := range pkgs {
		names[p.Name] = p.Version
	}
	assert.Equal(t, "1.0.188", names["serde"])
	assert.Equal(t, "1.35.0", names["tokio"])
}

func TestParseComposerJSON(t *testing.T) {
	content := `{
		"require": {
			"laravel/framework": "^10.0",
			"guzzlehttp/guzzle": "7.8.0"
		},
		"require-dev": {
			"phpunit/phpunit": "^10.0"
		}
	}`
	pkgs, err := parseComposerJSON(parsedFile{Path: "composer.json", Content: content})
	require.NoError(t, err)
	require.Len(t, pkgs, 3)

	names := make(map[string]string)
	for _, p := range pkgs {
		names[p.Name] = p.Version
	}
	assert.Equal(t, "10.0", names["laravel/framework"])
	assert.Equal(t, "7.8.0", names["guzzlehttp/guzzle"])
	assert.Equal(t, "10.0", names["phpunit/phpunit"])
}

func TestParseComposerLock(t *testing.T) {
	content := `{
		"packages": [
			{"name": "laravel/framework", "version": "10.25.0"},
			{"name": "monolog/monolog", "version": "3.5.0"}
		]
	}`
	pkgs, err := parseComposerLock(parsedFile{Path: "composer.lock", Content: content})
	require.NoError(t, err)
	require.Len(t, pkgs, 2)

	names := make(map[string]string)
	for _, p := range pkgs {
		names[p.Name] = p.Version
	}
	assert.Equal(t, "10.25.0", names["laravel/framework"])
	assert.Equal(t, "3.5.0", names["monolog/monolog"])
}

func TestParsePomXML(t *testing.T) {
	content := `<project>
	<properties>
		<spring.version>5.3.30</spring.version>
	</properties>
	<dependencies>
		<dependency>
			<groupId>org.springframework</groupId>
			<artifactId>spring-core</artifactId>
			<version>${spring.version}</version>
		</dependency>
		<dependency>
			<groupId>com.google.guava</groupId>
			<artifactId>guava</artifactId>
			<version>32.1.0</version>
		</dependency>
		<dependency>
			<groupId>org.junit.jupiter</groupId>
			<artifactId>junit-jupiter</artifactId>
			<version>5.10.0</version>
			<scope>test</scope>
		</dependency>
	</dependencies>
</project>`
	pkgs, err := parsePomXML(parsedFile{Path: "pom.xml", Content: content})
	require.NoError(t, err)
	require.Len(t, pkgs, 2)

	names := make(map[string]string)
	for _, p := range pkgs {
		names[p.Name] = p.Version
	}
	assert.Equal(t, "5.3.30", names["org.springframework:spring-core"])
	assert.Equal(t, "32.1.0", names["com.google.guava:guava"])
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"^18.2.0", "18.2.0"},
		{"~1.6.0", "1.6.0"},
		{">= 5.0", "5.0"},
		{"=1.0.0", "1.0.0"},
		{"v1.8.1", "1.8.1"},
		{"1.0.0", "1.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, ParseVersion(tt.input))
		})
	}
}

func TestCompareVersions(t *testing.T) {
	assert.Equal(t, -1, compareVersions("1.0.0", "2.0.0"))
	assert.Equal(t, 1, compareVersions("2.0.0", "1.0.0"))
	assert.Equal(t, 0, compareVersions("1.0.0", "1.0.0"))
	assert.Equal(t, -1, compareVersions("1.0.0", "1.0.1"))
	assert.Equal(t, 1, compareVersions("1.0.1", "1.0.0"))
	assert.Equal(t, -1, compareVersions("1.0", "1.0.0"))
	assert.Equal(t, 1, compareVersions("1.0.0", "1.0"))
	assert.Equal(t, -1, compareVersions("v1.0.0", "v2.0.0"))
	assert.Equal(t, 0, compareVersions("v1.0.0", "1.0.0"))
}

func TestAdvisoryDB_Lookup(t *testing.T) {
	db := NewAdvisoryDB()

	vulns := db.Lookup(Package{Name: "lodash", Version: "4.17.19", Ecosystem: EcosystemNPM})
	require.NotEmpty(t, vulns)
	assert.Equal(t, "CVE-2020-8203", vulns[0].ID)
	assert.Equal(t, "4.17.20", vulns[0].FixedVersion)
	assert.Equal(t, SeverityHigh, vulns[0].Severity)

	vulns = db.Lookup(Package{Name: "lodash", Version: "4.17.21", Ecosystem: EcosystemNPM})
	assert.Empty(t, vulns)
}

func TestAdvisoryDB_LookupBatch(t *testing.T) {
	db := NewAdvisoryDB()
	pkgs := []Package{
		{Name: "lodash", Version: "4.17.19", Ecosystem: EcosystemNPM},
		{Name: "lodash", Version: "4.17.21", Ecosystem: EcosystemNPM},
		{Name: "express", Version: "4.17.2", Ecosystem: EcosystemNPM},
		{Name: "react-dom", Version: "18.2.0", Ecosystem: EcosystemNPM},
	}
	vulnMap := db.LookupBatch(pkgs)
	assert.NotEmpty(t, vulnMap[pkgs[0]])
	assert.Empty(t, vulnMap[pkgs[1]])
	assert.NotEmpty(t, vulnMap[pkgs[2]])
	assert.Empty(t, vulnMap[pkgs[3]])
}

func TestScanner_ScanDir_Simple(t *testing.T) {
	dir := t.TempDir()

	pkgJSON := `{"dependencies": {"lodash": "4.17.19", "express": "4.17.2"}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644))

	s := NewScanner(WithOSVLookup(false))
	result, err := s.ScanDir(dir)
	require.NoError(t, err)
	assert.Equal(t, 2, result.PackageCount())

	find := func(name string) *Package {
		for _, p := range result.Packages {
			if p.Name == name {
				return &p
			}
		}
		return nil
	}
	assert.Equal(t, "4.17.19", find("lodash").Version)
	assert.Equal(t, "4.17.2", find("express").Version)

	assert.NotEmpty(t, result.Vulnerabilities)
}

func TestScanner_ScanDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	s := NewScanner(WithOSVLookup(false))
	result, err := s.ScanDir(dir)
	require.NoError(t, err)
	assert.Empty(t, result.Packages)
	assert.Empty(t, result.Vulnerabilities)
}

func TestScanner_ScanDir_MultipleFiles(t *testing.T) {
	dir := t.TempDir()

	pkgJSON := `{"dependencies": {"lodash": "4.17.21"}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644))

	goMod := `module example
go 1.21
require github.com/gorilla/mux v1.8.1
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644))

	s := NewScanner(WithOSVLookup(false))
	result, err := s.ScanDir(dir)
	require.NoError(t, err)
	assert.Equal(t, 2, result.PackageCount())
}

func TestScanner_ScanFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package.json")
	content := `{"dependencies": {"lodash": "4.17.19"}}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	s := NewScanner(WithOSVLookup(false))
	result, err := s.ScanFile(path)
	require.NoError(t, err)
	assert.Equal(t, 1, result.PackageCount())
	assert.NotEmpty(t, result.Vulnerabilities)
}

func TestScanner_ScanFile_Unsupported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "readme.md")
	require.NoError(t, os.WriteFile(path, []byte("# hello"), 0644))

	s := NewScanner(WithOSVLookup(false))
	_, err := s.ScanFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no parser for")
}

func TestScanner_ScanText_GoMod(t *testing.T) {
	content := `module example
go 1.21
require github.com/gorilla/mux v1.8.1
`
	s := NewScanner(WithOSVLookup(false))
	result, err := s.ScanText(content, "go.mod", EcosystemGo)
	require.NoError(t, err)
	assert.Equal(t, 1, result.PackageCount())
}

func TestScanner_ScanText_Requirements(t *testing.T) {
	content := `flask==2.3.0
requests>=2.31.0
`
	s := NewScanner(WithOSVLookup(false))
	result, err := s.ScanText(content, "requirements.txt", EcosystemPyPI)
	require.NoError(t, err)
	assert.Equal(t, 2, result.PackageCount())
}

func TestScanner_AdvisoryDB_Lodash(t *testing.T) {
	s := NewScanner(WithOSVLookup(false), WithAdvisoryLookup(true))
	result, err := s.ScanText(`{"dependencies": {"lodash": "4.17.19"}}`, "package.json", EcosystemNPM)
	require.NoError(t, err)

	require.NotEmpty(t, result.Vulnerabilities)
	var cve2020Found bool
	for _, v := range result.Vulnerabilities {
		if v.ID == "CVE-2020-8203" {
			cve2020Found = true
			assert.Equal(t, SeverityHigh, v.Severity)
			assert.Equal(t, "4.17.20", v.FixedVersion)
		}
	}
	assert.True(t, cve2020Found, "should detect CVE-2020-8203 for lodash 4.17.19")
}

func TestScanner_DedupVulns(t *testing.T) {
	vulns := []Vulnerability{
		{ID: "CVE-2020-8203", AffectedPackage: "lodash", AffectedVersion: "4.17.19"},
		{ID: "CVE-2020-8203", AffectedPackage: "lodash", AffectedVersion: "4.17.19"},
	}
	result := dedupVulns(vulns)
	assert.Len(t, result, 1)
}

func TestSeverityFromString(t *testing.T) {
	assert.Equal(t, SeverityCritical, SeverityFromString("CRITICAL"))
	assert.Equal(t, SeverityHigh, SeverityFromString("HIGH"))
	assert.Equal(t, SeverityMedium, SeverityFromString("medium"))
	assert.Equal(t, SeverityLow, SeverityFromString("Low"))
	assert.Equal(t, SeverityUnknown, SeverityFromString("unknown"))
}

func TestSeverityString(t *testing.T) {
	assert.Equal(t, "CRITICAL", SeverityCritical.String())
	assert.Equal(t, "HIGH", SeverityHigh.String())
	assert.Equal(t, "MEDIUM", SeverityMedium.String())
	assert.Equal(t, "LOW", SeverityLow.String())
	assert.Equal(t, "UNKNOWN", SeverityUnknown.String())
}

func TestVulnerableCount(t *testing.T) {
	r := &DepResult{
		Vulnerabilities: []Vulnerability{
			{ID: "CVE-1"},
			{ID: "CVE-2"},
		},
	}
	assert.Equal(t, 2, r.VulnerableCount())
	assert.Equal(t, 0, r.PackageCount())
}

func TestFindDepFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "node_modules"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "node_modules", "package.json"), []byte("{}"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(""), 0644))

	files, err := FindDepFiles(dir)
	require.NoError(t, err)

	basenames := make(map[string]bool)
	for _, f := range files {
		basenames[filepath.Base(f)] = true
	}
	assert.True(t, basenames["package.json"], "should find package.json")
	assert.True(t, basenames["go.mod"], "should find go.mod")
	assert.False(t, basenames["config"], "should skip .git/config")
	assert.Len(t, basenames, 2, "should only find 2 dep files")
}

func TestFilterBySeverity(t *testing.T) {
	vulns := []Vulnerability{
		{ID: "CVE-1", Severity: SeverityCritical},
		{ID: "CVE-2", Severity: SeverityHigh},
		{ID: "CVE-3", Severity: SeverityLow},
	}
	s := NewScanner(WithOSVLookup(false), WithMinSeverity(SeverityHigh))
	filtered := s.filterBySeverity(vulns)
	require.Len(t, filtered, 2)
	assert.Equal(t, "CVE-1", filtered[0].ID)
	assert.Equal(t, "CVE-2", filtered[1].ID)
}

func TestDetectEcosystemByFiles(t *testing.T) {
	eco, file := detectEcosystemByFiles([]string{"package.json", "package-lock.json"})
	assert.Equal(t, EcosystemNPM, eco)
	assert.Equal(t, "package-lock.json", file)

	eco, file = detectEcosystemByFiles([]string{"requirements.txt"})
	assert.Equal(t, EcosystemPyPI, eco)
	assert.Equal(t, "requirements.txt", file)

	eco, _ = detectEcosystemByFiles(nil)
	assert.Equal(t, Ecosystem(""), eco)
}

func TestJSONSerialization(t *testing.T) {
	r := &DepResult{
		Target: "/test",
		Packages: []Package{
			{Name: "lodash", Version: "4.17.19", Ecosystem: EcosystemNPM, Source: "package.json"},
		},
		Vulnerabilities: []Vulnerability{
			{ID: "CVE-2020-8203", Severity: SeverityHigh, Summary: "test", FixedVersion: "4.17.20"},
		},
	}
	data, err := json.Marshal(r)
	require.NoError(t, err)

	var r2 DepResult
	require.NoError(t, json.Unmarshal(data, &r2))
	assert.Equal(t, r.Target, r2.Target)
	assert.Len(t, r2.Packages, 1)
	assert.Len(t, r2.Vulnerabilities, 1)
	assert.Equal(t, "CVE-2020-8203", r2.Vulnerabilities[0].ID)
}

func TestScanner_FileNotFound(t *testing.T) {
	s := NewScanner(WithOSVLookup(false))
	result, err := s.ScanFile("/nonexistent/path/package.json")
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestScanner_BadDir(t *testing.T) {
	s := NewScanner(WithOSVLookup(false))
	result, err := s.ScanDir("/nonexistent/dir")
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestScanner_SkipNodeModules(t *testing.T) {
	dir := t.TempDir()

	pkgJSON := `{"dependencies": {"lodash": "4.17.21"}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644))

	nmDir := filepath.Join(dir, "node_modules", "some-pkg")
	require.NoError(t, os.MkdirAll(nmDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(nmDir, "package.json"), []byte(`{}`), 0644))

	s := NewScanner(WithOSVLookup(false))
	result, err := s.ScanDir(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, result.PackageCount(), "should not include node_modules deps")
}

func TestScanner_ComposerJSON(t *testing.T) {
	dir := t.TempDir()
	content := `{"require": {"laravel/framework": "10.25.0"}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "composer.json"), []byte(content), 0644))

	s := NewScanner(WithOSVLookup(false))
	result, err := s.ScanDir(dir)
	require.NoError(t, err)
	require.Len(t, result.Packages, 1)
	assert.Equal(t, "laravel/framework", result.Packages[0].Name)
	assert.Equal(t, "10.25.0", result.Packages[0].Version)
	assert.Equal(t, EcosystemPackagist, result.Packages[0].Ecosystem)
}

func TestScanner_PomXML(t *testing.T) {
	dir := t.TempDir()
	content := `<project>
	<dependencies>
		<dependency>
			<groupId>com.google.guava</groupId>
			<artifactId>guava</artifactId>
			<version>32.1.0</version>
		</dependency>
	</dependencies>
</project>`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(content), 0644))

	s := NewScanner(WithOSVLookup(false))
	result, err := s.ScanDir(dir)
	require.NoError(t, err)
	require.Len(t, result.Packages, 1)
	assert.Equal(t, "com.google.guava:guava", result.Packages[0].Name)
	assert.Equal(t, "32.1.0", result.Packages[0].Version)
	assert.Equal(t, EcosystemMaven, result.Packages[0].Ecosystem)
}

func TestScanner_OSVClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/querybatch", r.URL.Path)

		var req osvQueryBatch
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Len(t, req.Queries, 1)
		assert.Equal(t, "lodash", req.Queries[0].Package.Name)
		assert.Equal(t, "npm", req.Queries[0].Package.Ecosystem)

		resp := osvResponse{
			Results: []osvQueryResult{
				{
					Vulns: []osvVuln{
						{
							ID:      "CVE-2020-8203",
							Aliases: []string{},
							Summary: "Prototype pollution in lodash",
							DatabaseSpecific: struct {
								Severity string `json:"severity,omitempty"`
							}{Severity: "HIGH"},
							Affected: []struct {
								Package struct {
									Name      string `json:"name"`
									Ecosystem string `json:"ecosystem"`
								} `json:"package"`
								Ranges []struct {
									Type   string `json:"type"`
									Events []struct {
										Introduced   string `json:"introduced,omitempty"`
										Fixed        string `json:"fixed,omitempty"`
										LastAffected string `json:"last_affected,omitempty"`
									} `json:"events"`
								} `json:"ranges"`
								Versions []string `json:"versions,omitempty"`
							}{
								{
									Package: struct {
										Name      string `json:"name"`
										Ecosystem string `json:"ecosystem"`
									}{Name: "lodash", Ecosystem: "npm"},
									Ranges: []struct {
										Type   string `json:"type"`
										Events []struct {
											Introduced   string `json:"introduced,omitempty"`
											Fixed        string `json:"fixed,omitempty"`
											LastAffected string `json:"last_affected,omitempty"`
										} `json:"events"`
									}{
										{
											Type: "SEMVER",
											Events: []struct {
												Introduced   string `json:"introduced,omitempty"`
												Fixed        string `json:"fixed,omitempty"`
												LastAffected string `json:"last_affected,omitempty"`
											}{
												{Introduced: "0"},
												{Fixed: "4.17.20"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &OSVClient{
		client:  server.Client(),
		baseURL: server.URL + "/v1",
	}

	vulns, err := client.Query(Package{Name: "lodash", Version: "4.17.19", Ecosystem: EcosystemNPM})
	require.NoError(t, err)
	require.NotEmpty(t, vulns)
	assert.Equal(t, "CVE-2020-8203", vulns[0].ID)
	assert.Equal(t, "lodash", vulns[0].AffectedPackage)
	assert.Equal(t, SeverityHigh, vulns[0].Severity)
	assert.Equal(t, "4.17.20", vulns[0].FixedVersion)
}

func TestOSVClient_EmptyQuery(t *testing.T) {
	client := NewOSVClient()
	vulns, err := client.Query(Package{Name: "unknown-pkg-12345", Version: "1.0.0", Ecosystem: EcosystemNPM})
	if err != nil {
		t.Skipf("OSV API not reachable: %v", err)
	}
	assert.Empty(t, vulns)
}

func TestOSVClient_BatchEmpty(t *testing.T) {
	client := NewOSVClient()
	result, err := client.QueryBatch(nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestOSVClient_BatchEmptyVersions(t *testing.T) {
	client := NewOSVClient()
	result, err := client.QueryBatch([]Package{{Name: "lodash", Ecosystem: EcosystemNPM}})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestCVSSMapping(t *testing.T) {
	assert.Equal(t, SeverityCritical, cvssToSeverity(9.0))
	assert.Equal(t, SeverityCritical, cvssToSeverity(10.0))
	assert.Equal(t, SeverityHigh, cvssToSeverity(7.0))
	assert.Equal(t, SeverityHigh, cvssToSeverity(8.9))
	assert.Equal(t, SeverityMedium, cvssToSeverity(4.0))
	assert.Equal(t, SeverityMedium, cvssToSeverity(6.9))
	assert.Equal(t, SeverityLow, cvssToSeverity(0.1))
	assert.Equal(t, SeverityLow, cvssToSeverity(3.9))
	assert.Equal(t, SeverityUnknown, cvssToSeverity(0))
}

func TestParseGoMod_Incompatible(t *testing.T) {
	content := `module example

go 1.21

require github.com/foo/bar v1.0.0+incompatible
`
	pkgs, err := parseGoMod(parsedFile{Path: "go.mod", Content: content})
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	assert.Equal(t, "v1.0.0", pkgs[0].Version)
}

func TestParseCargoToml_TargetSection(t *testing.T) {
	content := `[package]
name = "test"

[dependencies]
serde = "1.0"

[target.'cfg(windows)'.dependencies]
winapi = "0.3"
`
	pkgs, err := parseCargoToml(parsedFile{Path: "Cargo.toml", Content: content})
	require.NoError(t, err)
	assert.Equal(t, 1, len(pkgs), "should not parse target-specific deps")
}

func BenchmarkParsePackageJSON(b *testing.B) {
	content := `{"dependencies": {"lodash": "4.17.21", "express": "4.18.2", "react": "18.2.0", "vue": "3.3.0"}}`
	for i := 0; i < b.N; i++ {
		parsePackageJSON(parsedFile{Content: content})
	}
}

func ExampleParseVersion() {
	fmt.Println(ParseVersion("^18.2.0"))
	fmt.Println(ParseVersion("v1.8.1"))
	// Output:
	// 18.2.0
	// 1.8.1
}

func TestScanner_MinSeverityFiltering_Integration(t *testing.T) {
	dir := t.TempDir()
	content := `{"dependencies": {"lodash": "4.17.19", "express": "4.17.2"}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0644))

	s := NewScanner(WithOSVLookup(false), WithMinSeverity(SeverityHigh))
	result, err := s.ScanDir(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Vulnerabilities)
	for _, v := range result.Vulnerabilities {
		assert.GreaterOrEqual(t, v.Severity, SeverityHigh, "should only return HIGH+ vulns")
	}
}

func TestAdvisoryDB_AddCustom(t *testing.T) {
	db := NewAdvisoryDB()
	db.Add(Advisory{
		ID:          "TEST-001",
		PackageName: "my-pkg",
		Ecosystem:   EcosystemNPM,
		Severity:    SeverityCritical,
		Summary:     "test advisory",
		FixedIn:     "2.0.0",
	})

	vulns := db.Lookup(Package{Name: "my-pkg", Version: "1.0.0", Ecosystem: EcosystemNPM})
	require.NotEmpty(t, vulns)
	assert.Equal(t, "TEST-001", vulns[0].ID)
	assert.Equal(t, SeverityCritical, vulns[0].Severity)
}

func TestScanner_ScanFile_JsonOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package.json")
	content := `{"dependencies": {"lodash": "4.17.21"}}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	s := NewScanner(WithOSVLookup(false))
	result, err := s.ScanFile(path)
	require.NoError(t, err)

	data, err := json.Marshal(result)
	require.NoError(t, err)
	assert.Contains(t, string(data), "lodash")
	assert.Contains(t, string(data), "4.17.21")
	assert.Contains(t, string(data), "packages")
}
