package depintel

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type parsedFile struct {
	Path    string
	Content string
}

type parserFunc func(pf parsedFile) ([]Package, error)

var parsers = map[string]parserFunc{
	"package.json":     parsePackageJSON,
	"go.mod":           parseGoMod,
	"requirements.txt": parseRequirementsTxt,
	"Gemfile":          parseGemfile,
	"Gemfile.lock":     parseGemfileLock,
	"Cargo.toml":       parseCargoToml,
	"Cargo.lock":       parseCargoLock,
	"composer.json":    parseComposerJSON,
	"composer.lock":    parseComposerLock,
	"pom.xml":          parsePomXML,
}

func parsePackageJSON(pf parsedFile) ([]Package, error) {
	var data struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal([]byte(pf.Content), &data); err != nil {
		return nil, err
	}
	var pkgs []Package
	for name, ver := range data.Dependencies {
		pkgs = append(pkgs, Package{Name: name, Version: ParseVersion(ver), Ecosystem: EcosystemNPM, Source: pf.Path})
	}
	for name, ver := range data.DevDependencies {
		pkgs = append(pkgs, Package{Name: name, Version: ParseVersion(ver), Ecosystem: EcosystemNPM, Source: pf.Path})
	}
	return pkgs, nil
}

func parseGoMod(pf parsedFile) ([]Package, error) {
	scanner := bufio.NewScanner(strings.NewReader(pf.Content))
	var pkgs []Package
	inBlock := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "module ") || strings.HasPrefix(trimmed, "go ") {
			continue
		}

		if strings.HasPrefix(trimmed, "require (") || trimmed == "require (" {
			inBlock = true
			continue
		}
		if trimmed == ")" {
			inBlock = false
			continue
		}

		if strings.HasPrefix(trimmed, "require ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 3 && parts[0] == "require" {
				skipGoModDep(&pkgs, parts[1], parts[2])
			}
			continue
		}

		if inBlock {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				skipGoModDep(&pkgs, parts[0], parts[1])
			}
		}
	}
	return pkgs, scanner.Err()
}

func skipGoModDep(pkgs *[]Package, name, ver string) {
	if strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") || name == "go" {
		return
	}
	if strings.HasSuffix(ver, "+incompatible") {
		ver = strings.TrimSuffix(ver, "+incompatible")
	}
	*pkgs = append(*pkgs, Package{Name: name, Version: ver, Ecosystem: EcosystemGo, Source: ""})
}

var pipReqRe = regexp.MustCompile(`(?i)^([a-zA-Z0-9_.-]+)\s*(==|>=|<=|!=|~=|>|<)\s*([a-zA-Z0-9.*_-]+)`)

func parseRequirementsTxt(pf parsedFile) ([]Package, error) {
	scanner := bufio.NewScanner(strings.NewReader(pf.Content))
	var pkgs []Package
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		matches := pipReqRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		pkgs = append(pkgs, Package{
			Name:      strings.TrimSpace(matches[1]),
			Version:   strings.TrimSpace(matches[3]),
			Ecosystem: EcosystemPyPI,
			Source:    pf.Path,
		})
	}
	return pkgs, scanner.Err()
}

var gemfileRe = regexp.MustCompile(`(?i)^\s*gem\s+["']([a-zA-Z0-9_-]+)["'](?:\s*,\s*["']([~<>=! ]*[0-9a-zA-Z.]+)["'])?`)

func parseGemfile(pf parsedFile) ([]Package, error) {
	scanner := bufio.NewScanner(strings.NewReader(pf.Content))
	var pkgs []Package
	for scanner.Scan() {
		line := scanner.Text()
		matches := gemfileRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		ver := ""
		if len(matches) > 2 {
			ver = ParseVersion(strings.TrimSpace(matches[2]))
		}
		pkgs = append(pkgs, Package{
			Name:      strings.TrimSpace(matches[1]),
			Version:   ver,
			Ecosystem: EcosystemRubyGems,
			Source:    pf.Path,
		})
	}
	return pkgs, scanner.Err()
}

var gemfileLockGemRe = regexp.MustCompile(`^\s{4}([a-zA-Z0-9_-]+)\s+\(([0-9a-zA-Z.]+)`)

func parseGemfileLock(pf parsedFile) ([]Package, error) {
	scanner := bufio.NewScanner(strings.NewReader(pf.Content))
	var pkgs []Package
	inSpecs := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "GEM") {
			inSpecs = true
			continue
		}
		if !inSpecs {
			continue
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "  specs:") {
			continue
		}
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
			continue
		}
		if strings.HasPrefix(line, "PLATFORMS") || strings.HasPrefix(line, "DEPENDENCIES") || strings.HasPrefix(line, "BUNDLED WITH") {
			break
		}
		matches := gemfileLockGemRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		pkgs = append(pkgs, Package{
			Name:      strings.TrimSpace(matches[1]),
			Version:   strings.TrimSpace(matches[2]),
			Ecosystem: EcosystemRubyGems,
			Source:    pf.Path,
		})
	}
	return pkgs, scanner.Err()
}

