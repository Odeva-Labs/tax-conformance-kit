package cvdr

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ramones/tax-conformance-kit/engines/go/internal/model"
)

type AnalyzeRequest struct {
	ExtractionDir string
}

type AnalysisManifest struct {
	AnalyzedAt                   string              `json:"analyzed_at"`
	ExtractionDir                string              `json:"extraction_dir"`
	BundleCount                  int                 `json:"bundle_count"`
	AnalyzedBundleCount          int                 `json:"analyzed_bundle_count"`
	BundleWithCandidateRuleCount int                 `json:"bundle_with_candidate_rule_count"`
	TotalCandidateRuleCount      int                 `json:"total_candidate_rule_count"`
	AssessmentPolicyBundleCount  int                 `json:"assessment_policy_bundle_count"`
	WarningCount                 int                 `json:"warning_count"`
	Bundles                      []BundleAnalysisRef `json:"bundles,omitempty"`
}

type BundleAnalysisRef struct {
	CVDRID             string `json:"cvdr_id"`
	Identifier         string `json:"identifier"`
	AnalysisPath       string `json:"analysis_path"`
	CandidateRuleCount int    `json:"candidate_rule_count"`
	WarningCount       int    `json:"warning_count"`
}

type BundleAnalysis struct {
	ArtifactType              string                  `json:"artifact_type"`
	Status                    string                  `json:"status"`
	AnalyzedAt                string                  `json:"analyzed_at"`
	Source                    DraftSource             `json:"source"`
	ArticleCount              int                     `json:"article_count"`
	Articles                  []ArticleSummary        `json:"articles"`
	GlobalExemptions          []model.Predicate       `json:"global_exemptions,omitempty"`
	CandidateRules            []RuleCandidate         `json:"candidate_rules,omitempty"`
	AssessmentPolicyCandidate *model.AssessmentPolicy `json:"assessment_policy_candidate,omitempty"`
	Warnings                  []string                `json:"warnings,omitempty"`
}

type ArticleSummary struct {
	Number     string            `json:"number,omitempty"`
	Title      string            `json:"title,omitempty"`
	Role       string            `json:"role,omitempty"`
	Text       string            `json:"text"`
	Paragraphs []string          `json:"paragraphs,omitempty"`
	ListItems  []ArticleListItem `json:"list_items,omitempty"`
}

type ArticleListItem struct {
	Label string `json:"label,omitempty"`
	Text  string `json:"text"`
}

type RuleCandidate struct {
	ID                     string            `json:"id"`
	MunicipalityName       string            `json:"municipality_name,omitempty"`
	ValidFrom              string            `json:"valid_from,omitempty"`
	AppliesTo              model.AppliesTo   `json:"applies_to"`
	Calculation            model.Calculation `json:"calculation"`
	Predicates             []model.Predicate `json:"predicates,omitempty"`
	Exemptions             []model.Predicate `json:"exemptions,omitempty"`
	EvidenceArticleNumbers []string          `json:"evidence_article_numbers,omitempty"`
	Confidence             string            `json:"confidence"`
	Notes                  string            `json:"notes,omitempty"`
}

var (
	rePercent   = regexp.MustCompile(`(?i)([0-9]+(?:[.,][0-9]+)?)\s*(?:%|procent)`)
	reEuro      = regexp.MustCompile(`€\s*([0-9]+(?:[.,][0-9]+)?)`)
	reAgePhrase = regexp.MustCompile(`(?i)leeftijd van ([a-z0-9\-]+)`)
)

type rateUnit string

const (
	rateUnitUnknown           rateUnit = ""
	rateUnitPercentage        rateUnit = "percentage"
	rateUnitPerPersonPerNight rateUnit = "per_person_per_night"
)

