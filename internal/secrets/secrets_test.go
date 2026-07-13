package secrets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nasij/nasij/internal/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanText_AWS(t *testing.T) {
	text := `aws_access_key_id = AKIAIOSFODNN7EXAMPLE`
	result := NewScanner().ScanText(text, "test")
	findings := result.Findings
	require.NotEmpty(t, findings)

	hasAccessKey := false
	for _, f := range findings {
		if f.Provider == "AWS" && f.Key == "AWS Access Key ID" {
			hasAccessKey = true
			assert.Contains(t, f.Match, "AKIA")
		}
	}
	assert.True(t, hasAccessKey, "should detect AWS Access Key ID")
}

func TestScanText_AWSSecretKey(t *testing.T) {
	text := `AWS_SECRET_ACCESS_KEY = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"`
	result := NewScanner().ScanText(text, "test")
	hasSecret := false
	for _, f := range result.Findings {
		if f.Provider == "AWS" && f.Key == "AWS Secret Access Key" {
			hasSecret = true
			assert.Equal(t, SeverityCritical, f.Severity)
		}
	}
	assert.True(t, hasSecret)
}

func TestScanText_AzureConnectionString(t *testing.T) {
	text := `DefaultEndpointsProtocol=https;AccountName=mystore;AccountKey=abc123==;BlobEndpoint=https://mystore.blob.core.windows.net/`
	result := NewScanner().ScanText(text, "test")
	assert.NotEmpty(t, result.Findings)
	hasConnStr := false
	for _, f := range result.Findings {
		if f.Provider == "Azure" {
			hasConnStr = true
			break
		}
	}
	assert.True(t, hasConnStr)
}

func TestScanText_AzureClientSecret(t *testing.T) {
	text := `client_secret = "abc123def456ghijklmnopqrstuvwxyzABCDEFGH"`
	result := NewScanner().ScanText(text, "test")
	hasSecret := false
	for _, f := range result.Findings {
		if f.Provider == "Azure" && f.Key == "Azure Client Secret" {
			hasSecret = true
			break
		}
	}
	assert.True(t, hasSecret)
}

func TestScanText_GCPAPIKey(t *testing.T) {
	text := `const API_KEY = "AIzaSyA1234567890abcdefghijklmnopqrstuvwxyz";`
	result := NewScanner().ScanText(text, "test")
	hasKey := false
	for _, f := range result.Findings {
		if f.Provider == "GCP" || f.Provider == "Google" {
			hasKey = true
			break
		}
	}
	assert.True(t, hasKey)
}

func TestScanText_GCPOAuthToken(t *testing.T) {
	text := `access_token: ya29.abcdefghijklmnopqrstuvwxyz1234567890`
	result := NewScanner().ScanText(text, "test")
	hasToken := false
	for _, f := range result.Findings {
		if f.Provider == "GCP" {
			hasToken = true
			break
		}
	}
	assert.True(t, hasToken)
}

func TestScanText_JWT(t *testing.T) {
	text := `Authorization: Bearer eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyMTIzIn0.signature`
	result := NewScanner().ScanText(text, "test")
	hasJWT := false
	for _, f := range result.Findings {
		if f.SecretType == TypeJWT {
			hasJWT = true
			break
		}
	}
	assert.True(t, hasJWT)
}

func TestScanText_OAuthClientSecret(t *testing.T) {
	text := `client_secret=abc123def456ghijklmnopqrstuvwxyz`
	result := NewScanner().ScanText(text, "test")
	hasOAuth := false
	for _, f := range result.Findings {
		if f.Provider == "OAuth" {
			hasOAuth = true
			break
		}
	}
	assert.True(t, hasOAuth)
}

func TestScanText_GitHubPAT(t *testing.T) {
	text := `token = ghp_abcdefghijklmnopqrstuvwxyz1234567890`
	result := NewScanner().ScanText(text, "test")
	hasGitHub := false
	for _, f := range result.Findings {
		if f.Provider == "GitHub" {
			hasGitHub = true
			assert.Equal(t, SeverityCritical, f.Severity)
			break
		}
	}
	assert.True(t, hasGitHub)
}

func TestScanText_GitHubFineGrained(t *testing.T) {
	text := `GITHUB_TOKEN: github_pat_11abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklmnopqrstuvwxyz12345678901234567890ab`
	result := NewScanner().ScanText(text, "test")
	hasPAT := false
	for _, f := range result.Findings {
		if f.Provider == "GitHub" && f.Key == "GitHub Fine-Grained PAT" {
			hasPAT = true
			break
		}
	}
	assert.True(t, hasPAT)
}

