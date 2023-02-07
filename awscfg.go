// Package awsconfigfile contains logic to template ~/.aws/config files
// based on Common Fate access rules.
package awsconfigfile

import (
	"bytes"
	"strings"
	"text/template"

	"gopkg.in/ini.v1"
)

type SSOProfile struct {
	// SSO details
	StartUrl  string
	SSORegion string
	// Account and role details
	AccountId     string
	AccountName   string
	RoleName      string
	CommonFateURL string
}

// ToIni converts a profile to a struct with `ini` tags
// ready to be written to an ini config file.
//
// if noCredentialProcess is true, the struct will contain sso_ parameters
// like sso_role_name, sso_start_url, etc.
//
// if noCredentialProcess is false, the struct will contain granted_sso parameters
// for use with the Granted credential process, like granted_sso_role_name,
// granted_sso_start_url, and so forth.
func (p SSOProfile) ToIni(profileName string, noCredentialProcess bool) any {
	if noCredentialProcess {
		return &regularProfile{
			SSOStartURL:           p.StartUrl,
			SSORegion:             p.SSORegion,
			SSOAccountID:          p.AccountId,
			SSORoleName:           p.RoleName,
			GeneratedByCommonFate: "true",
		}
	}

	credProcess := "granted credential-process --profile " + profileName

	if p.CommonFateURL != "" {
		credProcess += " --url " + p.CommonFateURL
	}

	return &credentialProcessProfile{
		SSOStartURL:           p.StartUrl,
		SSORegion:             p.SSORegion,
		SSOAccountID:          p.AccountId,
		SSORoleName:           p.RoleName,
		CredentialProcess:     credProcess,
		GeneratedByCommonFate: "true",
	}
}

type MergeOpts struct {
	Config              *ini.File
	Prefix              string
	Profiles            []SSOProfile
	SectionNameTemplate string
	NoCredentialProcess bool
}

func Merge(opts MergeOpts) error {
	if opts.SectionNameTemplate == "" {
		opts.SectionNameTemplate = "{{ .AccountName }}/{{ .RoleName }}"
	}

	sectionNameTempl, err := template.New("").Parse(opts.SectionNameTemplate)
	if err != nil {
		return err
	}

	for _, ssoProfile := range opts.Profiles {
		ssoProfile.AccountName = normalizeAccountName(ssoProfile.AccountName)
		sectionNameBuffer := bytes.NewBufferString("")
		err := sectionNameTempl.Execute(sectionNameBuffer, ssoProfile)
		if err != nil {
			return err
		}
		profileName := opts.Prefix + sectionNameBuffer.String()
		sectionName := "profile " + profileName

		opts.Config.DeleteSection(sectionName)
		section, err := opts.Config.NewSection(sectionName)
		if err != nil {
			return err
		}

		entry := ssoProfile.ToIni(profileName, opts.NoCredentialProcess)
		err = section.ReflectFrom(entry)
		if err != nil {
			return err
		}

	}

	return nil
}

type credentialProcessProfile struct {
	SSOStartURL           string `ini:"granted_sso_start_url"`
	SSORegion             string `ini:"granted_sso_region"`
	SSOAccountID          string `ini:"granted_sso_account_id"`
	SSORoleName           string `ini:"granted_sso_role_name"`
	GeneratedByCommonFate string `ini:"generated_by_common_fate"`
	CredentialProcess     string `ini:"credential_process"`
}

type regularProfile struct {
	SSOStartURL           string `ini:"sso_start_url"`
	SSORegion             string `ini:"sso_region"`
	SSOAccountID          string `ini:"sso_account_id"`
	GeneratedByCommonFate string `ini:"generated_by_common_fate"`
	SSORoleName           string `ini:"sso_role_name"`
}

func normalizeAccountName(accountName string) string {
	return strings.ReplaceAll(accountName, " ", "-")
}
