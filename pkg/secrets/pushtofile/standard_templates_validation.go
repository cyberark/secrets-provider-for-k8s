package pushtofile

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

const (
	maxYAMLKeyLen       = 1024
	maxJSONKeyLen       = 2097152
)

func checkValidYAMLKey(key string) error {
	if len(key) > maxYAMLKeyLen {
		return fmt.Errorf("the key '%s' is too long for YAML", key)
	}
	for _, c := range key {
		if !isValidYAMLChar(c) {
			return fmt.Errorf("invalid YAML character: '%c'", c)
		}
	}
	return nil
}

func isValidYAMLChar(c rune) bool {
	// Checks whether a character is in the YAML valid character set as
	// defined here: https://yaml.org/spec/1.2.2/#51-character-set
	switch {
	case c == '\u0009':
		return true // tab
	case c == '\u000A':
		return true // LF
	case c == '\u000D':
		return true // CR
	case c >= '\u0020' && c <= '\u007E':
		return true // Printable ASCII
	case c == '\u0085':
		return true // Next Line (NEL)
	case c >= '\u00A0' && c <= '\uD7FF':
		return true // Basic Multilingual Plane (BMP)
	case c >= '\uE000' && c <= '\uFFFD':
		return true // Additional Unicode Areas
	case c >= '\U00010000' && c <= '\U0010FFFF':
		return true // 32 bit
	default:
		return false
	}
}

func checkValidJSONKey(key string) error {
	if len(key) > maxJSONKeyLen {
		return fmt.Errorf("the key '%s' is too long for JSON", key)
	}
	for _, c := range key {
		if !isValidJSONChar(c) {
			return fmt.Errorf("invalid JSON character: '%c'", c)
		}
	}
	return nil
}

func isValidJSONChar(c rune) bool {
	// Checks whether a character is in the JSON valid character set as
	// defined here: https://www.json.org/json-en.html
	// This document specifies that any characters are valid except:
	//   - Control characters (0x00-0x1F and 0x7f [DEL])
	//   - Double quote (")
	//   - Backslash (\)
	switch {
	case c >= '\u0000' && c <= '\u001F':
		return false // Control characters other than DEL
	case c == '\u007F':
		return false // DEL
	case c == '"':
		return false // Double quote
	case c == '\\':
		return false // Backslash
	default:
		return true
	}
}

func checkValidBashVarName(name string) error {
	var valid = true

	if firstRune, _ := utf8.DecodeRuneInString(name); unicode.IsDigit(firstRune) {
		valid = false
	}


	for _, r := range name {
		if !valid {
			break
		}

		valid = unicode.IsLetter(r) ||
			unicode.IsDigit(r) ||
			r == '_'
	}

	if !valid {
		return fmt.Errorf(
			"invalid alias '%s': %s",
			name,
			"must be alphanumerics and underscores, with first char being a non-digit",
		)
	}

	return nil
}
