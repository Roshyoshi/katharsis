package usaco

import "math"

// baseRating estimates a Codeforces-scale rating from division and problem
// position. USACO orders P1 < P2 < P3 by intended difficulty, and the USACO
// Guide FAQ puts the divisions at roughly Bronze 900-1500, Silver
// 1200-1900, Gold 1500-2200 and Platinum 1900+ ("very rough"). The table
// spaces positions 150 apart inside those bands.
//
// Before the 2015-16 season there was no Platinum. Old Silver and Old Gold
// are rated about half a division above today's of the same name. That is
// an inference from where the guide files them: "Old Silver" problems in
// Silver and Gold modules, "Old Gold" ones in Gold, Platinum and Advanced
// modules. Old Bronze is not: it sat half a division up too until solving
// them showed otherwise (content QA, 2026-10-14: five old P1s, cpids 84,
// 94, 121, 340 and 358, took Codeforces 800-1000 effort, below the 1150
// they were rated), so it now shares today's Bronze row. Old Bronze had a
// fourth problem in six contests (Nov 2011, Dec 2014, Jan 2015 and the
// 2012, 2013 and 2015 US Opens), rated like P3.
//
//	            P1    P2    P3    P4
//	bronze     1000  1150  1300
//	silver     1350  1500  1650
//	gold       1700  1850  2000
//	platinum   2100  2250  2400
//	old bronze 1000  1150  1300  1300
//	old silver 1550  1700  1850  1850
//	old gold   1950  2100  2250  2250
var baseRating = map[string][4]int{
	"bronze":     {1000, 1150, 1300, 1300},
	"silver":     {1350, 1500, 1650, 1650},
	"gold":       {1700, 1850, 2000, 2000},
	"platinum":   {2100, 2250, 2400, 2400},
	"old bronze": {1000, 1150, 1300, 1300},
	"old silver": {1550, 1700, 1850, 1850},
	"old gold":   {1950, 2100, 2250, 2250},
}

// labelDelta adjusts the base by the guide's difficulty label. Labels are
// relative to the module the problem is listed in, so only modules in the
// problem's own section count (homeSections); "Easy" in a Platinum module
// says nothing about a Bronze problem. Several labels are averaged. The
// spread (±150) is half a division: a label refines the position, it does
// not overrule the division.
var labelDelta = map[string]int{
	"Very Easy": -150,
	"Easy":      -75,
	"Normal":    0,
	"Medium":    0,
	"Hard":      75,
	"Very Hard": 150,
	"Insane":    225,
}

// homeSections maps a division ("old gold" for pre-2015-16 contests) to
// the guide sections (content/<n>_<Section>) whose labels speak to it.
var homeSections = map[string][]string{
	"bronze":     {"bronze"},
	"silver":     {"silver"},
	"gold":       {"gold"},
	"platinum":   {"plat", "adv"},
	"old bronze": {"bronze", "silver"},
	"old silver": {"silver", "gold"},
	"old gold":   {"gold", "plat", "adv"},
}

// eraDivision is the rating-table key: "old gold" for a Gold problem from
// before the 2015-16 season, the division otherwise.
func eraDivision(cpid int, division string) string {
	if cpid < firstPlatinumCPID && division != "platinum" {
		return "old " + division
	}
	return division
}

// firstPlatinumCPID is 2015 December Bronze P1, the first problem of the
// season that introduced Platinum.
const firstPlatinumCPID = 567

// estimate returns the rating and how it was derived.
func estimate(era string, position int, labels []string) (int, string) {
	row, ok := baseRating[era]
	if !ok {
		return 0, ""
	}
	pos := min(max(position, 1), 4)
	r := row[pos-1]
	src := "division+position"
	if position == 0 {
		r = row[1] // middle of the band when the position is unknown
		src = "division"
	}
	var sum, n int
	for _, l := range labels {
		if d, ok := labelDelta[l]; ok {
			sum += d
			n++
		}
	}
	if n > 0 {
		r += int(math.Round(float64(sum) / float64(n)))
		src += "+guide-label"
	}
	return r, src
}

// DivisionBand is the band today's division (catalog.Problem.Division:
// bronze, silver, gold, platinum) is rated in before labels and solve
// rates move a problem: its P1 to its P3. ok is false for any other name.
func DivisionBand(division string) (band [2]int, ok bool) {
	switch division {
	case "bronze", "silver", "gold", "platinum":
		row := baseRating[division]
		return [2]int{row[0], row[2]}, true
	}
	return [2]int{}, false
}
