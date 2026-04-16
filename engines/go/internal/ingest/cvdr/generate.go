package cvdr

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ramones/tax-conformance-kit/engines/go/internal/model"
)

type GenerateDraftFixturesRequest struct {
	ExtractionDir           string
	RepoRoot                string
	StrictWarnings          bool
	Overwrite               bool
	MunicipalityCatalogPath string
}

type GenerateDraftFixturesResult struct {
	GeneratedAt             string   `json:"generated_at"`
	ExtractionDir           string   `json:"extraction_dir"`
	RepoRoot                string   `json:"repo_root"`
	StrictWarnings          bool     `json:"strict_warnings"`
	BundleCount             int      `json:"bundle_count"`
	GeneratedCount          int      `json:"generated_count"`
	SkippedExistingCount    int      `json:"skipped_existing_count"`
	SkippedWarningCount     int      `json:"skipped_warning_count"`
	SkippedUnsupportedCount int      `json:"skipped_unsupported_count"`
	SkippedEmptyCount       int      `json:"skipped_empty_count"`
	GeneratedPaths          []string `json:"generated_paths,omitempty"`
}

func GenerateDraftFixtures(req GenerateDraftFixturesRequest) (GenerateDraftFixturesResult, error) {
	if req.ExtractionDir == "" {
		return GenerateDraftFixturesResult{}, fmt.Errorf("extraction directory is required")
	}
	if req.RepoRoot == "" {
		return GenerateDraftFixturesResult{}, fmt.Errorf("repo root is required")
	}

	catalog, err := maybeReadMunicipalityCatalog(req.MunicipalityCatalogPath)
	if err != nil {
		return GenerateDraftFixturesResult{}, err
	}

	paths, err := filepath.Glob(filepath.Join(req.ExtractionDir, "bundles", "*", "*", "analysis.json"))
	if err != nil {
		return GenerateDraftFixturesResult{}, err
	}
	sort.Strings(paths)

	result := GenerateDraftFixturesResult{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		ExtractionDir:  req.ExtractionDir,
		RepoRoot:       req.RepoRoot,
		StrictWarnings: req.StrictWarnings,
		BundleCount:    len(paths),
		GeneratedPaths: []string{},
	}

	for _, analysisPath := range paths {
		var analysis BundleAnalysis
		if err := readJSON(analysisPath, &analysis); err != nil {
			return GenerateDraftFixturesResult{}, err
		}
		bundleDir := filepath.Dir(analysisPath)
		var draft DraftStub
		if err := readJSON(filepath.Join(bundleDir, "draft.json"), &draft); err != nil {
			return GenerateDraftFixturesResult{}, err
		}

		if len(analysis.CandidateRules) == 0 {
			result.SkippedEmptyCount++
			continue
		}
		if req.StrictWarnings && len(analysis.Warnings) > 0 {
			result.SkippedWarningCount++
			continue
		}
		if !allSupportedDraftCandidates(analysis.CandidateRules) {
			result.SkippedUnsupportedCount++
			continue
		}

		ruleset := buildDraftRuleSet(draft, analysis, catalog)
		targetPath := filepath.Join(req.RepoRoot, filepath.FromSlash(draft.SuggestedFixturePath))
		if _, err := os.Stat(targetPath); err == nil {
			if !req.Overwrite || !canOverwriteGeneratedFixture(targetPath) {
				result.SkippedExistingCount++
				continue
			}
		} else if err != nil && !os.IsNotExist(err) {
			return GenerateDraftFixturesResult{}, err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return GenerateDraftFixturesResult{}, err
		}
		if err := writeJSONFile(targetPath, ruleset); err != nil {
			return GenerateDraftFixturesResult{}, err
		}
		result.GeneratedCount++
		result.GeneratedPaths = append(result.GeneratedPaths, filepath.ToSlash(strings.TrimPrefix(targetPath, req.RepoRoot+"/")))
	}

	if result.GeneratedCount > 0 {
		fixtureRoot := filepath.Join(req.RepoRoot, "core", "fixtures", "regulation", "nl", "gemeentelijke_verordening")
		if err := reconcileGeneratedFixtureValidityWindows(fixtureRoot); err != nil {
			return GenerateDraftFixturesResult{}, err
		}
	}

	return result, nil
}

func allSupportedDraftCandidates(candidates []RuleCandidate) bool {
	for _, candidate := range candidates {
		switch candidate.Calculation.Kind {
		case "generic.per_person_per_night", "generic.percentage_of_base":
		default:
			return false
		}
	}
	return true
}