func AnalyzeExtractedBundles(req AnalyzeRequest) (AnalysisManifest, error) {
	if req.ExtractionDir == "" {
		return AnalysisManifest{}, fmt.Errorf("extraction directory is required")
	}

	paths, err := filepath.Glob(filepath.Join(req.ExtractionDir, "bundles", "*", "*", "draft.json"))
	if err != nil {
		return AnalysisManifest{}, err
	}
	sort.Strings(paths)

	manifest := AnalysisManifest{
		AnalyzedAt:    time.Now().UTC().Format(time.RFC3339),
		ExtractionDir: req.ExtractionDir,
		BundleCount:   len(paths),
		Bundles:       make([]BundleAnalysisRef, 0, len(paths)),
	}

	for _, draftPath := range paths {
		bundleDir := filepath.Dir(draftPath)
		publicationPath := filepath.Join(bundleDir, "publication.xml")
		var draft DraftStub
		if err := readJSON(draftPath, &draft); err != nil {
			return AnalysisManifest{}, err
		}

		analysis, err := analyzeBundle(publicationPath, draft)
		if err != nil {
			return AnalysisManifest{}, fmt.Errorf("%s: %w", bundleDir, err)
		}
		analysisPath := filepath.Join(bundleDir, "analysis.json")
		if err := writeJSONFile(analysisPath, analysis); err != nil {
			return AnalysisManifest{}, err
		}

		manifest.AnalyzedBundleCount++
		if len(analysis.CandidateRules) > 0 {
			manifest.BundleWithCandidateRuleCount++
		}
		manifest.TotalCandidateRuleCount += len(analysis.CandidateRules)
		if analysis.AssessmentPolicyCandidate != nil {
			manifest.AssessmentPolicyBundleCount++
		}
		manifest.WarningCount += len(analysis.Warnings)
		manifest.Bundles = append(manifest.Bundles, BundleAnalysisRef{
			CVDRID:             draft.Source.CVDRID,
			Identifier:         draft.Source.Identifier,
			AnalysisPath:       filepath.ToSlash(strings.TrimPrefix(analysisPath, req.ExtractionDir+"/")),
			CandidateRuleCount: len(analysis.CandidateRules),
			WarningCount:       len(analysis.Warnings),
		})
	}

	manifestOut := manifest
	manifestOut.Bundles = nil
	if err := writeJSONFile(filepath.Join(req.ExtractionDir, "analysis_manifest.json"), manifestOut); err != nil {
		return AnalysisManifest{}, err
	}

	return manifest, nil
}

func analyzeBundle(publicationPath string, draft DraftStub) (BundleAnalysis, error) {
	body, err := os.ReadFile(publicationPath)
	if err != nil {
		return BundleAnalysis{}, err
	}
	articles, err := parseArticleSummaries(body)
	if err != nil {
		return BundleAnalysis{}, err
	}

	globalExemptions, warnings := deriveGlobalExemptions(articles)
	candidateRules, ruleWarnings := deriveRuleCandidates(draft, articles, globalExemptions)
	warnings = append(warnings, ruleWarnings...)
	assessmentPolicy, assessmentWarnings := deriveAssessmentPolicy(articles)
	warnings = append(warnings, assessmentWarnings...)
	warnings = dedupeStrings(warnings)

	return BundleAnalysis{
		ArtifactType:              "cvdr_bundle_analysis",
		Status:                    "heuristic_candidates",
		AnalyzedAt:                time.Now().UTC().Format(time.RFC3339),
		Source:                    draft.Source,
		ArticleCount:              len(articles),
		Articles:                  articles,
		GlobalExemptions:          globalExemptions,
		CandidateRules:            candidateRules,
		AssessmentPolicyCandidate: assessmentPolicy,
		Warnings:                  warnings,
	}, nil
}

func parseArticleSummaries(body []byte) ([]ArticleSummary, error) {
	var root xmlNode
	if err := xml.Unmarshal(body, &root); err != nil {
		return nil, err
	}

	regelText, ok := firstPath(root, "body", "regeling", "regeling-tekst")
	if !ok {
		return nil, fmt.Errorf("publication xml has no regeling-tekst")
	}
	articleNodes := childNodesByName(regelText, "artikel")
	out := make([]ArticleSummary, 0, len(articleNodes))
	for _, articleNode := range articleNodes {
		summary := summarizeArticle(articleNode)
		if summary.Text == "" && len(summary.Paragraphs) == 0 && len(summary.ListItems) == 0 {
			continue
		}
		out = append(out, summary)
	}
	return out, nil
}

