package secrets

import "regexp"

type secretPattern struct {
	Type    SecretType
	Provider string
	Key     string
	Pattern *regexp.Regexp
	Severity Severity
}

var patterns []secretPattern

func init() {
	patterns = []secretPattern{

		{TypeAWS, "AWS", "AWS Access Key ID", regexp.MustCompile(`(AKIA[0-9A-Z]{16})`), SeverityHigh},
		{TypeAWS, "AWS", "AWS Secret Access Key", regexp.MustCompile(`(?i)(aws_secret_access_key|AWS_SECRET_ACCESS_KEY)\s*[:=]\s*['"]?([A-Za-z0-9\/+=]{40})`), SeverityCritical},
		{TypeAWS, "AWS", "AWS Session Token", regexp.MustCompile(`(?i)(aws_session_token|AWS_SESSION_TOKEN)\s*[:=]\s*['"]?([A-Za-z0-9\/+=]{100,})`), SeverityHigh},
		{TypeAWS, "AWS", "AWS Account ID", regexp.MustCompile(`(?i)(aws_account_id|AWS_ACCOUNT_ID)\s*[:=]\s*['"]?(\d{12})`), SeverityMedium},
		{TypeAWS, "AWS", "AWS AppSync API Key", regexp.MustCompile(`(da2-[A-Za-z0-9]{26})`), SeverityHigh},
		{TypeAWS, "AWS", "AWS API Gateway Key", regexp.MustCompile(`(?i)(x-api-key)\s*[:=]\s*['"]([A-Za-z0-9]{20,40})`), SeverityHigh},

		{TypeAzure, "Azure", "Azure Connection String", regexp.MustCompile(`(?i)(DefaultEndpointsProtocol|AccountName|AccountKey|BlobEndpoint|QueueEndpoint|TableEndpoint|FileEndpoint)\s*=\s*[^;\s]+`), SeverityHigh},
		{TypeAzure, "Azure", "Azure SQL Connection String (Server + Database)", regexp.MustCompile(`(?i)Server\s*=\s*tcp:[^;]+;\s*Database\s*=\s*[^;]+`), SeverityCritical},
		{TypeAzure, "Azure", "Azure Tenant ID", regexp.MustCompile(`(?i)(tenant_id|TENANT_ID|tenantId|azure_tenant)\s*[:=]\s*['"]?([0-9a-fA-F-]{36})`), SeverityMedium},
		{TypeAzure, "Azure", "Azure Client ID", regexp.MustCompile(`(?i)(client_id|CLIENT_ID|clientId|application_id)\s*[:=]\s*['"]?([0-9a-fA-F-]{36})`), SeverityMedium},
		{TypeAzure, "Azure", "Azure Client Secret", regexp.MustCompile(`(?i)(client_secret|CLIENT_SECRET|clientSecret)\s*[:=]\s*['"]?([A-Za-z0-9._~-]{34,})`), SeverityCritical},
		{TypeAzure, "Azure", "Azure Subscription Key", regexp.MustCompile(`(?i)(subscription_key|SUBSCRIPTION_KEY|ocp-apim-subscription-key)\s*[:=]\s*['"]?([A-Za-z0-9]{32})`), SeverityHigh},
		{TypeAzure, "Azure", "Azure Storage Account Key", regexp.MustCompile(`(?i)(AccountKey|storage_account_key)\s*[:=]\s*['"]?([A-Za-z0-9+/=]{86,88})`), SeverityCritical},
		{TypeAzure, "Azure", "Azure Redis Cache Key", regexp.MustCompile(`(?i)(redis_cache_key|RedisCacheKey)\s*[:=]\s*['"]?([A-Za-z0-9+/=]{40,})`), SeverityHigh},
		{TypeAzure, "Azure", "Azure Functions Key", regexp.MustCompile(`(?i)(x-functions-key|X-Functions-Key)\s*[:=]\s*['"]?([A-Za-z0-9]{32,})`), SeverityHigh},

		{TypeGCP, "GCP", "GCP API Key", regexp.MustCompile(`(?i)(AIza[0-9A-Za-z_-]{35})`), SeverityHigh},
		{TypeGCP, "GCP", "GCP Service Account", regexp.MustCompile(`(?i)([0-9a-f]{64})\s*\n\s*"private_key_id"|"private_key":\s*"-----BEGIN PRIVATE KEY-----`), SeverityCritical},
		{TypeGCP, "GCP", "GCP OAuth Token", regexp.MustCompile(`(ya29\.[0-9A-Za-z_-]+)`), SeverityHigh},
		{TypeGCP, "GCP", "GCP Service Account Email", regexp.MustCompile(`(?i)([a-zA-Z0-9_-]+@[a-zA-Z0-9_-]+\.iam\.gserviceaccount\.com)`), SeverityMedium},

		{TypeJWT, "JWT", "JWT Token (Bearer)", regexp.MustCompile(`(?i)Bearer\s+(eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+)`), SeverityHigh},
		{TypeJWT, "JWT", "JWT Token (Raw)", regexp.MustCompile(`(eyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]+)`), SeverityMedium},

		{TypeOAuth, "OAuth", "OAuth Client Secret", regexp.MustCompile(`(?i)(client_secret|CLIENT_SECRET)\s*[:=]\s*['"]?([A-Za-z0-9._~-]{20,})`), SeverityCritical},
		{TypeOAuth, "OAuth", "OAuth Access Token", regexp.MustCompile(`(?i)(access_token|ACCESS_TOKEN)\s*[:=]\s*['"]?([A-Za-z0-9._~-]{20,})`), SeverityHigh},
		{TypeOAuth, "OAuth", "OAuth Refresh Token", regexp.MustCompile(`(?i)(refresh_token|REFRESH_TOKEN)\s*[:=]\s*['"]?([A-Za-z0-9._~-]{20,})`), SeverityHigh},

		{TypeToken, "GitHub", "GitHub Personal Access Token", regexp.MustCompile(`(ghp_[0-9A-Za-z]{36}|gho_[0-9A-Za-z]{36}|ghu_[0-9A-Za-z]{36}|ghs_[0-9A-Za-z]{36}|ghr_[0-9A-Za-z]{36})`), SeverityCritical},
		{TypeToken, "GitHub", "GitHub Fine-Grained PAT", regexp.MustCompile(`(github_pat_[0-9A-Za-z]{82})`), SeverityCritical},
		{TypeToken, "GitHub", "GitHub OAuth Access Token", regexp.MustCompile(`(gho_[0-9A-Za-z_-]{36})`), SeverityCritical},
		{TypeToken, "GitHub", "GitHub App Token", regexp.MustCompile(`(ghs_[0-9A-Za-z_-]{36})`), SeverityCritical},
		{TypeToken, "GitHub", "GitHub Refresh Token", regexp.MustCompile(`(ghr_[0-9A-Za-z_-]{36})`), SeverityCritical},

		{TypeToken, "Slack", "Slack Bot Token", regexp.MustCompile(`(xoxb-[0-9A-Za-z-]{24,})`), SeverityCritical},
		{TypeToken, "Slack", "Slack User Token", regexp.MustCompile(`(xoxp-[0-9A-Za-z-]{24,})`), SeverityCritical},
		{TypeToken, "Slack", "Slack App Token", regexp.MustCompile(`(xapp-[0-9A-Za-z-]{24,})`), SeverityCritical},
		{TypeToken, "Slack", "Slack Webhook URL", regexp.MustCompile(`(https://hooks\.slack\.com/services/[A-Za-z0-9/]+)`), SeverityHigh},

		{TypeToken, "Stripe", "Stripe Secret Key (Live)", regexp.MustCompile(`(sk_live_[0-9A-Za-z]{24,})`), SeverityCritical},
		{TypeToken, "Stripe", "Stripe Secret Key (Test)", regexp.MustCompile(`(sk_test_[0-9A-Za-z]{24,})`), SeverityHigh},
		{TypeToken, "Stripe", "Stripe Publishable Key (Live)", regexp.MustCompile(`(pk_live_[0-9A-Za-z]{24,})`), SeverityMedium},
		{TypeToken, "Stripe", "Stripe Publishable Key (Test)", regexp.MustCompile(`(pk_test_[0-9A-Za-z]{24,})`), SeverityLow},
		{TypeToken, "Stripe", "Stripe Webhook Secret", regexp.MustCompile(`(whsec_[0-9A-Za-z]{24,})`), SeverityHigh},

		{TypeToken, "OpenAI", "OpenAI API Key", regexp.MustCompile(`(sk-proj-[0-9A-Za-z_-]{20,})`), SeverityCritical},
		{TypeToken, "OpenAI", "OpenAI Legacy Key", regexp.MustCompile(`(sk-[0-9A-Za-z_-]{20,})`), SeverityHigh},

		{TypeToken, "Twilio", "Twilio Account SID", regexp.MustCompile(`(AC[a-f0-9]{32})`), SeverityHigh},
		{TypeToken, "Twilio", "Twilio Auth Token", regexp.MustCompile(`(?i)(twilio.*auth.*token|TWILIO_AUTH_TOKEN)\s*[:=]\s*['"]?([A-Za-z0-9]{32})`), SeverityCritical},

		{TypeAPIKey, "Google", "Google API Key", regexp.MustCompile(`(?i)(AIza[0-9A-Za-z_-]{35})`), SeverityHigh},
		{TypeAPIKey, "SendGrid", "SendGrid API Key", regexp.MustCompile(`(SG\.[A-Za-z0-9_-]{22}\.[A-Za-z0-9_-]{43})`), SeverityHigh},
		{TypeAPIKey, "Mailgun", "Mailgun API Key", regexp.MustCompile(`(key-[0-9a-fA-F]{32})`), SeverityHigh},
		{TypeAPIKey, "Mailchimp", "Mailchimp API Key", regexp.MustCompile(`([0-9a-f]{32}-us[0-9]{1,2})`), SeverityHigh},
		{TypeAPIKey, "Heroku", "Heroku API Key", regexp.MustCompile(`(?i)(heroku.*api.*key|HEROKU_API_KEY)\s*[:=]\s*['"]?([A-Za-z0-9-]{36})`), SeverityHigh},
		{TypeAPIKey, "DigitalOcean", "DigitalOcean PAT", regexp.MustCompile(`(?i)(dop_v1_[0-9a-f]{64})`), SeverityHigh},
		{TypeAPIKey, "npm", "npm Access Token", regexp.MustCompile(`(npm_[A-Za-z0-9]{36})`), SeverityHigh},

		{TypePrivateKey, "SSH", "SSH Private Key", regexp.MustCompile(`-----BEGIN\s*(RSA|DSA|EC|OPENSSH|SSH2)\s*PRIVATE\s*KEY-----`), SeverityCritical},
		{TypePrivateKey, "PGP", "PGP Private Key", regexp.MustCompile(`-----BEGIN PGP PRIVATE KEY BLOCK-----`), SeverityCritical},
		{TypePrivateKey, "PKCS", "PKCS#8 Private Key", regexp.MustCompile(`-----BEGIN PRIVATE KEY-----`), SeverityCritical},
		{TypePrivateKey, "PKCS", "PKCS#8 Encrypted Private Key", regexp.MustCompile(`-----BEGIN ENCRYPTED PRIVATE KEY-----`), SeverityCritical},

		{TypeConnectionString, "MongoDB", "MongoDB Connection String", regexp.MustCompile(`(?i)(mongodb(?:\+srv)?:\/\/[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=-]+)`), SeverityHigh},
		{TypeConnectionString, "PostgreSQL", "PostgreSQL Connection String", regexp.MustCompile(`(?i)(postgres(?:ql)?:\/\/[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=-]+)`), SeverityHigh},
		{TypeConnectionString, "MySQL", "MySQL Connection String", regexp.MustCompile(`(?i)(mysql:\/\/[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=-]+)`), SeverityHigh},
		{TypeConnectionString, "Redis", "Redis Connection String", regexp.MustCompile(`(?i)(redis:\/\/[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=-]+)`), SeverityHigh},
		{TypeConnectionString, "SQLite", "SQLite Database Path", regexp.MustCompile(`(?i)(sqlite:\/\/[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=-]+)`), SeverityMedium},

		{TypePassword, "Generic", "Password in URI", regexp.MustCompile(`(?i)([a-z]+://[^:@\/\s]+):([^@\/\s]{4,})@`), SeverityHigh},
		{TypePassword, "Generic", "Password Field", regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*['"]?([A-Za-z0-9!@#$%^&*()_+={}\[\]|\\:;"'<>,.?/~` + "`" + `-]{8,})`), SeverityHigh},

		{TypeToken, "Generic", "Bearer Token", regexp.MustCompile(`(?i)Bearer\s+([A-Za-z0-9._~+/=-]{20,}=*)`), SeverityHigh},
		{TypeToken, "Generic", "Basic Auth Credentials", regexp.MustCompile(`(?i)Basic\s+([A-Za-z0-9+/=]{20,})`), SeverityHigh},

		{TypeToken, "Generic", "Generic API Key (assignment)", regexp.MustCompile(`(?i)(api[_-]?key|apikey|api_key)\s*[:=]\s*['"]?([A-Za-z0-9._~-]{16,64})`), SeverityMedium},
		{TypeToken, "Generic", "Generic Token (assignment)", regexp.MustCompile(`(?i)(token|secret|auth_token)\s*[:=]\s*['"]?([A-Za-z0-9._~-]{20,64})`), SeverityMedium},

		{TypeHighEntropy, "Generic", "High-Entropy Hex String (≥32)", regexp.MustCompile(`(?i)([0-9a-f]{32,})`), SeverityLow},
		{TypeHighEntropy, "Generic", "High-Entropy Base64 String (≥40)", regexp.MustCompile(`(?:[A-Za-z0-9+/]{40,}={0,2})`), SeverityLow},
	}
}