func buildDraftRuleSet(draft DraftStub, analysis BundleAnalysis, catalog *MunicipalityCatalog) model.RuleSet {
	slug := slugifyMunicipality(draft.Jurisdiction.MunicipalityName)
	datePart := fixtureDate(draft)
	match, hasMunicipalityCode := MunicipalityMatch{}, false
	if catalog != nil {
		match, hasMunicipalityCode = LookupMunicipalityCode(*catalog, draft.Jurisdiction.MunicipalityName)
	}
	rules := make([]model.Rule, 0, len(analysis.CandidateRules))
	for idx, candidate := range analysis.CandidateRules {
		ruleID := candidate.ID
		if strings.TrimSpace(ruleID) == "" {
			ruleID = fmt.Sprintf("nl-%s-%s-rule-%d", slug, strings.ReplaceAll(datePart, "-", ""), idx+1)
		}
		validFrom := firstNonEmpty(candidate.ValidFrom, datePart)
		ruleNotes := []string{
			candidate.Notes,
			"Evidence articles: " + strings.Join(candidate.EvidenceArticleNumbers, ", "),
			"Generated from CVDR analysis.",
		}
		if !hasMunicipalityCode {
			ruleNotes = append(ruleNotes, "municipality_code still needs confirmation.")
		}
		rules = append(rules, model.Rule{
			ID:               ruleID,
			MunicipalityCode: municipalityCodeOrTODO(match, hasMunicipalityCode),
			MunicipalityName: firstNonEmpty(candidate.MunicipalityName, draft.Jurisdiction.MunicipalityName),
			ValidFrom:        validFrom,
			AppliesTo:        candidate.AppliesTo,
			Calculation:      candidate.Calculation,
			Predicates:       clonePredicates(candidate.Predicates),
			Exemptions:       clonePredicates(candidate.Exemptions),
			Source: model.Source{
				SourceURL:  firstNonEmpty(analysis.Source.PreferredURL, analysis.Source.PublicationXMLURL),
				CVDRID:     analysis.Source.CVDRID,
				ScrapedAt:  stringPtr(time.Now().UTC().Format(time.RFC3339)),
				ReviewedAt: nil,
				Reviewer:   "",
			},
			Confidence: "scraped",
			Notes:      joinNonEmpty(ruleNotes...),
		})
	}

	notes := []string{
		"Auto-generated from CVDR publication analysis.",
		"Source-backed draft only; review required before conformance use.",
	}
	if !hasMunicipalityCode {
		notes = append(notes, "municipality_code is TODO until canonical municipal code mapping is added.")
	}
	if len(analysis.Warnings) > 0 {
		notes = append(notes, "Analyzer warnings: "+strings.Join(analysis.Warnings, " | "))
	}

	return model.RuleSet{
		ID:        fmt.Sprintf("nl-%s-%s", slug, datePart),
		Domain:    "tourist_tax",
		Lifecycle: "draft",
		Jurisdiction: model.Jurisdiction{
			CountryCode: draft.Jurisdiction.CountryCode,
			CountryName: draft.Jurisdiction.CountryName,
		},
		Notes:            strings.Join(notes, " "),
		AssessmentPolicy: cloneAssessmentPolicy(analysis.AssessmentPolicyCandidate),
		Rules:            rules,
	}
}

func municipalityCodeOrTODO(match MunicipalityMatch, ok bool) string {
	if ok {
		return match.Entry.Code
	}
	return "TODO"
}

func canOverwriteGeneratedFixture(path string) bool {
	var ruleset struct {
		Lifecycle string `json:"lifecycle"`
		Notes     string `json:"notes"`
	}
	if err := readJSON(path, &ruleset); err != nil {
		return false
	}
	return ruleset.Lifecycle == "draft" && strings.Contains(ruleset.Notes, "Auto-generated from CVDR publication analysis.")
}

func reconcileGeneratedFixtureValidityWindows(root string) error {
	municipalityDirs, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, municipalityDir := range municipalityDirs {
		if !municipalityDir.IsDir() {
			continue
		}
		pattern := filepath.Join(root, municipalityDir.Name(), "*.json")
		paths, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		sort.Strings(paths)

		type fixture struct {
			Path      string
			StartDate string
			RuleSet   model.RuleSet
		}
		fixtures := []fixture{}
		for _, path := range paths {
			var ruleset model.RuleSet
			if err := readJSON(path, &ruleset); err != nil {
				return err
			}
			if ruleset.Lifecycle != "draft" || !strings.Contains(ruleset.Notes, "Auto-generated from CVDR publication analysis.") {
				continue
			}
			startDate := filepath.Base(strings.TrimSuffix(path, filepath.Ext(path)))
			if len(ruleset.Rules) > 0 && strings.TrimSpace(ruleset.Rules[0].ValidFrom) != "" {
				startDate = ruleset.Rules[0].ValidFrom
			}
			fixtures = append(fixtures, fixture{
				Path:      path,
				StartDate: startDate,
				RuleSet:   ruleset,
			})
		}

		sort.Slice(fixtures, func(i, j int) bool {
			return fixtures[i].StartDate < fixtures[j].StartDate
		})

		for i := 0; i < len(fixtures)-1; i++ {
			nextStart, err := time.Parse("2006-01-02", fixtures[i+1].StartDate)
			if err != nil {
				continue
			}
			validTo := nextStart.AddDate(0, 0, -1).Format("2006-01-02")
			changed := false
			for idx := range fixtures[i].RuleSet.Rules {
				if fixtures[i].RuleSet.Rules[idx].ValidTo == nil || *fixtures[i].RuleSet.Rules[idx].ValidTo != validTo {
					fixtures[i].RuleSet.Rules[idx].ValidTo = stringPtr(validTo)
					changed = true
				}
			}
			if !changed {
				continue
			}
			if err := writeJSONFile(fixtures[i].Path, fixtures[i].RuleSet); err != nil {
				return err
			}
		}
	}
	return nil
}

func fixtureDate(draft DraftStub) string {
	return firstNonEmpty(draft.Source.EffectiveFrom, draft.Source.Issued, time.Now().UTC().Format("2006-01-02"))
}

func cloneAssessmentPolicy(policy *model.AssessmentPolicy) *model.AssessmentPolicy {
	if policy == nil {
		return nil
	}
	out := &model.AssessmentPolicy{
		Period: policy.Period,
		Notes:  policy.Notes,
	}
	if policy.MinimumAssessmentAmount != nil {
		out.MinimumAssessmentAmount = &model.MinimumAmountPolicy{
			Amount:   policy.MinimumAssessmentAmount.Amount,
			Currency: policy.MinimumAssessmentAmount.Currency,
		}
	}
	return out
}

func stringPtr(value string) *string {
	return &value
}

func joinNonEmpty(values ...string) string {
	out := []string{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return strings.Join(out, " ")
}
