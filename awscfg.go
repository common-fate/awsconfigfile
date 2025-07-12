// Package awsconfigfile contains logic to template ~/.aws/config files
// based on Common Fate access rules.
package awsconfigfile

import (
	"bytes"
	"slices"
	"sort"
	"strings"
	"text/template"

	"github.com/dlclark/regexp2"
	"github.com/Masterminds/sprig/v3"
	"github.com/common-fate/clio"
	"gopkg.in/ini.v1"
)

type SSOProfile interface {
	ToIni(profileName string, nocredentialProcessProfile bool) any
}

type SSOSession struct {
	SSOSessionName			    string
	SSOStartURL             string
	SSORegistrationScopes   string
	SSORegion               string
	GeneratedFrom string
}

type ssoSession struct {
	SSOStartURL             string `ini:"sso_start_url"`
	SSORegistrationScopes   string `ini:"sso_registration_scopes"`
	SSORegion               string `ini:"sso_region"`
	CommonFateGeneratedFrom string `ini:"common_fate_generated_from,omitempty"`
}

func (s *SSOSession) ToIni(profileName string, nocredentialProcessProfile bool) any {
	return &ssoSession{
		SSOStartURL:           s.SSOStartURL,
		SSORegistrationScopes: s.SSORegistrationScopes,
		SSORegion:             s.SSORegion,
		CommonFateGeneratedFrom: s.GeneratedFrom,
	}
}

type AccountProfile struct {
	AccountName    string
	SSOSessionName     string
	AccountID   string
	RoleName       string
	GeneratedFrom  string
	Region         string
	CommonFateURL  string
	// Legacy format used for credential process
	SSOStartURL string
	SSORegion   string
}

type credentialProcessProfile struct {
	SSOStartURL             string `ini:"granted_sso_start_url"`
	SSORegion               string `ini:"granted_sso_region,omitempty"`
	AccountID            string `ini:"granted_sso_account_id"`
	RoleName                string `ini:"granted_sso_role_name"`
	CommonFateGeneratedFrom string `ini:"common_fate_generated_from"`
	CredentialProcess       string `ini:"credential_process"`
	Region                  string `ini:"region,omitempty"`
}

type regularProfile struct {
	SSOSession              string `ini:"sso_session"`
	AccountID            string `ini:"sso_account_id"`
	CommonFateGeneratedFrom string `ini:"common_fate_generated_from"`
	RoleName                string `ini:"sso_role_name"`
	Region                  string `ini:"region,omitempty"`
}

func (a *AccountProfile) ToIni(profileName string, noCredentialProcess bool) any {
	if noCredentialProcess {
		return &regularProfile{
				SSOSession:              a.SSOSessionName,
				AccountID:            a.AccountID,
				RoleName:                a.RoleName,
				CommonFateGeneratedFrom: a.GeneratedFrom,
				Region:                  a.Region,
		}
	}
	credProcess := "granted credential-process --profile " + profileName
	if a.CommonFateURL != "" {
		credProcess += " --url " + a.CommonFateURL
	}
	
	return &credentialProcessProfile{
		SSOStartURL:             a.SSOStartURL,
		SSORegion:               a.SSORegion,
		AccountID:            a.AccountID,
		RoleName:                a.RoleName,
		CredentialProcess:       credProcess,
		CommonFateGeneratedFrom: a.GeneratedFrom,
		Region:                  a.Region,
	}
}

type MergeOpts struct {
	Config              *ini.File
	Prefix              string
	Profiles            []SSOProfile
	SectionNameTemplate string
	NoCredentialProcess bool
	// PruneStartURLs is a slice of AWS SSO start URLs which profiles are being generated for.
	// Existing profiles with these start URLs will be removed if they aren't found in the Profiles field.
	PruneStartURLs []string
	SessionName		string
	SSOScopes			[]string
	PreferRoles		[]string
	Verbose 			bool
	DefaultRegion string
}

