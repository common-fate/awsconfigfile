package awsconfigfile

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/ini.v1"
)

func parseIni(t *testing.T, data string) *ini.File {
	ini, err := ini.Load([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	return ini
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name    string
		args    MergeOpts
		want    string
		wantErr bool
	}{
		{
			name: "ok",
			args: MergeOpts{
				Config: parseIni(t, `
[profile example]
test = 1
`),
				Profiles: []SSOProfile{
					{
						SSOStartURL:   "https://example.com",
						SSORegion:     "ap-southeast-2",
						AccountID:     "123456789012",
						AccountName:   "testing",
						RoleName:      "DevRole",
						GeneratedFrom: "aws-sso",
						CommonFateURL: "https://commonfate.example.com",
					},
				},
			},
			want: `
[profile example]
test = 1

[profile testing/DevRole]
granted_sso_start_url      = https://example.com
granted_sso_region         = ap-southeast-2
granted_sso_account_id     = 123456789012
granted_sso_role_name      = DevRole
common_fate_generated_from = aws-sso
credential_process         = granted credential-process --profile testing/DevRole --url https://commonfate.example.com
`,
		},
		{
			name: "ok with no credential process",
			args: MergeOpts{
				Config: parseIni(t, `
[profile example]
test = 1
`),
				NoCredentialProcess: true,
				Profiles: []SSOProfile{
					{
						SSOStartURL:   "https://example.com",
						SSORegion:     "ap-southeast-2",
						AccountID:     "123456789012",
						AccountName:   "testing",
						RoleName:      "DevRole",
						GeneratedFrom: "aws-sso",
						CommonFateURL: "https://commonfate.example.com",
					},
				},
			},
			want: `
[profile example]
test = 1

[profile testing/DevRole]
sso_start_url              = https://example.com
sso_region                 = ap-southeast-2
sso_account_id             = 123456789012
common_fate_generated_from = aws-sso
sso_role_name              = DevRole
`,
		},
		{
			name: "no common fate url",
			args: MergeOpts{
				Config: parseIni(t, `
[profile example]
test = 1
`),
				Profiles: []SSOProfile{
					{
						SSOStartURL:   "https://example.com",
						SSORegion:     "ap-southeast-2",
						AccountID:     "123456789012",
						AccountName:   "testing",
						RoleName:      "DevRole",
						GeneratedFrom: "aws-sso",
					},
				},
			},
			want: `
[profile example]
test = 1

[profile testing/DevRole]
granted_sso_start_url      = https://example.com
granted_sso_region         = ap-southeast-2
granted_sso_account_id     = 123456789012
granted_sso_role_name      = DevRole
common_fate_generated_from = aws-sso
credential_process         = granted credential-process --profile testing/DevRole
`,
		},
		{
			name: "ok with sprig formatting",
			args: MergeOpts{
				Config: parseIni(t, `
[profile example]
test = 1
`),
				SectionNameTemplate: "{{ replace ` ` `-` .AccountName | lower }}/{{ .RoleName }}",
				Profiles: []SSOProfile{
					{
						SSOStartURL:   "https://example.com",
						SSORegion:     "ap-southeast-2",
						AccountID:     "123456789012",
						AccountName:   "Testing Title Case With Space",
						RoleName:      "DevRole",
						GeneratedFrom: "aws-sso",
						CommonFateURL: "https://commonfate.example.com",
					},
				},
			},
			want: `
[profile example]
test = 1

[profile testing-title-case-with-space/DevRole]
granted_sso_start_url      = https://example.com
granted_sso_region         = ap-southeast-2
granted_sso_account_id     = 123456789012
granted_sso_role_name      = DevRole
common_fate_generated_from = aws-sso
credential_process         = granted credential-process --profile testing-title-case-with-space/DevRole --url https://commonfate.example.com
`,
		},
		{
			name: "profile names (AccountName/RoleName) sorted alphabetically",
			args: MergeOpts{
				Config: parseIni(t, ""),
				Profiles: []SSOProfile{
					{
						SSOStartURL:   "https://example.awsapps.com/start",
						SSORegion:     "ap-southeast-2",
						AccountID:     "123456789012",
						AccountName:   "account1",
						RoleName:      "DevRoleTwo",
						GeneratedFrom: "aws-sso",
						Region:        "us-west-2",
					},
					{
						SSOStartURL:   "https://example.awsapps.com/start",
						SSORegion:     "ap-southeast-2",
						AccountID:     "123456789012",
						AccountName:   "account1",
						RoleName:      "DevRoleOne",
						GeneratedFrom: "aws-sso",
						Region:        "us-west-2",
					},
					{
						SSOStartURL:   "https://example.awsapps.com/start",
						SSORegion:     "ap-southeast-2",
						AccountID:     "123456789012",
						AccountName:   "account2",
						RoleName:      "DevRoleOne",
						GeneratedFrom: "aws-sso",
						Region:        "us-west-2",
					},
				},
			},
			want: `
[profile account1/DevRoleOne]
granted_sso_start_url      = https://example.awsapps.com/start
granted_sso_region         = ap-southeast-2
granted_sso_account_id     = 123456789012
granted_sso_role_name      = DevRoleOne
common_fate_generated_from = aws-sso
credential_process         = granted credential-process --profile account1/DevRoleOne
region                     = us-west-2

[profile account1/DevRoleTwo]
granted_sso_start_url      = https://example.awsapps.com/start
granted_sso_region         = ap-southeast-2
granted_sso_account_id     = 123456789012
granted_sso_role_name      = DevRoleTwo
common_fate_generated_from = aws-sso
credential_process         = granted credential-process --profile account1/DevRoleTwo
region                     = us-west-2

[profile account2/DevRoleOne]
granted_sso_start_url      = https://example.awsapps.com/start
granted_sso_region         = ap-southeast-2
granted_sso_account_id     = 123456789012
granted_sso_role_name      = DevRoleOne
common_fate_generated_from = aws-sso
credential_process         = granted credential-process --profile account2/DevRoleOne
region                     = us-west-2
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Merge(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("Merge() error = %v, wantErr %v", err, tt.wantErr)
			}
			var b bytes.Buffer

			_, err := tt.args.Config.WriteTo(&b)
			if err != nil {
				t.Fatal(err)
			}

			// ignore leading/trailing whitespace so it's easier to format the 'want' section in the test table
			got := strings.TrimSpace(b.String())
			want := strings.TrimSpace(tt.want)

			assert.Equal(t, want, got)
		})
	}
}
