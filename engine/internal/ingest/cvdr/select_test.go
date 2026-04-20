package cvdr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectCandidatesChoosesCurrentAndHistoricalVersions(t *testing.T) {
	dir := t.TempDir()
	mustWriteWorkFile(t, dir, WorkFile{
		Work: WorkTimeline{
			CVDRID:              "CVDR100",
			Creator:             "Amsterdam",
			RepresentativeTitle: "Verordening toeristenbelasting",
		},
		Versions: []Record{
			{
				Identifier:     "CVDR100_1",
				PreferredURL:   "https://example/CVDR100/1",
				Issued:         "2023-12-20",
				EffectiveFrom:  "2024-01-01",
				EffectiveTo:    "2025-01-01",
				ChangeCategory: "new_regulation",
			},
			{
				Identifier:     "CVDR100_2",
				PreferredURL:   "https://example/CVDR100/2",
				Issued:         "2024-11-14",
				EffectiveFrom:  "2025-01-01",
				ChangeCategory: "amendment",
			},
		},
	})

	got, err := SelectCandidates(SelectionRequest{
		IndexDir:             dir,
		AsOfDate:             "2025-06-01",
		MaxHistoricalPerWork: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WorkCount != 1 || got.CurrentCandidateCount != 1 {
		t.Fatalf("unexpected manifest counts: %+v", got)
	}
	if got.Works[0].CurrentCandidate.Identifier != "CVDR100_2" {
		t.Fatalf("expected current candidate CVDR100_2, got %s", got.Works[0].CurrentCandidate.Identifier)
	}
	if len(got.Works[0].HistoricalCandidates) == 0 || got.Works[0].HistoricalCandidates[0].Identifier != "CVDR100_1" {
		t.Fatalf("expected historical candidate CVDR100_1, got %+v", got.Works[0].HistoricalCandidates)
	}
}

func TestSelectCandidatesSkipsSupportingTouristTaxInstruments(t *testing.T) {
	dir := t.TempDir()
	mustWriteWorkFile(t, dir, WorkFile{
		Work: WorkTimeline{
			CVDRID:              "CVDR200",
			Creator:             "Assen",
			RepresentativeTitle: "Uitvoeringsregeling toeristenbelasting",
		},
		Versions: []Record{
			{
				Identifier:     "CVDR200_1",
				Title:          "Uitvoeringsregeling toeristenbelasting",
				Alternative:    "Uitvoeringsregeling toeristenbelasting gemeente Assen",
				RatifiedBy:     "college van burgemeester en wethouders",
				EffectiveFrom:  "2020-05-01",
				ChangeCategory: "new_regulation",
			},
		},
	})

	got, err := SelectCandidates(SelectionRequest{
		IndexDir:             dir,
		AsOfDate:             "2025-06-01",
		MaxHistoricalPerWork: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WorkCount != 0 || got.CurrentCandidateCount != 0 {
		t.Fatalf("expected supporting instrument work to be skipped, got %+v", got)
	}
}

func TestSelectCandidatesKeepsLongFormMunicipalOrdinances(t *testing.T) {
	dir := t.TempDir()
	mustWriteWorkFile(t, dir, WorkFile{
		Work: WorkTimeline{
			CVDRID:              "CVDR300",
			Creator:             "Arnhem",
			RepresentativeTitle: "Verordening op de heffing en de invordering van toeristenbelasting Arnhem 2026",
		},
		Versions: []Record{
			{
				Identifier:     "CVDR300_1",
				Title:          "Verordening op de heffing en de invordering van toeristenbelasting Arnhem 2026",
				Alternative:    "Verordening toeristenbelasting Arnhem 2026",
				RatifiedBy:     "gemeenteraad",
				EffectiveFrom:  "2025-11-29",
				ChangeCategory: "new_regulation",
			},
		},
	})

	got, err := SelectCandidates(SelectionRequest{
		IndexDir:             dir,
		AsOfDate:             "2026-04-17",
		MaxHistoricalPerWork: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WorkCount != 1 || got.Works[0].CurrentCandidate.Identifier != "CVDR300_1" {
		t.Fatalf("expected long-form ordinance to be kept, got %+v", got)
	}
}

func TestExportSelectionWritesFiles(t *testing.T) {
	dir := t.TempDir()
	selection := SelectionManifest{
		SelectedAt:               "2026-04-17T00:00:00Z",
		IndexDir:                 "/tmp/index",
		AsOfDate:                 "2026-04-17",
		MaxHistoricalPerWork:     3,
		WorkCount:                1,
		CurrentCandidateCount:    1,
		HistoricalCandidateCount: 1,
		Works: []WorkSelection{
			{
				CVDRID:              "CVDR100",
				Creator:             "Amsterdam",
				RepresentativeTitle: "Verordening toeristenbelasting",
				CurrentCandidate: Candidate{
					Identifier:       "CVDR100_2",
					SelectionReasons: []string{"latest_active_version"},
				},
				HistoricalCandidates: []Candidate{
					{Identifier: "CVDR100_1", SelectionReasons: []string{"earliest_version"}},
				},
			},
		},
	}

	if err := ExportSelection(dir, selection); err != nil {
		t.Fatalf("unexpected export error: %v", err)
	}

	assertFileExists(t, filepath.Join(dir, "manifest.json"))
	assertFileExists(t, filepath.Join(dir, "work_index.json"))
	assertFileExists(t, filepath.Join(dir, "works", "CVDR100.json"))
}

func mustWriteWorkFile(t *testing.T, dir string, workFile WorkFile) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "works"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := writeJSONFile(filepath.Join(dir, "works", workFile.Work.CVDRID+".json"), workFile); err != nil {
		t.Fatalf("write work file: %v", err)
	}
}