func summarizeArticle(article xmlNode) ArticleSummary {
	number := nestedText(article, "kop", "nr")
	title := nestedText(article, "kop", "titel")
	paragraphs := collectArticleParagraphs(article)
	listItems := collectArticleListItems(article)
	textParts := append([]string{}, paragraphs...)
	for _, item := range listItems {
		if item.Label != "" {
			textParts = append(textParts, item.Label+" "+item.Text)
		} else {
			textParts = append(textParts, item.Text)
		}
	}

	return ArticleSummary{
		Number:     number,
		Title:      title,
		Role:       classifyArticleRole(title),
		Text:       normalizeSpace(strings.Join(textParts, " ")),
		Paragraphs: paragraphs,
		ListItems:  listItems,
	}
}

func collectArticleParagraphs(node xmlNode) []string {
	out := []string{}
	var walk func(xmlNode, bool)
	walk = func(current xmlNode, inList bool) {
		switch current.XMLName.Local {
		case "kop":
			return
		case "li":
			inList = true
		case "al":
			if !inList {
				text := normalizeSpace(flattenText(current))
				if text != "" {
					out = append(out, text)
				}
			}
		}
		for _, child := range current.Children {
			walk(child, inList)
		}
	}
	walk(node, false)
	return dedupePreserveOrder(out)
}

func collectArticleListItems(node xmlNode) []ArticleListItem {
	out := []ArticleListItem{}
	var walk func(xmlNode)
	walk = func(current xmlNode) {
		if current.XMLName.Local == "li" {
			text := normalizeSpace(flattenText(current))
			if text != "" {
				out = append(out, ArticleListItem{
					Label: attrValue(current, "nr"),
					Text:  text,
				})
			}
			return
		}
		for _, child := range current.Children {
			walk(child)
		}
	}
	walk(node)
	return out
}

func flattenText(node xmlNode) string {
	parts := []string{}
	var walk func(xmlNode)
	walk = func(current xmlNode) {
		text := normalizeSpace(current.Content)
		if text != "" {
			parts = append(parts, text)
		}
		for _, child := range current.Children {
			walk(child)
		}
	}
	walk(node)
	return normalizeSpace(strings.Join(parts, " "))
}

func deriveGlobalExemptions(articles []ArticleSummary) ([]model.Predicate, []string) {
	exemptions := []model.Predicate{}
	warnings := []string{}

	taxable := articleByRole(articles, "taxable_event")
	if taxable != nil && strings.Contains(strings.ToLower(taxable.Text), "niet als ingezetene") {
		exemptions = append(exemptions, model.Predicate{Kind: "guest.resident_of_same_municipality"})
	}

	for _, article := range articles {
		if article.Role != "exemptions" {
			continue
		}
		lower := strings.ToLower(article.Text)
		if strings.Contains(lower, "wet toetreding zorgaanbieders") {
			exemptions = append(exemptions, model.Predicate{Kind: "stay.wtza_care_institution"})
		}
		if strings.Contains(lower, "toegelaten instelling") || strings.Contains(lower, "wet toelating zorginstellingen") {
			exemptions = append(exemptions, model.Predicate{Kind: "stay.wtza_care_institution"})
		}
		if strings.Contains(lower, "centraal orgaan opvang asielzoekers") || strings.Contains(lower, "centrale orgaan opvang asielzoekers") || strings.Contains(lower, "coa") {
			exemptions = append(exemptions, model.Predicate{Kind: "stay.coa_asylum_housing"})
		}
		if agePred, ok := inferAgeExemption(lower); ok {
			exemptions = append(exemptions, agePred)
		}
		if strings.Contains(lower, "forensenbelasting") {
			exemptions = append(exemptions, model.Predicate{
				Kind:   "cross_tax.already_subject_to",
				Params: map[string]any{"tax": "forensenbelasting"},
			})
		}
		if strings.Contains(lower, "watertoeristenbelasting") {
			exemptions = append(exemptions, model.Predicate{
				Kind:   "cross_tax.already_subject_to",
				Params: map[string]any{"tax": "watertoeristenbelasting"},
			})
		}
	}

	exemptions = dedupePredicates(exemptions)
	if len(exemptions) == 0 {
		warnings = append(warnings, "No reusable exemptions inferred from taxable-event or vrijstellingen articles.")
	}
	return exemptions, warnings
}

