package awsconfigfile

import (
	"os"
	"path/filepath"
)

// DefaultSharedConfigFilename returns the AWS SDK's default file path for
// the shared config file.
// It is vendored from the AWS Go SDK v2 to prevent importing the entire module.
//
// Builds the shared config file path based on the OS's platform.
//
//   - Linux/Unix: $HOME/.aws/config
//   - Windows: %USERPROFILE%\.aws\config
func DefaultSharedConfigFilename() string {
	return filepath.Join(userHomeDir(), ".aws", "config")
}

func userHomeDir() string {
	// Ignore errors since we only care about Windows and *nix.
	homedir, _ := os.UserHomeDir()
	return homedir
}
