package gateway

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var scriptNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

const invalidScriptNameMessage = "script_name must start with an alphanumeric character and contain only alphanumerics, underscores, or dashes"

func validateScriptNameComponent(scriptName string) error {
	if scriptName == "" {
		return errors.New(invalidScriptNameMessage)
	}
	if filepath.IsAbs(scriptName) || strings.Contains(scriptName, "/") || strings.Contains(scriptName, "\\") || strings.Contains(scriptName, "..") {
		return errors.New(invalidScriptNameMessage)
	}
	if !scriptNamePattern.MatchString(scriptName) {
		return errors.New(invalidScriptNameMessage)
	}
	return nil
}

func canonicalAllowedScriptName(allowedScripts map[string]struct{}, scriptName string) (string, error) {
	requestedName := strings.TrimSpace(scriptName)
	if err := validateScriptNameComponent(requestedName); err != nil {
		return "", err
	}
	if len(allowedScripts) == 0 {
		return "", errors.New("script_name allowlist is empty")
	}
	for allowedName := range allowedScripts {
		if err := validateScriptNameComponent(allowedName); err != nil {
			return "", fmt.Errorf("configured script_name %q is invalid: %w", allowedName, err)
		}
		if requestedName == allowedName {
			return allowedName, nil
		}
	}
	return "", errors.New("script_name is not in the configured allowlist")
}