func deriveRuleCandidates(draft DraftStub, articles []ArticleSummary, globalExemptions []model.Predicate) ([]RuleCandidate, []string) {
	baseArticle := articleByRole(articles, "base")
	rateArticle := articleByRole(articles, "rate")
	if rateArticle == nil {
		return nil, []string{"No tariff article detected; no candidate rules emitted."}
	}

	rules := []RuleCandidate{}
	warnings := []string{}
	validFrom := firstNonEmpty(draft.Source.EffectiveFrom, draft.Source.Issued)
	idBase := strings.TrimSuffix(filepath.Base(draft.SuggestedFixturePath), filepath.Ext(draft.SuggestedFixturePath))
	evidence := []string{}
	if rateArticle.Number != "" {
		evidence = append(evidence, rateArticle.Number)
	}
	if baseArticle != nil && baseArticle.Number != "" {
		evidence = append(evidence, baseArticle.Number)
	}

	if ratePct, ok := firstPercent(rateArticle); ok {
		base := inferBase(baseArticle)
		rules = append(rules, RuleCandidate{
			ID:               idBase + "-rate",
			MunicipalityName: draft.Jurisdiction.MunicipalityName,
			ValidFrom:        validFrom,
			AppliesTo:        model.AppliesTo{},
			Calculation: model.Calculation{
				Kind: "generic.percentage_of_base",
				Params: map[string]any{
					"rate_pct": ratePct,
					"base":     base,
				},
				Currency: "EUR",
			},
			Exemptions:             clonePredicates(globalExemptions),
			EvidenceArticleNumbers: dedupeStrings(evidence),
			Confidence:             "heuristic",
			Notes:                  "Generated from tariff and heffingsmaatstaf articles. Reviewer must confirm exact accommodation scope and municipality code.",
		})
	}

	perNightRules, perNightWarnings := derivePerNightRuleCandidates(idBase, draft, rateArticle, globalExemptions)
	rules = append(rules, perNightRules...)
	warnings = append(warnings, perNightWarnings...)

	if len(rules) == 0 {
		warnings = append(warnings, "Tariff article found, but no supported percentage/per-night pattern was recognized.")
	}
	return rules, warnings
}

func derivePerNightRuleCandidates(idBase string, draft DraftStub, rateArticle *ArticleSummary, globalExemptions []model.Predicate) ([]RuleCandidate, []string) {
	unit := detectRateUnit(rateArticle.Text)
	chunks := []string{}
	chunks = append(chunks, rateArticle.Paragraphs...)
	for _, item := range rateArticle.ListItems {
		chunks = append(chunks, item.Text)
	}
	if len(chunks) == 0 && rateArticle.Text != "" {
		chunks = append(chunks, rateArticle.Text)
	}

	rules := []RuleCandidate{}
	warnings := []string{}
	seen := map[string]struct{}{}
	for idx, chunk := range chunks {
		amount, kind, ok := inferRateAmountAndKind(chunk, unit)
		if !ok {
			continue
		}
		appliesTo, scopeWarning := inferAccommodationTypes(chunk)
		if scopeWarning != "" {
			warnings = append(warnings, scopeWarning)
		}
		if unit == rateUnitPerPersonPerNight && !hasExplicitNightUnit(chunk) && len(appliesTo) == 0 {
			continue
		}
		key := fmt.Sprintf("%.2f|%s", amount, strings.Join(appliesTo, ","))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		notes := "Generated from tariff article. Reviewer must confirm category boundaries and exclusions."
		if idx == 0 && len(rateArticle.ListItems) > 0 && len(appliesTo) == 0 {
			notes = "Likely default tariff from the opening paragraph; reviewer must exclude specific reduced categories mentioned later in the same article."
		}
		rules = append(rules, RuleCandidate{
			ID:               fmt.Sprintf("%s-rate-%d", idBase, len(rules)+1),
			MunicipalityName: draft.Jurisdiction.MunicipalityName,
			ValidFrom:        firstNonEmpty(draft.Source.EffectiveFrom, draft.Source.Issued),
			AppliesTo: model.AppliesTo{
				AccommodationTypes: appliesTo,
			},
			Calculation: model.Calculation{
				Kind: kind,
				Params: map[string]any{
					"amount": amount,
				},
				Currency: "EUR",
			},
			Exemptions:             clonePredicates(globalExemptions),
			EvidenceArticleNumbers: []string{rateArticle.Number},
			Confidence:             "heuristic",
			Notes:                  notes,
		})
	}

	if hasFixedPitchTariff(rateArticle.Text) {
		warnings = append(warnings, "Tariff article contains fixed standplaats amounts; draft fixture will need a separate fixed-per-pitch heuristic or kind.")
	}
	return rules, warnings
}

