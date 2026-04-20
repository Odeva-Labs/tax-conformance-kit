package cvdr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildCoverageReport(t *testing.T) {
	t.Parallel()

	selectionDir := t.TempDir()
	if err := ExportSelection(selectionDir, SelectionManifest{
		Works: []WorkSelection{
			{
				CVDRID:              "CVDR100",
				Creator:             "Amsterdam",
				RepresentativeTitle: "Verordening toeristenbelasting 2026",
				CurrentCandidate: Candidate{
					Identifier:    "CVDR100_1",
					PreferredURL:  "https://example.invalid/CVDR100/1",
					EffectiveFrom: "2026-01-01",
				},
			},
			{
				CVDRID:              "CVDR200",
				Creator:             "'s-Gravenhage",
				RepresentativeTitle: "Verordening op de heffing en invordering van toeristenbelasting 2026",
				CurrentCandidate: Candidate{
					Identifier:   "CVDR200_1",
					PreferredURL: "https://example.invalid/CVDR200/1",
				},
			},
		},
	}); err != nil {
		t.Fatalf("export selection: %v", err)
	}

	fixtureRoot := t.TempDir()
	for _, slug := range []string{"amsterdam", "breda"} {
		if err := os.MkdirAll(filepath.Join(fixtureRoot, slug), 0o755); err != nil {
			t.Fatalf("mkdir fixture dir: %v", err)
		}
		if err := writeJSONFile(filepath.Join(fixtureRoot, slug, "2026-01-01.json"), map[string]any{"id": slug}); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}

	report, err := BuildCoverageReport(CoverageReportRequest{
		SelectionDir: selectionDir,
		FixtureRoot:  fixtureRoot,
	})
	if err != nil {
		t.Fatalf("build report: %v", err)
	}

	if report.CurrentCandidateWorkCount != 2 {
		t.Fatalf("expected 2 current candidates, got %d", report.CurrentCandidateWorkCount)
	}
	if report.SelectionMunicipalityCount != 2 {
		t.Fatalf("expected 2 municipalities in selection, got %d", report.SelectionMunicipalityCount)
	}
	if report.CoveredMunicipalityCount != 1 {
		t.Fatalf("expected 1 covered municipality, got %d", report.CoveredMunicipalityCount)
	}
	if report.MissingMunicipalityCount != 1 {
		t.Fatalf("expected 1 missing municipality, got %d", report.MissingMunicipalityCount)
	}
	if report.ExtraFixtureCount != 1 {
		t.Fatalf("expected 1 extra fixture, got %d", report.ExtraFixtureCount)
	}
	if got := report.MissingMunicipalities[0].Slug; got != "s_gravenhage" {
		t.Fatalf("expected slug s_gravenhage, got %s", got)
	}
	if got := report.MissingMunicipalities[0].WorkCount; got != 1 {
		t.Fatalf("expected missing work count 1, got %d", got)
	}
	if got := report.ExtraFixtures[0].Slug; got != "breda" {
		t.Fatalf("expected extra fixture breda, got %s", got)
	}
}
