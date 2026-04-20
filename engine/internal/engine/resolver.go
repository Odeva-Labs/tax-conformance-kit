package engine

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/odeva-labs/tax-conformance-kit/engine/internal/model"
)

type ResolveRuleSetRequest struct {
	FixtureRoot string
	Domain      string
}

type ResolvedRuleSet struct {
	Path    string
	RuleSet model.RuleSet
}

type candidateRuleSet struct {
	resolved       ResolvedRuleSet
	score          int
	matchingRuleID string
}

type indexedRuleSet struct {
	resolved ResolvedRuleSet
	domain   string
}

type fixtureRuleSetIndex struct {
	rulesets []indexedRuleSet
}

type fixtureRuleSetIndexCacheEntry struct {
	once  sync.Once
	index fixtureRuleSetIndex
	err   error
}

var fixtureRuleSetIndexCache sync.Map

func ResolveRuleSet(input model.BookingInput, req ResolveRuleSetRequest) (ResolvedRuleSet, error) {
	if req.FixtureRoot == "" {
		return ResolvedRuleSet{}, fmt.Errorf("fixture root is required")
	}
	if _, err := time.Parse("2006-01-02", input.StayDate); err != nil {
		return ResolvedRuleSet{}, fmt.Errorf("invalid stay_date")
	}

	property := input.EffectivePropertyLocation()
	if property.CountryCode == "" {
		return ResolvedRuleSet{}, fmt.Errorf("property_location.country_code is required for ruleset resolution")
	}

	domain := req.Domain
	if domain == "" {
		domain = "tourist_tax"
	}

	searchRoot := resolveFixtureSearchRoot(req.FixtureRoot, property.CountryCode)
	index, err := loadFixtureRuleSetIndex(searchRoot)
	if err != nil {
		return ResolvedRuleSet{}, err
	}

	candidates := make([]candidateRuleSet, 0)
	for _, entry := range index.rulesets {
		if entry.domain != "" && entry.domain != domain {
			continue
		}

		rs := entry.resolved.RuleSet
		score := -1
		matchingRuleID := ""
		for _, rule := range rs.Rules {
			ok, ruleScore := ruleMatchesResolution(rule, rs.Jurisdiction, input)
			if !ok {
				continue
			}
			if ruleScore > score {
				score = ruleScore
				matchingRuleID = rule.ID
			}
		}
		if score < 0 {
			continue
		}

		candidates = append(candidates, candidateRuleSet{
			resolved:       entry.resolved,
			score:          score,
			matchingRuleID: matchingRuleID,
		})
	}
	if len(candidates) == 0 {
		return ResolvedRuleSet{}, fmt.Errorf("no ruleset matched property location %s in %s", property.CountryCode, filepath.ToSlash(req.FixtureRoot))
	}

	slices.SortFunc(candidates, func(a, b candidateRuleSet) int {
		switch {
		case a.score != b.score:
			return b.score - a.score
		case a.resolved.Path < b.resolved.Path:
			return -1
		case a.resolved.Path > b.resolved.Path:
			return 1
		default:
			return 0
		}
	})

	best := candidates[0]
	if len(candidates) > 1 && candidates[1].score == best.score && candidates[1].resolved.Path != best.resolved.Path {
		return ResolvedRuleSet{}, fmt.Errorf(
			"ambiguous ruleset resolution between %s (%s) and %s (%s)",
			best.resolved.Path,
			best.matchingRuleID,
			candidates[1].resolved.Path,
			candidates[1].matchingRuleID,
		)
	}

	return best.resolved, nil
}

func loadFixtureRuleSetIndex(root string) (fixtureRuleSetIndex, error) {
	cacheKey := filepath.Clean(root)
	entryValue, _ := fixtureRuleSetIndexCache.LoadOrStore(cacheKey, &fixtureRuleSetIndexCacheEntry{})
	entry := entryValue.(*fixtureRuleSetIndexCacheEntry)
	entry.once.Do(func() {
		entry.index, entry.err = buildFixtureRuleSetIndex(cacheKey)
	})
	return entry.index, entry.err
}

func buildFixtureRuleSetIndex(root string) (fixtureRuleSetIndex, error) {
	index := fixtureRuleSetIndex{
		rulesets: make([]indexedRuleSet, 0),
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".json" || strings.Contains(filepath.ToSlash(path), "/conformance/") {
			return nil
		}

		rs, err := readRuleSetFile(path)
		if err != nil {
			return err
		}

		index.rulesets = append(index.rulesets, indexedRuleSet{
			resolved: ResolvedRuleSet{
				Path:    filepath.ToSlash(path),
				RuleSet: rs,
			},
			domain: rs.Domain,
		})
		return nil
	})
	if err != nil {
		return fixtureRuleSetIndex{}, err
	}

	return index, nil
}

func resolveFixtureSearchRoot(fixtureRoot, countryCode string) string {
	countryDir := filepath.Join(fixtureRoot, strings.ToLower(strings.TrimSpace(countryCode)))
	info, err := os.Stat(countryDir)
	if err == nil && info.IsDir() {
		return countryDir
	}
	return fixtureRoot
}

func ruleMatchesResolution(rule model.Rule, jurisdiction model.Jurisdiction, input model.BookingInput) (bool, int) {
	property := input.EffectivePropertyLocation()
	scope := rule.EffectiveLocationScope(jurisdiction)
	if !matchesLocation(scope, property) {
		return false, 0
	}
	if len(rule.AppliesTo.AccommodationTypes) > 0 && !slices.Contains(rule.AppliesTo.AccommodationTypes, input.AccommodationType) {
		return false, 0
	}

	stayDate, err := time.Parse("2006-01-02", input.StayDate)
	if err != nil || !dateInRange(stayDate, rule.ValidFrom, rule.ValidTo) {
		return false, 0
	}

	for _, predicate := range rule.Predicates {
		matches, err := evalPredicate(predicate, input)
		if err != nil || !matches {
			return false, 0
		}
	}

	return true, resolutionScore(scope, rule)
}

func resolutionScore(scope model.Location, rule model.Rule) int {
	score := 0
	switch {
	case scope.LocalityCode != "":
		score += 400
	case scope.LocalityName != "":
		score += 350
	case scope.RegionCode != "":
		score += 300
	case scope.CountryCode != "":
		score += 200
	}

	if len(rule.AppliesTo.AccommodationTypes) > 0 {
		score += 10
	}
	if rule.ValidFrom != "" {
		score++
	}
	return score
}

func readRuleSetFile(path string) (model.RuleSet, error) {
	var rs model.RuleSet
	body, err := os.ReadFile(path)
	if err != nil {
		return model.RuleSet{}, err
	}
	if err := json.Unmarshal(body, &rs); err != nil {
		return model.RuleSet{}, err
	}
	return rs, nil
}
