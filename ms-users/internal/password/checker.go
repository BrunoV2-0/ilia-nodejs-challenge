package password

import (
	"errors"
	"unicode"

	zxcvbn "github.com/nbutton23/zxcvbn-go"
)

const minZxcvbnScore = 2

// StrengthChecker satisfies domain/user.PasswordStrengthChecker.
type StrengthChecker struct{}

func NewStrengthChecker() StrengthChecker { return StrengthChecker{} }

func (StrengthChecker) CheckStrength(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	var hasLetter, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLetter {
		return errors.New("password must contain at least one letter")
	}
	if !hasDigit {
		return errors.New("password must contain at least one number")
	}

	if result := zxcvbn.PasswordStrength(password, nil); result.Score < minZxcvbnScore {
		return errors.New("password is too weak: avoid common words, sequences, and patterns")
	}

	return nil
}
