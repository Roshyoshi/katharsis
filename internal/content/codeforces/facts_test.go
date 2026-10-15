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
		output, description string
		want                bool
	}{
		{"If there are multiple answers, print any of them.", "", true},
		{"If there are several solutions, output any.", "", true},
		{"For each test case, output any string of length $$$n$$$ that satisfies the conditions.", "", true},
		{"print any suitable permutation", "", true},
		// cf:233A and cf:219A: judged against one reference answer before.
		{"If there are multiple solutions, print any of them.", "", true},
		{"", "Your task is to find any suitable division for each of the q independent queries.", true},
		{"If there exists more than one solution, you can find any.", "", true},
		{"On the second line print d + 1 numbers — any of the possible shortest paths for Vasya.", "", true},
		{"If there are several optimal solutions, he is fine with any of them.", "", true},
		{"Print the indices of the chosen roads in any order.", "", true},
		{"If there are multiple answers with the minimum number of operations, print any of them.", "", true},

		{"Print the only integer — the answer.", "", false},
		{"Print the number of valid pairs.", "", false},
		// A condition that picks one answer is a unique answer.
		{"If there are multiple answers, print the lexicographically smallest one.", "", false},
		{"If there are multiple solutions, print the one with the minimum number a. If there still are multiple solutions, print the one with the minimum b.", "", false},
		{"If there are many answers, output the smallest among them.", "", false},
		{"Since there may be many possible answers, print all of them.", "", false},
		{"If there are several solutions, Vasya wants to find the smallest possible x.", "", false},
		// Negated.
		{"Do not print any extra characters.", "", false},
		{"", "Please note, that in this problem there aren't multiple possible answers — the answer is unique.", false},
		// Story, not output, in a description.
		{"", "You are allowed to skip any of them, however changing the order is prohibited.", false},
		{"", "Divisor of n is any such natural number, that n can be divided by it without remainder.", false},
		{"", "The numbers are given in arbitrary order.", false},
	}
	for _, tc := range tests {
		if got := multiAnswer(tc.output, tc.description); got != tc.want {
			t.Errorf("multiAnswer(%q, %q) = %v", tc.output, tc.description, got)
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
		// The phrasings caseRE missed (cf:832A, cf:832B, cf:868B, cf:899B,
		// cf:939A, cf:955B, cf:1624B, cf:989C).
		{"You can print each letter in arbitrary case (upper of lower).", true},
		{"You can choose the case (lower or upper) for each letter arbitrary.", true},
		{`You can print each character either upper- or lowercase ("YeS" and "yes" are valid).`, true},
		{"You can print each letter in arbitrary case (small or large).", true},
		{"You can output any letter in lower case or in upper case.", true},
		{"Each letter can be printed in arbitrary case.", true},
		{"You can print YES and NO in any (upper or lower) case.", true},
		{"You can output each letter in either case (upper or lower).", true},
		{"Print the string in lowercase.", false},
		{"In any test case, print YES.", false},
		{"In either case, b_1 = 1.", false},
		{"Note that in neither case Hermione will be given the hat.", false},
	}
	for _, tc := range tests {
		if got := caseInsensitive(tc.text); got != tc.want {
			t.Errorf("caseInsensitive(%q) = %v", tc.text, got)
		}
	}
}