var cargoTomlDepRe = regexp.MustCompile(`(?i)^([a-zA-Z0-9_-]+)\s*=\s*["']([^"']+)["']`)
var cargoTomlInlineRe = regexp.MustCompile(`(?i)^([a-zA-Z0-9_-]+)\s*=\s*\{`)

func parseCargoToml(pf parsedFile) ([]Package, error) {
	scanner := bufio.NewScanner(strings.NewReader(pf.Content))
	var pkgs []Package
	inDeps := false
	inTarget := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[package]") {
			continue
		}
		if strings.HasPrefix(line, "[dependencies]") {
			inDeps = true
			continue
		}
		if strings.HasPrefix(line, "[dev-dependencies]") {
			inDeps = true
			continue
		}
		if strings.HasPrefix(line, "[build-dependencies]") {
			inDeps = true
			continue
		}
		if strings.HasPrefix(line, "[target") {
			inTarget = true
			inDeps = false
			continue
		}
		if inTarget && strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "[target") {
			inTarget = false
		}
		if strings.HasPrefix(line, "[") {
			inDeps = false
			continue
		}
		if !inDeps {
			continue
		}
		if m := cargoTomlInlineRe.FindStringSubmatch(line); m != nil {
			if ver := extractCargoInlineVersion(line); ver != "" {
				pkgs = append(pkgs, Package{
					Name:      strings.TrimSpace(m[1]),
					Version:   ver,
					Ecosystem: EcosystemCrates,
					Source:    pf.Path,
				})
			}
			continue
		}
		matches := cargoTomlDepRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		ver := ParseVersion(matches[2])
		pkgs = append(pkgs, Package{
			Name:      strings.TrimSpace(matches[1]),
			Version:   ver,
			Ecosystem: EcosystemCrates,
			Source:    pf.Path,
		})
	}
	return pkgs, scanner.Err()
}

func extractCargoInlineVersion(line string) string {
	idx := strings.Index(line, "version")
	if idx == -1 {
		return ""
	}
	rest := line[idx:]
	start := strings.Index(rest, `"`)
	if start == -1 {
		return ""
	}
	rest = rest[start+1:]
	end := strings.Index(rest, `"`)
	if end == -1 {
		return ""
	}
	return ParseVersion(rest[:end])
}

var cargoLockPkgRe = regexp.MustCompile(`^\[\[package\]\]$`)
var cargoLockNameRe = regexp.MustCompile(`^name\s*=\s*"([^"]+)"`)
var cargoLockVerRe = regexp.MustCompile(`^version\s*=\s*"([^"]+)"`)

func parseCargoLock(pf parsedFile) ([]Package, error) {
	scanner := bufio.NewScanner(strings.NewReader(pf.Content))
	var pkgs []Package
	var current *Package
	for scanner.Scan() {
		line := scanner.Text()
		if cargoLockPkgRe.MatchString(line) {
			if current != nil && current.Name != "" {
				pkgs = append(pkgs, *current)
			}
			current = &Package{Ecosystem: EcosystemCrates, Source: pf.Path}
			continue
		}
		if current == nil {
			continue
		}
		if m := cargoLockNameRe.FindStringSubmatch(line); m != nil {
			current.Name = m[1]
		} else if m := cargoLockVerRe.FindStringSubmatch(line); m != nil {
			current.Version = m[1]
		}
	}
	if current != nil && current.Name != "" {
		pkgs = append(pkgs, *current)
	}
	return pkgs, scanner.Err()
}

var composerJSONRe = regexp.MustCompile(`"([a-zA-Z0-9_/.-]+)"\s*:\s*"([^"]+)"`)

func parseComposerJSON(pf parsedFile) ([]Package, error) {
	var data struct {
		Require    map[string]string `json:"require"`
		RequireDev map[string]string `json:"require-dev"`
	}
	if err := json.Unmarshal([]byte(pf.Content), &data); err != nil {
		return nil, err
	}
	var pkgs []Package
	for name, ver := range data.Require {
		if name == "php" || name == "ext-*" {
			continue
		}
		pkgs = append(pkgs, Package{
			Name:      name,
			Version:   ParseVersion(ver),
			Ecosystem: EcosystemPackagist,
			Source:    pf.Path,
		})
	}
	for name, ver := range data.RequireDev {
		if name == "php" || name == "ext-*" {
			continue
		}
		pkgs = append(pkgs, Package{
			Name:      name,
			Version:   ParseVersion(ver),
			Ecosystem: EcosystemPackagist,
			Source:    pf.Path,
		})
	}
	return pkgs, nil
}

type composerLockPkg struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
type composerLock struct {
	Packages []composerLockPkg `json:"packages"`
}

func parseComposerLock(pf parsedFile) ([]Package, error) {
	var data composerLock
	if err := json.Unmarshal([]byte(pf.Content), &data); err != nil {
		return nil, err
	}
	var pkgs []Package
	for _, p := range data.Packages {
		pkgs = append(pkgs, Package{
			Name:      p.Name,
			Version:   ParseVersion(p.Version),
			Ecosystem: EcosystemPackagist,
			Source:    pf.Path,
		})
	}
	return pkgs, nil
}