func Merge(opts MergeOpts) error {
	if opts.Verbose {
		clio.SetLevelFromString("debug")
	}
	if opts.SectionNameTemplate == "" {
		opts.SectionNameTemplate = "{{ .AccountName }}/{{ .RoleName }}"
	}
	
	// Separate SSOSession and AccountProfile types
	var ssoSessions []SSOSession
	var accountProfiles []*AccountProfile
	
	for _, profile := range opts.Profiles {
		switch p := profile.(type) {
		case *SSOSession:
			ssoSessions = append(ssoSessions, *p) // Store a copy of the session
		case *AccountProfile:
			accountProfiles = append(accountProfiles, p)
		default:
			return nil // Unsupported profile type, skip
		}
	}
		

	// Sort profiles by CombinedName (AccountName/RoleName)
	sort.SliceStable(accountProfiles, func(i, j int) bool {
		combinedNameI := accountProfiles[i].AccountName + "/" + accountProfiles[i].RoleName
		combinedNameJ := accountProfiles[j].AccountName + "/" + accountProfiles[j].RoleName
		return combinedNameI < combinedNameJ
	})

	funcMap := sprig.TxtFuncMap()
	sectionNameTempl, err := template.New("").Funcs(funcMap).Parse(opts.SectionNameTemplate)
	if err != nil {
		return err
	}

	// remove any config sections that have 'common_fate_generated_from' as a key
	for _, sec := range opts.Config.Sections() {
		var startURL string

		if sec.HasKey("granted_sso_start_url") {
			startURL = sec.Key("granted_sso_start_url").String()
		} else if sec.HasKey("sso_start_url") {
			startURL = sec.Key("sso_start_url").String()
		}

		for _, pruneURL := range opts.PruneStartURLs {
			isGenerated := sec.HasKey("common_fate_generated_from") // true if the profile was created automatically.

			if isGenerated && startURL == pruneURL {
				opts.Config.DeleteSection(sec.Name())
			}
		}
	}
	
	for _, ssoSession := range ssoSessions {
		ssoSession.SSOSessionName = normalizeAccountName(ssoSession.SSOSessionName)

		sectionName := "sso-session " + ssoSession.SSOSessionName
		
		opts.Config.DeleteSection(sectionName)
		section, err := opts.Config.NewSection(sectionName)
		if err != nil {
			return err
		}
		entry := ssoSession.ToIni(ssoSession.SSOSessionName, opts.NoCredentialProcess)
		err = section.ReflectFrom(entry)
		if err != nil {
			return err
		}
	}

	// Create auto-generated SSO session profiles when using no-credential-process mode
	// and the profile doesn't already reference an existing SSO session
	var ssoSessionName string
	if opts.NoCredentialProcess {
		ssoSessionName = opts.SessionName
		// Track created session names to avoid duplicates
		createdSessions := make(map[string]bool)
		
		// First pass: find all account profiles that don't have an SSO session name set
		for _, accountProfile := range accountProfiles {
			// Skip if this account profile already has an SSOSessionName
			if accountProfile.SSOSessionName != "" {
				continue
			}
			
			// Generate a session name based on account name and role
			sessionName := opts.SessionName
			if opts.Prefix != "" {
				sessionName = normalizeAccountName(opts.Prefix + sessionName)
			}
			
			// Skip if we've already created this session
			if createdSessions[sessionName] {
				continue
			}
			
			// Create an SSO session
			ssoSession := SSOSession{
				SSORegistrationScopes: strings.Join(opts.SSOScopes, " "),
				SSOSessionName: sessionName,
				SSOStartURL:    accountProfile.SSOStartURL,
				SSORegion:      accountProfile.SSORegion,
				GeneratedFrom:  accountProfile.GeneratedFrom,
			}
			
			// Create the session section
			sectionName := "sso-session " + sessionName
			opts.Config.DeleteSection(sectionName)
			section, err := opts.Config.NewSection(sectionName)
			if err != nil {
				return err
			}
			
			entry := ssoSession.ToIni(sessionName, opts.NoCredentialProcess)
			err = section.ReflectFrom(entry)
			if err != nil {
				return err
			}
			
			// Update the account profile to reference this session
			accountProfile.SSOSessionName = sessionName
			
			// Mark this session as created
			createdSessions[sessionName] = true
		}
	}
	
	// Now process all account profiles
	var seenProfileNames []string
	var profileNameToRoles = make(map[string][]string)
	
	for _, accountProfile := range accountProfiles {
		clio.Debugf("Processing account profile: %s/%s", accountProfile.AccountName, accountProfile.RoleName)
		accountProfile.AccountName = normalizeAccountName(accountProfile.AccountName)
		accountProfile.SSOSessionName = ssoSessionName
		sectionNameBuffer := bytes.NewBufferString("")
		err := sectionNameTempl.Execute(sectionNameBuffer, accountProfile)
		if err != nil {
			return err
		}
		
		if accountProfile.Region == "" && opts.DefaultRegion != "" {
			accountProfile.Region = opts.DefaultRegion
		}
		
		profileName := opts.Prefix + sectionNameBuffer.String()
		sectionName := "profile " + profileName
		
		// Is profileName in the seenProfileNames list?
		var isSeen bool
		for _, seenName := range seenProfileNames {
			if seenName == profileName {
				isSeen = true
				break
			}
		}
		var isOverwrite = false
		if isSeen {
			// If it is, check if the user provided any PreferRoles
			if len(opts.PreferRoles) > 0 {
				existingSection, err := opts.Config.GetSection(sectionName)
				if err != nil {
					return err
				}
				thisRoleName := accountProfile.RoleName
				// Check granted_sso_role_name and sso_role_name to get the existing role
				var existingRoleName string
				if existingSection.HasKey("granted_sso_role_name") {
					existingRoleName = existingSection.Key("granted_sso_role_name").String()
				} else {
					existingRoleName = existingSection.Key("sso_role_name").String()
				}

				var shouldOverwrite bool = true
				for _, preferRole := range opts.PreferRoles {
					r, _ := regexp2.Compile(preferRole, 0)
					matchesThisRole, _ := r.MatchString(thisRoleName)
					matchesExistingRole, _ := r.MatchString(existingRoleName)
					if matchesThisRole && !matchesExistingRole {
						// Overwrite the existing section with the new profile
						clio.Debugf("[%s] Overwriting existing role %s with new role %s", profileName, existingRoleName, thisRoleName)
						break
					} else if !matchesThisRole && matchesExistingRole {
						// Don't overwrite the existing section with the new profile
						clio.Debugf("[%s} Existing role %s matches with a higher priority than %s (%s)", profileName, existingRoleName, thisRoleName, preferRole)
						shouldOverwrite = false
						break
					}
			}
			if !shouldOverwrite {
				clio.Infof("Skipping profile %s as it already exists and no prefer roles matched higher than the existing role", profileName)
				continue
			}
			isOverwrite = true
		}
	}

		opts.Config.DeleteSection(sectionName)
		section, err := opts.Config.NewSection(sectionName)
		if err != nil {
			return err
		}

		entry := accountProfile.ToIni(profileName, opts.NoCredentialProcess)
		err = section.ReflectFrom(entry)
		if err != nil {
			return err
		}
		if !isOverwrite {
			seenProfileNames = append(seenProfileNames, profileName)
			profileNameToRoles[profileName] = append(profileNameToRoles[profileName], accountProfile.RoleName)
		}
	}
	// Check for duplicate profile names
	slices.Sort(seenProfileNames)
	var dupes []string
	for i := 1; i < len(seenProfileNames); i++ {
		if seenProfileNames[i] == seenProfileNames[i-1] {
			dupes = append(dupes, seenProfileNames[i])
		}
	}
	slices.Sort(dupes)
	dupes = slices.Compact(dupes)
	if len(dupes) > 0 {
		clio.Warn("Duplicate profile names detected. Only the last result will be used. You may need to manually modify the generated config file to use the correct role:")
		for _, dup := range dupes {
			roles := profileNameToRoles[dup]
			clio.Warnf("Profile %s has roles: %s", dup, strings.Join(roles, ", "))
		}
	}

	return nil
}


func normalizeAccountName(accountName string) string {
	return strings.ReplaceAll(accountName, " ", "-")
}