func deriveAssessmentPolicy(articles []ArticleSummary) (*model.AssessmentPolicy, []string) {
	policy := &model.AssessmentPolicy{}
	warnings := []string{}

	if periodArticle := articleByRole(articles, "assessment_period"); periodArticle != nil {
		lower := strings.ToLower(periodArticle.Text)
		periods := []string{}
		if strings.Contains(lower, "kalenderkwartaal") {
			periods = append(periods, "calendar_quarter")
		}
		if strings.Contains(lower, "kalenderjaar") {
			periods = append(periods, "calendar_year")
		}
		if strings.Contains(lower, "kalendermaand") {
			periods = append(periods, "calendar_month")
		}
		periods = dedupeStrings(periods)
		switch len(periods) {
		case 0:
		case 1:
			policy.Period = periods[0]
		default:
			warnings = append(warnings, "Assessment period article contains multiple periods; manual encoding required.")
		}
	}

	if thresholdArticle := articleByRole(articles, "assessment_threshold"); thresholdArticle != nil {
		if amount, ok := firstEuro(thresholdArticle.Text); ok {
			policy.MinimumAssessmentAmount = &model.MinimumAmountPolicy{
				Amount:   amount,
				Currency: "EUR",
			}
		}
	}

	if policy.Period == "" && policy.MinimumAssessmentAmount == nil {
		return nil, warnings
	}
	notes := []string{}
	if policy.Period != "" {
		notes = append(notes, "Assessment period inferred from the belastingtijdvak article.")
	}
	if policy.MinimumAssessmentAmount != nil {
		notes = append(notes, "Minimum assessment amount inferred from the aanslaggrens article.")
	}
	policy.Notes = strings.Join(notes, " ")
	return policy, warnings
}

func articleByRole(articles []ArticleSummary, role string) *ArticleSummary {
	for i := range articles {
		if articles[i].Role == role {
			return &articles[i]
		}
	}
	return nil
}

func classifyArticleRole(title string) string {
	lower := strings.ToLower(normalizeSpace(title))
	switch {
	case strings.Contains(lower, "belastbaar feit"):
		return "taxable_event"
	case strings.Contains(lower, "vrijstelling"):
		return "exemptions"
	case strings.Contains(lower, "maatstaf"):
		return "base"
	case strings.Contains(lower, "tarief"):
		return "rate"
	case strings.Contains(lower, "belastingtijdvak"):
		return "assessment_period"
	case strings.Contains(lower, "belastingjaar"):
		return "assessment_period"
	case strings.Contains(lower, "aanslaggrens"):
		return "assessment_threshold"
	case strings.Contains(lower, "wijze van heffing"):
		return "collection_method"
	default:
		return ""
	}
}

func inferBase(article *ArticleSummary) string {
	if article == nil {
		return "subtotal"
	}
	lower := strings.ToLower(article.Text)
	if strings.Contains(lower, "toeristenbelasting daaronder niet begrepen") || strings.Contains(lower, "belasting daaronder niet begrepen") {
		return "accommodation_fee_exclusive_of_tax"
	}
	if strings.Contains(lower, "vergoeding voor het verblijf") || strings.Contains(lower, "logies") {
		return "accommodation_fee_exclusive_of_tax"
	}
	return "subtotal"
}