func TestScanText_SlackBotToken(t *testing.T) {
	val := "xoxb-" + "000000000000-0000000000000-aaaaaaaaaaaaaa"
	text := "SLACK_BOT_TOKEN = " + val
	result := NewScanner().ScanText(text, "test")
	hasSlack := false
	for _, f := range result.Findings {
		if f.Provider == "Slack" {
			hasSlack = true
			break
		}
	}
	assert.True(t, hasSlack)
}

func TestScanText_StripeKey(t *testing.T) {
	val := "sk_live_" + "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	text := `stripe_secret_key = "` + val + `"`
	result := NewScanner().ScanText(text, "test")
	hasStripe := false
	for _, f := range result.Findings {
		if f.Provider == "Stripe" {
			hasStripe = true
			assert.Equal(t, SeverityCritical, f.Severity)
			break
		}
	}
	assert.True(t, hasStripe)
}

func TestScanText_OpenAIKey(t *testing.T) {
	text := `OPENAI_API_KEY="sk-proj-abcdefghijklmnopqrstuvwxyz1234567890"`
	result := NewScanner().ScanText(text, "test")
	hasOpenAI := false
	for _, f := range result.Findings {
		if f.Provider == "OpenAI" {
			hasOpenAI = true
			assert.Equal(t, SeverityCritical, f.Severity)
			break
		}
	}
	assert.True(t, hasOpenAI)
}

func TestScanText_PrivateKey(t *testing.T) {
	text := `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA...
-----END RSA PRIVATE KEY-----`
	result := NewScanner().ScanText(text, "test")
	hasKey := false
	for _, f := range result.Findings {
		if f.SecretType == TypePrivateKey {
			hasKey = true
			break
		}
	}
	assert.True(t, hasKey)
}

func TestScanText_MongoDB(t *testing.T) {
	text := `mongodb+srv://admin:password123@cluster0.mongodb.net/myapp`
	result := NewScanner().ScanText(text, "test")
	hasMongo := false
	for _, f := range result.Findings {
		if f.Provider == "MongoDB" {
			hasMongo = true
			break
		}
	}
	assert.True(t, hasMongo)
}

func TestScanText_PasswordInURI(t *testing.T) {
	text := `postgres://user:supersecretpass123@localhost:5432/db`
	result := NewScanner().ScanText(text, "test")
	hasPass := false
	for _, f := range result.Findings {
		if f.SecretType == TypePassword {
			hasPass = true
			break
		}
	}
	assert.True(t, hasPass)
}

func TestScanText_BearerToken(t *testing.T) {
	text := `Authorization: Bearer abcdefghijklmnopqrstuvwxyz1234567890abcdef`
	result := NewScanner().ScanText(text, "test")
	hasBearer := false
	for _, f := range result.Findings {
		if f.Provider == "Generic" && f.Key == "Bearer Token" {
			hasBearer = true
			break
		}
	}
	assert.True(t, hasBearer)
}

func TestScanText_Twilio(t *testing.T) {
	text := `TWILIO_AUTH_TOKEN = "abcdefghijklmnopqrstuvwxyz123456"`
	result := NewScanner().ScanText(text, "test")
	hasTwilio := false
	for _, f := range result.Findings {
		if f.Provider == "Twilio" {
			hasTwilio = true
			break
		}
	}
	assert.True(t, hasTwilio)
}

func TestScanText_SendGrid(t *testing.T) {
	text := `SENDGRID_API_KEY = "SG.aaaaaaaaaaaaaaaaaaaaaa.bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"`
	result := NewScanner().ScanText(text, "test")
	hasSG := false
	for _, f := range result.Findings {
		if f.Provider == "SendGrid" {
			hasSG = true
			break
		}
	}
	assert.True(t, hasSG)
}



func TestScanText_Empty(t *testing.T) {
	result := NewScanner().ScanText("", "empty")
	assert.Empty(t, result.Findings)
}

func TestScanText_NoSecrets(t *testing.T) {
	result := NewScanner().ScanText("const x = 42; console.log('hello world');", "test")
	assert.Empty(t, result.Findings)
}

func TestScanText_FalsePositiveFiltering(t *testing.T) {
	text := `api_key = "your-actual-api-key-here"` // has "your-" prefix
	result := NewScanner().ScanText(text, "test")
	for _, f := range result.Findings {
		if f.Provider == "Generic" && f.Key == "Generic API Key (assignment)" {
			t.Errorf("should have filtered false positive: %+v", f)
		}
	}
}

