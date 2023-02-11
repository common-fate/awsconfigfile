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

	SSOStartURL string
	SSORegion   string

	// Account and role details
	Region        string
	AccountID     string
	AccountName   string
	RoleName      string
	CommonFateURL string
	// GeneratedFrom is the source that the profile
	// was created from, such as 'commonfate' or 'aws-sso'
	GeneratedFrom string
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
			SSOStartURL:             p.SSOStartURL,
			SSORegion:               p.SSORegion,
			SSOAccountID:            p.AccountID,
			SSORoleName:             p.RoleName,
			CommonFateGeneratedFrom: p.GeneratedFrom,
			Region:                  p.Region,
		}
	}

	credProcess := "granted credential-process --profile " + profileName

	if p.CommonFateURL != "" {
		credProcess += " --url " + p.CommonFateURL
	}

	return &credentialProcessProfile{
		SSOStartURL:             p.SSOStartURL,
		SSORegion:               p.SSORegion,
		SSOAccountID:            p.AccountID,
		SSORoleName:             p.RoleName,
		CredentialProcess:       credProcess,
		CommonFateGeneratedFrom: p.GeneratedFrom,
		Region:                  p.Region,
	}
}

type MergeOpts struct {
	Config              *ini.File
	Prefix              string
	Profiles            []SSOProfile
	SectionNameTemplate string
	NoCredentialProcess bool
	Prune               bool
}

func Merge(opts MergeOpts) error {
	if opts.SectionNameTemplate == "" {
		opts.SectionNameTemplate = "{{ .AccountName }}/{{ .RoleName }}"
	}

	sectionNameTempl, err := template.New("").Parse(opts.SectionNameTemplate)
	if err != nil {
		return err
	}

	if opts.Prune {
		// remove any config sections that have 'common_fate_generated_from' as a key
		for _, sec := range opts.Config.Sections() {
			if sec.HasKey("common_fate_generated_from") && sec.Key("granted_sso_start_url").String() == opts.Profiles[0].SSOStartURL {
				opts.Config.DeleteSection(sec.Name())
			}
		}
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
	SSOStartURL             string `ini:"granted_sso_start_url"`
	SSORegion               string `ini:"granted_sso_region"`
	SSOAccountID            string `ini:"granted_sso_account_id"`
	SSORoleName             string `ini:"granted_sso_role_name"`
	CommonFateGeneratedFrom string `ini:"common_fate_generated_from"`
	CredentialProcess       string `ini:"credential_process"`
	Region                  string `ini:"region,omitempty"`
}

type regularProfile struct {
	SSOStartURL             string `ini:"sso_start_url"`
	SSORegion               string `ini:"sso_region"`
	SSOAccountID            string `ini:"sso_account_id"`
	CommonFateGeneratedFrom string `ini:"common_fate_generated_from"`
	SSORoleName             string `ini:"sso_role_name"`
	Region                  string `ini:"region,omitempty"`
}

func normalizeAccountName(accountName string) string {
	return strings.ReplaceAll(accountName, " ", "-")
}
