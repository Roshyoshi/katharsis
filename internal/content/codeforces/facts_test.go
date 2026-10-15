package codeforces

import "testing"

// The phrasings are from output sections at rev.
func TestFloatTolerance(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text string
		want float64
	}{
		{"Your answer will be considered correct if its absolute or relative error does not exceed $$$10^{-6}$$$.", 1e-6},
		{"Your answer will be accepted if it has absolute or relative error at most 10 - 9. More specifically", 1e-9},
		{"The answer will be considered correct if its relative or absolute error doesn't exceed 10 - 6.", 1e-6},
		{"Your answer is considered correct if its absolute or relative error does not exceed 10^{-4}.", 1e-4},
		{"with an absolute or relative error no more than 1e - 6", 1e-6},
		{"absolute or relative error of no more than $$$2 \\cdot 10^{-5}$$$", 2e-5},
		{"Absolute or relative error should not be greater than 0.000001", 1e-6},
		{"Print the answer with absolute or relative error from the correct answer.", 1e-6},
		{"Print one real number with precision 10 - 6 — the maximal possible probability", 1e-6},
		{"Print the coordinates with 6 digits after the decimal point.", 1e-6},
		{"print the probability with at least four digits after the decimal point", 1e-4},
		{"Print the absolute value of the difference.", 0},
		{"Print YES or NO.", 0},
		{"If the message cannot contain an error, output NO.", 0},
	}
	for _, tc := range tests {
		if got := floatTolerance(tc.text); got != tc.want {
			t.Errorf("floatTolerance(%q) = %g, want %g", tc.text, got, tc.want)
		}
	}
}

func TestMultiAnswer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text string
		want bool
	}{
		{"If there are multiple answers, print any of them.", true},
		{"If there are several solutions, output any.", true},
		{"For each test case, output any string of length $$$n$$$ that satisfies the conditions.", true},
		{"print any suitable permutation", true},
		{"Print the only integer — the answer.", false},
		{"Print the number of valid pairs.", false},
	}
	for _, tc := range tests {
		if got := multiAnswer(tc.text); got != tc.want {
			t.Errorf("multiAnswer(%q) = %v", tc.text, got)
		}
	}
}

func TestCaseInsensitive(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text string
		want bool
	}{
		{`You can output the answer in any case (for example, the strings "yEs", "yes" will be recognized).`, true},
		{"You may print every letter in any case you want.", true},
		{"You can print each letter in any register.", true},
		{"Letters can be printed in upper or lower case.", true},
		{"Print the string in lowercase.", false},
		{"In any test case, print YES.", false},
	}
	for _, tc := range tests {
		if got := caseInsensitive(tc.text); got != tc.want {
			t.Errorf("caseInsensitive(%q) = %v", tc.text, got)
		}
	}
}
