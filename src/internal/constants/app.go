package constants

// AppName is the name of the application.
const (
	AppName    = "dire"
	AppVersion = "0.4.7"
)

// Constants for config file names.
const (
	ConfigFileBaseName = "." + AppName
	ConfigFileNameTOML = ConfigFileBaseName + ".toml"
)

// File permission constants - secure by default since this is a new application.
const (
	// SecureFilePermissions for locale files (owner read/write only).
	SecureFilePermissions = 0o600
	// SecureDirPermissions for locale directories (owner full access, group read/execute).
	SecureDirPermissions = 0o750
)

// Format constants for CLI output formatting.
const (
	FormatDefault = "default"
	FormatJSON    = "json"
)

// Exit code constants for CLI commands.
const (
	ExitCodeSuccess = 0 // All operations completed successfully
	ExitCodeWarning = 1 // Operation completed with warnings (e.g., missing translations)
	ExitCodeError   = 2 // Configuration or system errors
)
