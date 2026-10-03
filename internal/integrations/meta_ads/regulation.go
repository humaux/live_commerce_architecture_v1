package metaads

// Verified request declarations, not a local advertising-law policy. Source:
// owner validate-only probe 2026-10-03, ads-graph.md Amendment 2. Add rows only
// with provider evidence. Meta decides eligibility, age and beneficiary rules.
var regionalDeclarations = [...]struct{ country, category string }{
	{"TW", "TAIWAN_UNIVERSAL"},
	{"SG", "SINGAPORE_UNIVERSAL"},
}

func regulatedCategories(countries []string) []string {
	var categories []string
	for _, row := range regionalDeclarations {
		for _, country := range countries {
			if country == row.country {
				categories = append(categories, row.category)
				break
			}
		}
	}
	return categories
}