func firstPercent(article *ArticleSummary) (float64, bool) {
	if article == nil {
		return 0, false
	}
	match := rePercent.FindStringSubmatch(article.Text)
	if len(match) < 2 {
		return 0, false
	}
	value, err := parseDutchDecimal(match[1])
	if err != nil {
		return 0, false
	}
	return value, true
}

func perPersonPerNightAmount(text string) (float64, bool) {
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "per persoon per overnachting") {
		return 0, false
	}
	return firstEuro(text)
}

func detectRateUnit(text string) rateUnit {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "procent"):
		return rateUnitPercentage
	case strings.Contains(lower, "per persoon per overnachting"),
		strings.Contains(lower, "per persoon, per overnachting"),
		strings.Contains(lower, "per overnachting €"),
		strings.Contains(lower, "€") && strings.Contains(lower, "per overnachting"),
		strings.Contains(lower, "per persoon per nacht"),
		strings.Contains(lower, "per persoon, per nacht"),
		strings.Contains(lower, "per nacht €"),
		strings.Contains(lower, "€") && strings.Contains(lower, "per nacht"):
		return rateUnitPerPersonPerNight
	default:
		return rateUnitUnknown
	}
}

func inferRateAmountAndKind(chunk string, inheritedUnit rateUnit) (float64, string, bool) {
	amount, ok := firstEuro(chunk)
	if !ok {
		return 0, "", false
	}

	lower := strings.ToLower(chunk)
	switch {
	case strings.Contains(lower, "procent"):
		return 0, "", false
	case strings.Contains(lower, "per persoon per overnachting"),
		strings.Contains(lower, "per persoon, per overnachting"),
		strings.Contains(lower, "per persoon per nacht"),
		strings.Contains(lower, "per persoon, per nacht"),
		strings.Contains(lower, "per overnachting"),
		strings.Contains(lower, "per nacht"):
		return amount, "generic.per_person_per_night", true
	case inheritedUnit == rateUnitPerPersonPerNight:
		return amount, "generic.per_person_per_night", true
	default:
		return 0, "", false
	}
}

func hasExplicitNightUnit(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "per persoon per overnachting") ||
		strings.Contains(lower, "per persoon, per overnachting") ||
		strings.Contains(lower, "per persoon per nacht") ||
		strings.Contains(lower, "per persoon, per nacht") ||
		strings.Contains(lower, "per overnachting") ||
		strings.Contains(lower, "per nacht")
}

func firstEuro(text string) (float64, bool) {
	match := reEuro.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, false
	}
	value, err := parseDutchDecimal(match[1])
	if err != nil {
		return 0, false
	}
	return value, true
}

func inferAccommodationTypes(text string) ([]string, string) {
	lower := strings.ToLower(text)
	accommodationTypes := []string{}
	if strings.Contains(lower, "camping") {
		accommodationTypes = append(accommodationTypes, "camping")
	}
	if strings.Contains(lower, "kampeerterrein") || strings.Contains(lower, "kampeerterreinen") {
		accommodationTypes = append(accommodationTypes, "camping")
	}
	if strings.Contains(lower, "haven") {
		accommodationTypes = append(accommodationTypes, "harbor")
	}
	if strings.Contains(lower, "vaartuig") || strings.Contains(lower, "vaartuigen") || strings.Contains(lower, "ligplaats") {
		accommodationTypes = append(accommodationTypes, "harbor")
	}
	if strings.Contains(lower, "vakantieverhuur") {
		accommodationTypes = append(accommodationTypes, "vacation_rental")
	}
	if strings.Contains(lower, "hotel") || strings.Contains(lower, "hotels") {
		accommodationTypes = append(accommodationTypes, "hotel")
	}
	if strings.Contains(lower, "groepsaccommod") {
		accommodationTypes = append(accommodationTypes, "group_accommodation")
	}
	if strings.Contains(lower, "recreatiewoning") {
		accommodationTypes = append(accommodationTypes, "recreation_home")
	}

	warning := ""
	if strings.Contains(lower, "in afwijking") && len(accommodationTypes) == 0 {
		warning = "Tariff exception detected without a recognized accommodation type; manual review required."
	}
	return dedupeStrings(accommodationTypes), warning
}