var pomXMLPropKeyRe = regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9_.-]*)>([^<]+)</`)
var pomXMLArtifactRe = regexp.MustCompile(`<groupId>([^<]+)</groupId>\s*<artifactId>([^<]+)</artifactId>`)
var pomXMLVersionRe = regexp.MustCompile(`<version>([^<]+)</version>`)
var pomXMLScopeRe = regexp.MustCompile(`<scope>([^<]+)</scope>`)

func parsePomXML(pf parsedFile) ([]Package, error) {
	content := pf.Content

	props := make(map[string]string)
	if propBlock := extractBetween(content, "<properties>", "</properties>"); propBlock != "" {
		for _, m := range pomXMLPropKeyRe.FindAllStringSubmatch(propBlock, -1) {
			if len(m) > 2 {
				props[m[1]] = m[2]
			}
		}
	}

	var pkgs []Package
	deps := splitPOMDependencies(content)
	for _, dep := range deps {
		gav := pomXMLArtifactRe.FindStringSubmatch(dep)
		if gav == nil {
			continue
		}
		groupId := gav[1]
		artifactId := gav[2]
		verMatch := pomXMLVersionRe.FindStringSubmatch(dep)
		if verMatch == nil {
			continue
		}

		if strings.HasPrefix(verMatch[1], "${") && strings.HasSuffix(verMatch[1], "}") {
			propName := verMatch[1][2 : len(verMatch[1])-1]
			if resolved, ok := props[propName]; ok {
				verMatch[1] = resolved
			}
		}

		scopeMatch := pomXMLScopeRe.FindStringSubmatch(dep)
		scope := ""
		if len(scopeMatch) > 1 {
			scope = scopeMatch[1]
		}
		if scope == "test" || scope == "provided" {
			continue
		}

		pkgs = append(pkgs, Package{
			Name:      groupId + ":" + artifactId,
			Version:   verMatch[1],
			Ecosystem: EcosystemMaven,
			Source:    pf.Path,
		})
	}
	return pkgs, nil
}

func extractBetween(content, open, close string) string {
	start := strings.Index(content, open)
	if start == -1 {
		return ""
	}
	rest := content[start+len(open):]
	end := strings.Index(rest, close)
	if end == -1 {
		return ""
	}
	return rest[:end]
}

func splitPOMDependencies(content string) []string {
	start := strings.Index(content, "<dependencies>")
	if start == -1 {
		return nil
	}
	end := strings.Index(content[start:], "</dependencies>")
	if end == -1 {
		return nil
	}
	block := content[start : start+end+15]
	var deps []string
	for {
		ds := strings.Index(block, "<dependency>")
		if ds == -1 {
			break
		}
		de := strings.Index(block[ds:], "</dependency>")
		if de == -1 {
			break
		}
		deps = append(deps, block[ds:ds+de+13])
		block = block[ds+de+13:]
	}
	return deps
}

var ecosystemAndLockFiles = map[string]struct {
	ecosystem Ecosystem
	lockFiles []string
	manifestFiles []string
}{
	"node_modules":       {EcosystemNPM, []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml"}, []string{"package.json"}},
	"vendor":             {EcosystemPackagist, []string{"composer.lock"}, []string{"composer.json"}},
	"Gemfile.lock":       {EcosystemRubyGems, []string{"Gemfile.lock"}, []string{"Gemfile"}},
	"Cargo.lock":         {EcosystemCrates, []string{"Cargo.lock"}, []string{"Cargo.toml"}},
}

func detectEcosystemByFiles(files []string) (Ecosystem, string) {
	for _, lock := range []struct {
		lockFile string
		eco      Ecosystem
	}{
		{"package-lock.json", EcosystemNPM},
		{"yarn.lock", EcosystemNPM},
		{"pnpm-lock.yaml", EcosystemNPM},
		{"Cargo.lock", EcosystemCrates},
		{"Gemfile.lock", EcosystemRubyGems},
		{"composer.lock", EcosystemPackagist},
	} {
		if containsFile(files, lock.lockFile) {
			return lock.eco, lock.lockFile
		}
	}
	for _, mf := range []struct {
		manifest string
		eco      Ecosystem
	}{
		{"package.json", EcosystemNPM},
		{"go.mod", EcosystemGo},
		{"requirements.txt", EcosystemPyPI},
		{"Cargo.toml", EcosystemCrates},
		{"Gemfile", EcosystemRubyGems},
		{"composer.json", EcosystemPackagist},
		{"pom.xml", EcosystemMaven},
	} {
		if containsFile(files, mf.manifest) {
			return mf.eco, mf.manifest
		}
	}
	return "", ""
}

func containsFile(files []string, target string) bool {
	for _, f := range files {
		if filepath.Base(f) == target {
			return true
		}
	}
	return false
}

func FindDepFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == ".next" || name == "__pycache__" || name == ".cache" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if _, ok := parsers[name]; ok {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
