package core

import (
	"strings"
	"testing"
)

func TestPasswordPolicyUsesUnicodeCodePointsAndUTF8ByteLimit(t *testing.T) {
	tests := []struct {
		name     string
		password string
		valid    bool
	}{
		{name: "seven ascii", password: "Abc12!x", valid: false},
		{name: "eight ascii", password: "Abc12!xy", valid: true},
		{name: "whitespace only", password: "        ", valid: false},
		{name: "four emoji are four code points", password: "😀😀😀😀", valid: false},
		{name: "eight emoji", password: strings.Repeat("😀", 8), valid: true},
		{name: "seventy two bytes", password: strings.Repeat("a", 72), valid: true},
		{name: "seventy three bytes", password: strings.Repeat("a", 73), valid: false},
		{name: "nineteen four byte code points", password: strings.Repeat("😀", 19), valid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validatePasswordValue(test.password)
			if test.valid && err != nil {
				t.Fatalf("validatePasswordValue(%q) error=%v", test.password, err)
			}
			if !test.valid && err == nil {
				t.Fatalf("validatePasswordValue(%q) unexpectedly passed", test.password)
			}
		})
	}
}

func TestPasswordConfirmationMustMatch(t *testing.T) {
	if err := validateNewPassword("Abc12!xy", "Abc12!xy"); err != nil {
		t.Fatal(err)
	}
	if err := validateNewPassword("Abc12!xy", "Abc12!xz"); err == nil {
		t.Fatal("mismatched confirmation unexpectedly passed")
	}
}
