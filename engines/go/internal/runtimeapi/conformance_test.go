package runtimeapi

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ramones/tax-conformance-kit/engines/go/internal/model"
)

func TestConformanceFixtures(t *testing.T) {
	repoRoot := findRepoRoot(t)
	registry := readKindRegistry(t, filepath.Join(repoRoot, "core", "schemas", "kind-registry.v1.json"))

	casePaths := make([]string, 0)
	err := filepath.WalkDir(filepath.Join(repoRoot, "core", "fixtures"), func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		if !strings.Contains(filepath.ToSlash(path), "/conformance/") {
			return nil
		}
		casePaths = append(casePaths, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk conformance fixtures: %v", err)
	}
	if len(casePaths) == 0 {
		t.Fatal("expected at least one conformance fixture")
	}

	for _, casePath := range casePaths {
		casePath := casePath
		t.Run(strings.TrimPrefix(filepath.ToSlash(casePath), filepath.ToSlash(repoRoot)+"/"), func(t *testing.T) {
			if strings.HasSuffix(casePath, ".assessment.json") {
				runAssessmentConformanceCase(t, casePath, registry)
				return
			}
			runBookingConformanceCase(t, casePath, registry)
		})
	}
}

func runBookingConformanceCase(t *testing.T, casePath string, registry model.KindRegistry) {
	cc := readBookingCase(t, casePath)
	rulesetPath := filepath.Join(filepath.Dir(casePath), cc.RuleSetPath)
	ruleset := readRuleSet(t, rulesetPath)

	response := Evaluate(model.RuntimeEvaluateRequest{
		RuleSet:      ruleset,
		BookingInput: cc.BookingInput,
	}, registry)
	if !response.OK {
		t.Fatalf("expected OK response, got %+v", response)
	}
	if response.Result == nil {
		t.Fatal("expected evaluation result")
	}
	if response.Result.TotalTax != cc.Expected.TotalTax {
		t.Fatalf("expected total tax %v, got %v", cc.Expected.TotalTax, response.Result.TotalTax)
	}
	if len(response.Result.MatchedRuleIDs) != len(cc.Expected.MatchedRuleIDs) {
		t.Fatalf("expected matched rule ids %+v, got %+v", cc.Expected.MatchedRuleIDs, response.Result.MatchedRuleIDs)
	}
	for i := range cc.Expected.MatchedRuleIDs {
		if response.Result.MatchedRuleIDs[i] != cc.Expected.MatchedRuleIDs[i] {
			t.Fatalf("expected matched rule ids %+v, got %+v", cc.Expected.MatchedRuleIDs, response.Result.MatchedRuleIDs)
		}
	}
}

func runAssessmentConformanceCase(t *testing.T, casePath string, registry model.KindRegistry) {
	ac := readAssessmentCase(t, casePath)
	rulesetPath := filepath.Join(filepath.Dir(casePath), ac.RuleSetPath)
	ruleset := readRuleSet(t, rulesetPath)

	response := EvaluateAssessment(model.RuntimeEvaluateAssessmentRequest{
		RuleSet:         ruleset,
		AssessmentInput: ac.AssessmentInput,
	}, registry)
	if !response.OK {
		t.Fatalf("expected OK response, got %+v", response)
	}
	if response.Result == nil {
		t.Fatal("expected assessment result")
	}
	if response.Result.TotalBookingTax != ac.Expected.TotalBookingTax {
		t.Fatalf("expected total booking tax %v, got %v", ac.Expected.TotalBookingTax, response.Result.TotalBookingTax)
	}
	if response.Result.TotalAssessmentTax != ac.Expected.TotalAssessmentTax {
		t.Fatalf("expected total assessment tax %v, got %v", ac.Expected.TotalAssessmentTax, response.Result.TotalAssessmentTax)
	}
	if len(response.Result.BookingResults) != len(ac.Expected.BookingResults) {
		t.Fatalf("expected booking results %+v, got %+v", ac.Expected.BookingResults, response.Result.BookingResults)
	}
	for i := range ac.Expected.BookingResults {
		got := response.Result.BookingResults[i]
		want := ac.Expected.BookingResults[i]
		if got.TotalTax != want.TotalTax {
			t.Fatalf("booking %d expected total tax %v, got %v", i, want.TotalTax, got.TotalTax)
		}
		if len(got.MatchedRuleIDs) != len(want.MatchedRuleIDs) {
			t.Fatalf("booking %d expected matched rule ids %+v, got %+v", i, want.MatchedRuleIDs, got.MatchedRuleIDs)
		}
		for j := range want.MatchedRuleIDs {
			if got.MatchedRuleIDs[j] != want.MatchedRuleIDs[j] {
				t.Fatalf("booking %d expected matched rule ids %+v, got %+v", i, want.MatchedRuleIDs, got.MatchedRuleIDs)
			}
		}
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	dir := wd
	for {
		candidate := filepath.Join(dir, "core", "schemas", "kind-registry.v1.json")
		if _, err := os.Stat(candidate); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repo root")
		}
		dir = parent
	}
}

func readKindRegistry(t *testing.T, path string) model.KindRegistry {
	t.Helper()
	var registry model.KindRegistry
	readJSON(t, path, &registry)
	return registry
}

func readRuleSet(t *testing.T, path string) model.RuleSet {
	t.Helper()
	var ruleset model.RuleSet
	readJSON(t, path, &ruleset)
	return ruleset
}

func readBookingCase(t *testing.T, path string) model.ConformanceCase {
	t.Helper()
	var cc model.ConformanceCase
	readJSON(t, path, &cc)
	return cc
}

func readAssessmentCase(t *testing.T, path string) model.AssessmentCase {
	t.Helper()
	var ac model.AssessmentCase
	readJSON(t, path, &ac)
	return ac
}

func readJSON(t *testing.T, path string, target any) {
	t.Helper()
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(bytes, target); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
}
