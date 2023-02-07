package awsconfigfile

import (
	"bytes"
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
						StartUrl:      "https://example.com",
						SSORegion:     "ap-southeast-2",
						AccountId:     "123456789012",
						AccountName:   "testing",
						RoleName:      "DevRole",
						CommonFateURL: "https://commonfate.example.com",
					},
				},
			},
			want: `[profile example]
test = 1

[profile testing/DevRole]
granted_sso_start_url    = https://example.com
granted_sso_region       = ap-southeast-2
granted_sso_account_id   = 123456789012
granted_sso_role_name    = DevRole
generated_by_common_fate = true
credential_process       = granted credential-process --profile testing/DevRole --url https://commonfate.example.com
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

			assert.Equal(t, tt.want, b.String())
		})
	}
}
