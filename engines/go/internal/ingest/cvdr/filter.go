package cvdr

import "strings"

var excludedTouristTaxTitleTerms = []string{
	"uitvoeringsregeling",
	"beleidsregel",
	"beleidsregels",
	"nadere regel",
	"nadere regels",
	"wijzigingsverordening",
	"wijziging",
	"intrekking",
	"aanwijzingsbesluit",
	"uitwerkingsbesluit",
}

func isLikelyTouristTaxBaseWork(workFile WorkFile) bool {
	for _, version := range workFile.Versions {
		if isLikelyTouristTaxBaseRecord(version) {
			return true
		}
	}
	return isLikelyTouristTaxBaseTitle(workFile.Work.RepresentativeTitle)
}

func isLikelyTouristTaxBaseRecord(record Record) bool {
	titleText := normalizeFilterText(record.Title + " " + record.Alternative)
	if !strings.Contains(titleText, "toeristenbelasting") {
		return false
	}
	for _, term := range excludedTouristTaxTitleTerms {
		if strings.Contains(titleText, term) {
			return false
		}
	}

	ratifiedBy := normalizeFilterText(record.RatifiedBy)
	if ratifiedBy != "" {
		return strings.Contains(ratifiedBy, "gemeenteraad")
	}

	return strings.Contains(titleText, "verordening")
}

func isLikelyTouristTaxBaseTitle(title string) bool {
	normalized := normalizeFilterText(title)
	if !strings.Contains(normalized, "toeristenbelasting") {
		return false
	}
	for _, term := range excludedTouristTaxTitleTerms {
		if strings.Contains(normalized, term) {
			return false
		}
	}
	return strings.Contains(normalized, "verordening")
}

func normalizeFilterText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Join(strings.Fields(value), " ")
}