func TestScanText_FalsePositiveTest(t *testing.T) {
	text := `password = "test_password_123"` // contains "test"
	result := NewScanner().ScanText(text, "test")
	for _, f := range result.Findings {
		if f.SecretType == TypePassword {
			t.Logf("found (may be false positive): %s = %s", f.Key, f.Match)
		}
	}
}

func TestScanner_MinSeverity(t *testing.T) {
	text := `AWS_SECRET_ACCESS_KEY = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"`
	result := NewScanner(WithMinSeverity(SeverityCritical)).ScanText(text, "test")
	require.NotEmpty(t, result.Findings)

	result2 := NewScanner(WithMinSeverity(SeverityHigh)).ScanText(text, "test")
	require.NotEmpty(t, result2.Findings)
}

func TestScanner_MaxFindings(t *testing.T) {
	text := `AWS_SECRET_ACCESS_KEY = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"`
	result := NewScanner(WithMaxFindings(1)).ScanText(text, "test")
	assert.Len(t, result.Findings, 1)
}

func TestScanRuntime_Requests(t *testing.T) {
	r := &runtime.Result{URL: "https://api.example.com"}
	r.Requests = []runtime.RequestRecord{
		{
			URL:    "https://api.example.com/data",
			Method: "GET",
			Headers: map[string]string{
				"Authorization": "Bearer sk-proj-abcdefghijklmnopqrstuvwxyz123456",
			},
		},
	}
	result := NewScanner().ScanRuntime(r)
	require.NotEmpty(t, result.Findings)
	hasOpenAI := false
	for _, f := range result.Findings {
		if f.Provider == "OpenAI" {
			hasOpenAI = true
			break
		}
	}
	assert.True(t, hasOpenAI)
}