func hasFixedPitchTariff(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "vaste standplaats") || strings.Contains(lower, "standplaats")
}

func inferAgeExemption(text string) (model.Predicate, bool) {
	match := reAgePhrase.FindStringSubmatch(text)
	if len(match) < 2 {
		return model.Predicate{}, false
	}
	ageToken := strings.ToLower(match[1])
	age, ok := parseDutchIntegerWord(ageToken)
	if !ok {
		return model.Predicate{}, false
	}
	params := map[string]any{
		"max_age": age,
	}
	if strings.Contains(text, "eerste overnachting") || strings.Contains(text, "eerste nacht") {
		params["night_index"] = "first"
	}
	return model.Predicate{
		Kind:   "guest.age_below",
		Params: params,
	}, true
}

func parseDutchIntegerWord(value string) (int, bool) {
	if n, err := strconv.Atoi(value); err == nil {
		return n, true
	}
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, value)
	known := map[string]int{
		"een": 1, "eentje": 1,
		"twee":      2,
		"drie":      3,
		"vier":      4,
		"vijf":      5,
		"zes":       6,
		"zeven":     7,
		"acht":      8,
		"negen":     9,
		"tien":      10,
		"elf":       11,
		"twaalf":    12,
		"dertien":   13,
		"veertien":  14,
		"vijftien":  15,
		"zestien":   16,
		"zeventien": 17,
		"achttien":  18,
		"negentien": 19,
		"twintig":   20,
	}
	n, ok := known[normalized]
	return n, ok
}

func parseDutchDecimal(value string) (float64, error) {
	value = strings.ReplaceAll(value, ".", "")
	value = strings.ReplaceAll(value, ",", ".")
	return strconv.ParseFloat(value, 64)
}

func dedupePredicates(predicates []model.Predicate) []model.Predicate {
	out := []model.Predicate{}
	seen := map[string]struct{}{}
	for _, predicate := range predicates {
		paramsJSON, _ := json.Marshal(predicate.Params)
		key := predicate.Kind + "|" + string(paramsJSON)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, predicate)
	}
	return out
}

func clonePredicates(in []model.Predicate) []model.Predicate {
	out := make([]model.Predicate, 0, len(in))
	for _, predicate := range in {
		clone := model.Predicate{
			Kind:   predicate.Kind,
			Params: map[string]any{},
		}
		for key, value := range predicate.Params {
			clone.Params[key] = value
		}
		if predicate.Params == nil {
			clone.Params = nil
		}
		out = append(out, clone)
	}
	return out
}

func firstPath(root xmlNode, names ...string) (xmlNode, bool) {
	current := root
	for _, name := range names {
		next, ok := firstChildByName(current, name)
		if !ok {
			return xmlNode{}, false
		}
		current = next
	}
	return current, true
}

func firstChildByName(node xmlNode, name string) (xmlNode, bool) {
	for _, child := range node.Children {
		if child.XMLName.Local == name {
			return child, true
		}
	}
	return xmlNode{}, false
}

func childNodesByName(node xmlNode, name string) []xmlNode {
	out := []xmlNode{}
	for _, child := range node.Children {
		if child.XMLName.Local == name {
			out = append(out, child)
		}
	}
	return out
}

func nestedText(node xmlNode, names ...string) string {
	current := node
	for _, name := range names {
		next, ok := firstChildByName(current, name)
		if !ok {
			return ""
		}
		current = next
	}
	return normalizeSpace(flattenText(current))
}

func attrValue(node xmlNode, name string) string {
	for _, attr := range node.Attrs {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func normalizeSpace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func dedupePreserveOrder(values []string) []string {
	out := []string{}
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := normalizeSpace(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}
