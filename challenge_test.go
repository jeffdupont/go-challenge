package main

import (
	"regexp"
	"testing"
)

// validate
func validateRune(input string) bool {
	if len(input) > 64 {
		return false
	}
	for i, r := range input {
		// Make sure that the '-' is not the first char in the string
		if i == 0 && r == '-' {
			return false
		}
		// Using the ASCII values, we can determine if each rune is valid. We first
		// check to see if it is outside the given ranges for 0-9, A-Z, and a-z
		// and then finally make sure that it's not '-'
		if !(r >= '0' && r <= '9') && !(r >= 'A' && r <= 'Z') && !(r >= 'a' && r <= 'z') && r != '-' {
			return false
		}
	}
	return true
}

func validateRegex(input string) bool {
	if len(input) > 64 {
		return false
	}
	ok, _ := regexp.MatchString("[0-9a-zA-Z]+[0-9a-zA-Z-]*", input)
	return ok
}

// testCase
type testCase struct {
	input          string
	expectedResult bool
}

var testCases = []testCase{
	{
		"asdf",
		true,
	},
	{
		"-asdf-asdf",
		false,
	},
	{
		"asdf asdf",
		false,
	},
	{
		"asdf-asdf-asdf-asdf-asdf",
		true,
	},
	{
		"-asdf",
		false,
	},
	{
		"!asdf",
		false,
	},
	{
		"asdfasdfasdfasdfasdfasdfasdfasdfasdfasdfasdfasdfasdfasdfasdfasdfx",
		false,
	},
}

func TestRune(t *testing.T) {
	for _, tc := range testCases {
		r := validateRune(tc.input)
		if r != tc.expectedResult {
			t.Errorf("validate(%s); got %v, want %v", tc.input, r, tc.expectedResult)
		}
	}
}

func BenchmarkRune(b *testing.B) {
	for i := 0; i < b.N; i++ {
		validateRune("asdf-asdf-asdf-asdf-asdf")
	}
}

func TestRegex(t *testing.T) {
	for _, tc := range testCases {
		r := validateRegex(tc.input)
		if r != tc.expectedResult {
			t.Errorf("validate(%s); got %v, want %v", tc.input, r, tc.expectedResult)
		}
	}
}

func BenchmarkRegex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		validateRegex("asdf-asdf-asdf-asdf-asdf")
	}
}