func TestScanRuntime_Cookies(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Cookies = []runtime.CookieRecord{
		{Name: "session", Value: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.signature"},
	}
	result := NewScanner().ScanRuntime(r)
	hasJWT := false
	for _, f := range result.Findings {
		if f.SecretType == TypeJWT {
			hasJWT = true
			break
		}
	}
	assert.True(t, hasJWT)
}

func TestScanRuntime_LocalStorage(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.LocalStorage = []runtime.StorageRecord{
		{Key: "github_token", Value: "ghp_abcdefghijklmnopqrstuvwxyz1234567890"},
	}
	result := NewScanner().ScanRuntime(r)
	hasGH := false
	for _, f := range result.Findings {
		if f.Provider == "GitHub" {
			hasGH = true
			break
		}
	}
	assert.True(t, hasGH)
}

func TestScanRuntime_SessionStorage(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.SessionStorage = []runtime.StorageRecord{
		{Key: "access_token", Value: "sk_live_" + "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
	}
	result := NewScanner().ScanRuntime(r)
	hasStripe := false
	for _, f := range result.Findings {
		if f.Provider == "Stripe" {
			hasStripe = true
			break
		}
	}
	assert.True(t, hasStripe)
}

func TestScanRuntime_Empty(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	result := NewScanner().ScanRuntime(r)
	assert.Empty(t, result.Findings)
}

func TestScanFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.env")
	content := `AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
SECRET_KEY=supersecretvalue12345`
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	result, err := NewScanner().ScanFile(path)
	require.NoError(t, err)
	require.NotEmpty(t, result.Findings)
	assert.Equal(t, path, result.Target)
}

func TestScanFile_NonExistent(t *testing.T) {
	_, err := NewScanner().ScanFile("/nonexistent/path")
	assert.Error(t, err)
}

func TestShannonEntropy(t *testing.T) {
	assert.Equal(t, 0.0, shannonEntropy(""))
	assert.Equal(t, 0.0, shannonEntropy("aaaaaa"))
	e := shannonEntropy("abcdefghijklmnop")
	assert.Greater(t, e, 3.5)
	assert.Less(t, e, 4.5)

	high := shannonEntropy("aB3dEfGh1jKlMnOpQrStUvWxYz")
	assert.Greater(t, high, 4.0)
}

func TestHighEntropy(t *testing.T) {
	assert.False(t, highEntropy(""))
	assert.False(t, highEntropy("aaaaaaaabbbbbbbbccccccccddddddd"))
	e := shannonEntropy("aB3dEfGh1jKlMnOpQrStUvWxYz1234567890")
	t.Logf("entropy: %f", e)
}

func TestCountBySeverity(t *testing.T) {
	r := &ScanResult{
		Findings: []Finding{
			{Severity: SeverityHigh, Provider: "AWS"},
			{Severity: SeverityCritical, Provider: "AWS"},
			{Severity: SeverityHigh, Provider: "GitHub"},
		},
	}
	counts := r.CountBySeverity()
	assert.Equal(t, 2, counts[SeverityHigh])
	assert.Equal(t, 1, counts[SeverityCritical])
}

func TestCountByType(t *testing.T) {
	r := &ScanResult{
		Findings: []Finding{
			{SecretType: TypeAWS, Provider: "AWS"},
			{SecretType: TypeAWS, Provider: "AWS"},
			{SecretType: TypeJWT, Provider: "JWT"},
		},
	}
	counts := r.CountByType()
	assert.Equal(t, 2, counts[TypeAWS])
	assert.Equal(t, 1, counts[TypeJWT])
}

func TestDedupFindings(t *testing.T) {
	findings := []Finding{
		{SecretType: TypeAWS, Match: "AKIAIOSFODNN7EXAMPLE", Source: "file1"},
		{SecretType: TypeAWS, Match: "AKIAIOSFODNN7EXAMPLE", Source: "file1"},
		{SecretType: TypeJWT, Match: "eyJ...", Source: "file2"},
	}
	result := dedupFindings(findings)
	assert.Len(t, result, 2)
}

func TestProviderCoverage(t *testing.T) {
	providers := make(map[string]bool)
	for _, p := range patterns {
		providers[p.Provider] = true
	}
	expected := []string{"AWS", "Azure", "GCP", "JWT", "OAuth", "GitHub", "Slack",
		"Stripe", "OpenAI", "Twilio", "Google", "SendGrid", "Mailgun",
		"Mailchimp", "Heroku", "DigitalOcean", "npm", "SSH", "PGP",
		"PKCS", "MongoDB", "PostgreSQL", "MySQL", "Redis", "SQLite", "Generic"}
	for _, e := range expected {
		assert.True(t, providers[e], "missing provider: %s", e)
	}
}

func TestSeverityString(t *testing.T) {
	assert.Equal(t, "info", SeverityInfo.String())
	assert.Equal(t, "low", SeverityLow.String())
	assert.Equal(t, "medium", SeverityMedium.String())
	assert.Equal(t, "high", SeverityHigh.String())
	assert.Equal(t, "critical", SeverityCritical.String())
	assert.Equal(t, "unknown", Severity(99).String())
}

func TestScanText_PasswordAssignment(t *testing.T) {
	text := `DB_PASSWORD = "MyS3cur3P@ssw0rd!2024"`
	result := NewScanner().ScanText(text, "config")
	hasPass := false
	for _, f := range result.Findings {
		if f.SecretType == TypePassword {
			hasPass = true
			break
		}
	}
	assert.True(t, hasPass)
}

func TestScanText_Mailgun(t *testing.T) {
	val := "key-" + "00000000000000000000000000000000"
	text := `MAILGUN_API_KEY = "` + val + `"`
	result := NewScanner().ScanText(text, "test")
	hasMG := false
	for _, f := range result.Findings {
		if f.Provider == "Mailgun" {
			hasMG = true
			break
		}
	}
	assert.True(t, hasMG)
}

func TestScanText_DigitalOcean(t *testing.T) {
	val := "dop_v1_" + "0000000000000000000000000000000000000000000000000000000000000000"
	text := `DIGITALOCEAN_TOKEN = "` + val + `"`
	result := NewScanner().ScanText(text, "test")
	hasDO := false
	for _, f := range result.Findings {
		if f.Provider == "DigitalOcean" {
			hasDO = true
			break
		}
	}
	assert.True(t, hasDO)
}

func TestScanRuntime_MultipleSources(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Requests = []runtime.RequestRecord{
		{
			URL:    "https://example.com/login",
			Method: "POST",
			Headers: map[string]string{
				"Authorization": "Bearer ghx_abcdefghijklmnopqrstuvwxyz123456",
			},
		},
	}
	r.Cookies = []runtime.CookieRecord{
		{Name: "session", Value: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.signature"},
	}
	r.LocalStorage = []runtime.StorageRecord{
		{Key: "stripe_key", Value: "sk_live_" + "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
	}
	result := NewScanner().ScanRuntime(r)
	assert.NotEmpty(t, result.Findings)

	types := make(map[SecretType]bool)
	for _, f := range result.Findings {
		types[f.SecretType] = true
	}
	assert.Contains(t, types, TypeJWT)
	assert.Contains(t, types, TypeToken)
}
