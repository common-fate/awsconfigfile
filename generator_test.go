package awsconfigfile

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/ini.v1"
)

// testSource implements the Source interface
// and provides mock AWS profiles
type testSource struct {
	Profiles []SSOProfile
}

func (s testSource) GetProfiles(ctx context.Context) ([]SSOProfile, error) {
	return s.Profiles, nil
}

func TestGenerator_Generate(t *testing.T) {
	tests := []struct {
		name                string
		profiles            []SSOProfile
		config              string
		noCredentialProcess bool
		sectionNameTemplate string
		prefix              string
		pruneStartURLs      []string
		want                string
		wantErr             bool
	}{
		{
			name: "ok",
			profiles: []SSOProfile{
				&AccountProfile{
					SSOStartURL:    "https://example.awsapps.com/start",
					SSORegion:        "ap-southeast-2",
					AccountID:  "123456789012",
					AccountName:   "prod",
					RoleName:   "DevRole",
					GeneratedFrom: "aws-sso",
				},
			},
			config: `
[profile example]
test = 1
`,
			want: `
[profile example]
test = 1

[profile prod/DevRole]
granted_sso_start_url      = https://example.awsapps.com/start
granted_sso_region         = ap-southeast-2
granted_sso_account_id     = 123456789012
granted_sso_role_name      = DevRole
common_fate_generated_from = aws-sso
credential_process         = granted credential-process --profile prod/DevRole
`,
		},
		{
			name:   "ok with prefix",
			prefix: "myprefix-",
			profiles: []SSOProfile{
				&AccountProfile{
					SSOStartURL:    "https://example.awsapps.com/start",
					SSORegion:        "ap-southeast-2",
					AccountID:  "123456789012",
					AccountName:   "prod",
					RoleName:   "DevRole",
					GeneratedFrom: "aws-sso",
				},
			},
			config: `
[profile example]
test = 1
`,
			want: `
[profile example]
test = 1

[profile myprefix-prod/DevRole]
granted_sso_start_url      = https://example.awsapps.com/start
granted_sso_region         = ap-southeast-2
granted_sso_account_id     = 123456789012
granted_sso_role_name      = DevRole
common_fate_generated_from = aws-sso
credential_process         = granted credential-process --profile myprefix-prod/DevRole
`,
		},
		{
			name:                "invalid template fails whitespace",
			sectionNameTemplate: "{{ .AccountName }}. ",
			wantErr:             true,
		},
		{
			name:                "invalid template fails ;",
			sectionNameTemplate: "{{ .AccountName }}.;",
			wantErr:             true,
		},
		{
			name:                "valid template",
			sectionNameTemplate: "{{ .AccountName }}.hello",
			profiles: []SSOProfile{
				&AccountProfile{
					SSOStartURL:    "https://example.awsapps.com/start",
					SSORegion:        "ap-southeast-2",
					AccountID:  "123456789012",
					AccountName:   "prod",
					RoleName:   "DevRole",
					GeneratedFrom: "aws-sso",
				},
			},
			want: `
[profile prod.hello]
granted_sso_start_url      = https://example.awsapps.com/start
granted_sso_region         = ap-southeast-2
granted_sso_account_id     = 123456789012
granted_sso_role_name      = DevRole
common_fate_generated_from = aws-sso
credential_process         = granted credential-process --profile prod.hello
`,
		},
		{
			name: "ok with region",
			profiles: []SSOProfile{
				&AccountProfile{
					SSOStartURL:    "https://example.awsapps.com/start",
					SSORegion: 			"ap-southeast-2",
					AccountID:  "123456789012",
					AccountName:   "prod",
					RoleName:   "DevRole",
					GeneratedFrom: "aws-sso",
					Region:        "us-west-2",
				},
			},
			want: `
[profile prod/DevRole]
granted_sso_start_url      = https://example.awsapps.com/start
granted_sso_region         = ap-southeast-2
granted_sso_account_id     = 123456789012
granted_sso_role_name      = DevRole
common_fate_generated_from = aws-sso
credential_process         = granted credential-process --profile prod/DevRole
region                     = us-west-2
`,
		},
		{
			name: "ok with pruning",
			config: `
[profile should_be_removed]
common_fate_generated_from = aws-sso
granted_sso_start_url = https://deleteme.example.com

[profile should_be_kept]
test = 1
			`,
			profiles: []SSOProfile{
				&AccountProfile{
					SSOStartURL:    "https://example.awsapps.com/start",
					SSORegion: "ap-southeast-2",
					AccountID:  "123456789012",
					AccountName:   "prod",
					RoleName:   "DevRole",
					GeneratedFrom: "aws-sso",
					Region:        "us-west-2",
				},
			},
			pruneStartURLs: []string{"https://deleteme.example.com"},
			want: `
[profile should_be_kept]
test = 1

[profile prod/DevRole]
granted_sso_start_url      = https://example.awsapps.com/start
granted_sso_region         = ap-southeast-2
granted_sso_account_id     = 123456789012
granted_sso_role_name      = DevRole
common_fate_generated_from = aws-sso
credential_process         = granted credential-process --profile prod/DevRole
region                     = us-west-2
`,
		},
		{
			name: "pruning one start url should not remove the other",
			config: `
[profile should_be_removed]
common_fate_generated_from = aws-sso
granted_sso_start_url = https://deleteme.example.com

[profile should_be_kept]
common_fate_generated_from = aws-sso
granted_sso_start_url = https://somethingelse.example.com
`,
			profiles: []SSOProfile{
				&AccountProfile{
					SSOStartURL:    "https://example.awsapps.com/start",
					SSORegion: "ap-southeast-2",
					AccountID:  "123456789012",
					AccountName:   "prod",
					RoleName:   "DevRole",
					GeneratedFrom: "aws-sso",
					Region:        "us-west-2",
				},
			},
			pruneStartURLs: []string{"https://deleteme.example.com"},
			want: `
[profile should_be_kept]
common_fate_generated_from = aws-sso
granted_sso_start_url      = https://somethingelse.example.com

[profile prod/DevRole]
granted_sso_start_url      = https://example.awsapps.com/start
granted_sso_region         = ap-southeast-2
granted_sso_account_id     = 123456789012
granted_sso_role_name      = DevRole
common_fate_generated_from = aws-sso
credential_process         = granted credential-process --profile prod/DevRole
region                     = us-west-2
`,
		},
		{
			name: "sso profiles sorted alphabetically",
			profiles: []SSOProfile{
				&AccountProfile{
					SSOStartURL:   "https://example.awsapps.com/start",
					SSORegion:     "ap-southeast-2",
					AccountID:     "123456789012",
					AccountName:   "prod",
					RoleName:      "DevRoleTwo",
					GeneratedFrom: "aws-sso",
					Region:        "us-west-2",
				},
				&AccountProfile{
					SSOStartURL:   "https://example.awsapps.com/start",
					SSORegion:     "ap-southeast-2",
					AccountID:     "123456789012",
					AccountName:   "prod",
					RoleName:      "DevRoleOne",
					GeneratedFrom: "aws-sso",
					Region:        "us-west-2",
				},
			},
			want: `
[profile prod/DevRoleOne]
granted_sso_start_url      = https://example.awsapps.com/start
granted_sso_region         = ap-southeast-2
granted_sso_account_id     = 123456789012
granted_sso_role_name      = DevRoleOne
common_fate_generated_from = aws-sso
credential_process         = granted credential-process --profile prod/DevRoleOne
region                     = us-west-2

[profile prod/DevRoleTwo]
granted_sso_start_url      = https://example.awsapps.com/start
granted_sso_region         = ap-southeast-2
granted_sso_account_id     = 123456789012
granted_sso_role_name      = DevRoleTwo
common_fate_generated_from = aws-sso
credential_process         = granted credential-process --profile prod/DevRoleTwo
region                     = us-west-2
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var output bytes.Buffer

			cfg, err := ini.Load([]byte(tt.config))
			if err != nil {
				t.Fatal(err)
			}

			g := &Generator{
				Sources:             []Source{testSource{Profiles: tt.profiles}},
				Config:              cfg,
				NoCredentialProcess: tt.noCredentialProcess,
				ProfileNameTemplate: tt.sectionNameTemplate,
				Prefix:              tt.prefix,
				PruneStartURLs:      tt.pruneStartURLs,
			}
			if err := g.Generate(ctx); (err != nil) != tt.wantErr {
				t.Errorf("Generator.Generate() error = %v, wantErr %v", err, tt.wantErr)
			}

			_, err = cfg.WriteTo(&output)
			if err != nil {
				t.Fatal(err)
			}

			// ignore leading/trailing whitespace so it's easier to format the 'want' section in the test table
			got := strings.TrimSpace(output.String())
			want := strings.TrimSpace(tt.want)
			assert.Equal(t, want, got)
		})
	}
}

func TestGenerator_GenerateNoCredentialProcess(t *testing.T) {
	tests := []struct {
		name                string
		profiles            []SSOProfile
		config              string
		sectionNameTemplate string
		prefix              string
		pruneStartURLs      []string
		want                string
		wantErr             bool
	}{
		{
			name: "ok",
			profiles: []SSOProfile{
				&SSOSession{
					SSOSessionName: "company",
					SSOStartURL:		"https://example.awsapps.com/start",
					SSORegion:			"ap-southeast-2",
					SSORegistrationScopes: "sso:account:access",
					GeneratedFrom: "aws-sso",
				},
				&AccountProfile{
					SSOSessionName: "company",
					AccountID:      "123456789012",
					AccountName:    "prod",
					RoleName:       "DevRole",
					GeneratedFrom:  "aws-sso",
				},
			},
			config: `
[profile example]
test = 1
`,
			want: `
[profile example]
test = 1

[sso-session company]
sso_start_url              = https://example.awsapps.com/start
sso_registration_scopes    = sso:account:access
sso_region                 = ap-southeast-2
common_fate_generated_from = aws-sso

[profile prod/DevRole]
sso_session                = company
sso_account_id             = 123456789012
common_fate_generated_from = aws-sso
sso_role_name              = DevRole
`,
		},
		{
			name:   "ok with prefix",
			prefix: "myprefix-",
			profiles: []SSOProfile{
				&SSOSession{
					SSOSessionName: "company",
					SSOStartURL:        "https://example.awsapps.com/start",
					SSORegion:          "ap-southeast-2",
					SSORegistrationScopes: "sso:account:access",
					GeneratedFrom: "aws-sso",
			},
			&AccountProfile{
					SSOSessionName: "company",
					AccountID:      "123456789012",
					AccountName:    "prod",
					RoleName:       "DevRole",
					GeneratedFrom:  "aws-sso",
			},
			},
			config: `
[profile example]
test = 1
`,
			want: `
[profile example]
test = 1

[sso-session company]
sso_start_url              = https://example.awsapps.com/start
sso_registration_scopes    = sso:account:access
sso_region                 = ap-southeast-2
common_fate_generated_from = aws-sso

[profile myprefix-prod/DevRole]
sso_session                = company
sso_account_id             = 123456789012
common_fate_generated_from = aws-sso
sso_role_name              = DevRole
`,
		},
		{
			name:                "invalid template fails whitespace",
			sectionNameTemplate: "{{ .AccountName }}. ",
			wantErr:             true,
		},
		{
			name:                "invalid template fails ;",
			sectionNameTemplate: "{{ .AccountName }}.;",
			wantErr:             true,
		},
		{
			name:                "valid template",
			sectionNameTemplate: "{{ .AccountName }}.hello",
			profiles: []SSOProfile{
				&SSOSession{
					SSOSessionName: "company",
					SSOStartURL:    "https://example.awsapps.com/start",
					SSORegion:      "ap-southeast-2",
					SSORegistrationScopes: "sso:account:access",
					GeneratedFrom:  "aws-sso",
				},
				&AccountProfile{
					SSOSessionName: "company",
					AccountID:      "123456789012",
					AccountName:    "prod",
					RoleName:       "DevRole",
					GeneratedFrom:  "aws-sso",
				},
			},
			want: `
[sso-session company]
sso_start_url              = https://example.awsapps.com/start
sso_registration_scopes    = sso:account:access
sso_region                 = ap-southeast-2
common_fate_generated_from = aws-sso

[profile prod.hello]
sso_session                = company
sso_account_id             = 123456789012
common_fate_generated_from = aws-sso
sso_role_name              = DevRole
`,
		},
		{
			name: "ok with region",
			profiles: []SSOProfile{
				&SSOSession{
					SSOSessionName: "company",
					SSOStartURL:    "https://example.awsapps.com/start",
					SSORegion:      "ap-southeast-2",
					SSORegistrationScopes: "sso:account:access",
					GeneratedFrom:  "aws-sso",
				},
				&AccountProfile{
					SSOSessionName: "company",
					AccountID:      "123456789012",
					AccountName:    "prod",
					RoleName:       "DevRole",
					GeneratedFrom:  "aws-sso",
					Region:         "us-west-2",
				},
			},
			want: `
[sso-session company]
sso_start_url              = https://example.awsapps.com/start
sso_registration_scopes    = sso:account:access
sso_region                 = ap-southeast-2
common_fate_generated_from = aws-sso

[profile prod/DevRole]
sso_session                = company
sso_account_id             = 123456789012
common_fate_generated_from = aws-sso
sso_role_name              = DevRole
region                     = us-west-2
`,
		},
		{
			name: "ok with pruning",
			config: `
[profile should_be_removed]
common_fate_generated_from = aws-sso
granted_sso_start_url = https://deleteme.example.com

[profile should_be_kept]
test = 1
			`,
			profiles: []SSOProfile{
				&SSOSession{
					SSOSessionName: "company",
					SSOStartURL:    "https://example.awsapps.com/start",
					SSORegion:      "ap-southeast-2",
					SSORegistrationScopes: "sso:account:access",
					GeneratedFrom:  "aws-sso",
				},
				&AccountProfile{
					SSOSessionName: "company",
					AccountID:      "123456789012",
					AccountName:    "prod",
					RoleName:       "DevRole",
					GeneratedFrom:  "aws-sso",
					Region:         "us-west-2",
				},
			},
			pruneStartURLs: []string{"https://deleteme.example.com"},
			want: `
[profile should_be_kept]
test = 1

[sso-session company]
sso_start_url              = https://example.awsapps.com/start
sso_registration_scopes    = sso:account:access
sso_region                 = ap-southeast-2
common_fate_generated_from = aws-sso

[profile prod/DevRole]
sso_session                = company
sso_account_id             = 123456789012
common_fate_generated_from = aws-sso
sso_role_name              = DevRole
region                     = us-west-2
`,
		},
		{
			name: "pruning one start url should not remove the other",
			config: `
[profile should_be_removed]
common_fate_generated_from = aws-sso
granted_sso_start_url = https://deleteme.example.com

[profile should_be_kept]
common_fate_generated_from = aws-sso
granted_sso_start_url = https://somethingelse.example.com
`,
			profiles: []SSOProfile{
				&SSOSession{
					SSOSessionName: "company",
					SSOStartURL:    "https://example.awsapps.com/start",
					SSORegion:      "ap-southeast-2",
					SSORegistrationScopes: "sso:account:access",
					GeneratedFrom:  "aws-sso",
				},
				&AccountProfile{
					SSOSessionName: "company",
					AccountID:      "123456789012",
					AccountName:    "prod",
					RoleName:       "DevRole",
					GeneratedFrom:  "aws-sso",
					Region:         "us-west-2",
				},
			},
			pruneStartURLs: []string{"https://deleteme.example.com"},
			want: `
[profile should_be_kept]
common_fate_generated_from = aws-sso
granted_sso_start_url      = https://somethingelse.example.com

[sso-session company]
sso_start_url              = https://example.awsapps.com/start
sso_registration_scopes    = sso:account:access
sso_region                 = ap-southeast-2
common_fate_generated_from = aws-sso

[profile prod/DevRole]
sso_session                = company
sso_account_id             = 123456789012
common_fate_generated_from = aws-sso
sso_role_name              = DevRole
region                     = us-west-2
`,
		},
		{
			name: "sso profiles sorted alphabetically",
			profiles: []SSOProfile{
				&SSOSession{
					SSOSessionName: "company",
					SSOStartURL:    "https://example.awsapps.com/start",
					SSORegion:      "ap-southeast-2",
					SSORegistrationScopes: "sso:account:access",
					GeneratedFrom:  "aws-sso",
				},
				&AccountProfile{
					SSOSessionName: "company",
					AccountID:     "123456789012",
					AccountName:   "prod",
					RoleName:      "DevRoleTwo",
					GeneratedFrom: "aws-sso",
					Region:        "us-west-2",
				},
				&AccountProfile{
					SSOSessionName: "company",
					AccountID:     "123456789012",
					AccountName:   "prod",
					RoleName:      "DevRoleOne",
					GeneratedFrom: "aws-sso",
					Region:        "us-west-2",
				},
			},
			want: `
[sso-session company]
sso_start_url              = https://example.awsapps.com/start
sso_registration_scopes    = sso:account:access
sso_region                 = ap-southeast-2
common_fate_generated_from = aws-sso

[profile prod/DevRoleOne]
sso_session                = company
sso_account_id             = 123456789012
common_fate_generated_from = aws-sso
sso_role_name              = DevRoleOne
region                     = us-west-2

[profile prod/DevRoleTwo]
sso_session                = company
sso_account_id             = 123456789012
common_fate_generated_from = aws-sso
sso_role_name              = DevRoleTwo
region                     = us-west-2
`,
		},
		{
			name: "with SSO session",
			profiles: []SSOProfile{
				&SSOSession{
					SSOStartURL:          "https://example.com",
					SSOSessionName:       "example-session",
					SSORegistrationScopes: "example-scope",
					SSORegion:            "ap-southeast-2",
					GeneratedFrom:         "aws-sso",
				},
				&AccountProfile{
					SSOSessionName: "example-session",
					AccountID:      "123456789012",
					AccountName:    "testing",
					RoleName:       "DevRole",
					GeneratedFrom:  "aws-sso",
					Region:         "ap-southeast-2",
				},
			},
			want: `
[sso-session example-session]
sso_start_url              = https://example.com
sso_registration_scopes    = example-scope
sso_region                 = ap-southeast-2
common_fate_generated_from = aws-sso

[profile testing/DevRole]
sso_session                = example-session
sso_account_id             = 123456789012
common_fate_generated_from = aws-sso
sso_role_name              = DevRole
region                     = ap-southeast-2
`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var output bytes.Buffer

			cfg, err := ini.Load([]byte(tt.config))
			if err != nil {
				t.Fatal(err)
			}

			g := &Generator{
				Sources:             []Source{testSource{Profiles: tt.profiles}},
				Config:              cfg,
				NoCredentialProcess: true, // Force no credential process for all tests in this suite
				ProfileNameTemplate: tt.sectionNameTemplate,
				Prefix:              tt.prefix,
				PruneStartURLs:      tt.pruneStartURLs,
			}
			if err := g.Generate(ctx); (err != nil) != tt.wantErr {
				t.Errorf("Generator.Generate() error = %v, wantErr %v", err, tt.wantErr)
			}

			_, err = cfg.WriteTo(&output)
			if err != nil {
				t.Fatal(err)
			}

			// ignore leading/trailing whitespace so it's easier to format the 'want' section in the test table
			got := strings.TrimSpace(output.String())
			want := strings.TrimSpace(tt.want)
			assert.Equal(t, want, got)
		})
	}
}
