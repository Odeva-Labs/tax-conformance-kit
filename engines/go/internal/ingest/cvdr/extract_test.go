package cvdr

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractStubsWritesCurrentBundle(t *testing.T) {
	indexDir := t.TempDir()
	selectionDir := t.TempDir()
	outputDir := t.TempDir()
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://example.test/xml/CVDR100_2.xml" {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Status:     "404 Not Found",
					Body:       io.NopCloser(bytes.NewBufferString("not found")),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(bytes.NewBufferString("<regeling id=\"CVDR100_2\"/>")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	mustWriteWorkFile(t, indexDir, WorkFile{
		Work: WorkTimeline{
			CVDRID:              "CVDR100",
			Creator:             "Amsterdam",
			RepresentativeTitle: "Verordening toeristenbelasting",
		},
		Versions: []Record{
			{
				CVDRID:            "CVDR100",
				Identifier:        "CVDR100_2",
				Title:             "Verordening toeristenbelasting Amsterdam 2025",
				Creator:           "Amsterdam",
				Issued:            "2024-11-14",
				EffectiveFrom:     "2025-01-01",
				PreferredURL:      "https://example/CVDR100/2",
				PreferredWorkURL:  "https://example/CVDR100",
				PublicationXMLURL: "https://example.test/xml/CVDR100_2.xml",
			},
		},
	})
	mustWriteSelectionManifest(t, selectionDir, SelectionManifest{
		SelectedAt:            "2026-04-17T00:00:00Z",
		IndexDir:              indexDir,
		AsOfDate:              "2026-04-17",
		WorkCount:             1,
		CurrentCandidateCount: 1,
		Works: []WorkSelection{
			{
				CVDRID:              "CVDR100",
				Creator:             "Amsterdam",
				RepresentativeTitle: "Verordening toeristenbelasting",
				CurrentCandidate: Candidate{
					Identifier:       "CVDR100_2",
					SelectionReasons: []string{"latest_active_version"},
				},
			},
		},
	})

	result, err := ExtractStubs(ExtractionRequest{
		SelectionDir: selectionDir,
		OutputDir:    outputDir,
	}, Client{HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("unexpected extraction error: %v", err)
	}

	if result.WorkCount != 1 || result.BundleCount != 1 || result.CurrentBundleCount != 1 {
		t.Fatalf("unexpected extraction manifest: %+v", result)
	}

	assertFileExists(t, filepath.Join(outputDir, "manifest.json"))
	assertFileExists(t, filepath.Join(outputDir, "work_index.json"))
	assertFileExists(t, filepath.Join(outputDir, "works", "CVDR100.json"))
	assertFileExists(t, filepath.Join(outputDir, "bundles", "CVDR100", "CVDR100_2", "publication.xml"))
	assertFileExists(t, filepath.Join(outputDir, "bundles", "CVDR100", "CVDR100_2", "record.json"))
	assertFileExists(t, filepath.Join(outputDir, "bundles", "CVDR100", "CVDR100_2", "draft.json"))

	var draft DraftStub
	readJSONFile(t, filepath.Join(outputDir, "bundles", "CVDR100", "CVDR100_2", "draft.json"), &draft)
	if draft.Selection.Role != "current" {
		t.Fatalf("expected current role, got %s", draft.Selection.Role)
	}
	if draft.SuggestedFixturePath != "core/fixtures/regulation/nl/gemeentelijke_verordening/amsterdam/2025-01-01.json" {
		t.Fatalf("unexpected suggested fixture path: %s", draft.SuggestedFixturePath)
	}
}

func TestExtractStubsIncludesHistoricalBundlesWhenEnabled(t *testing.T) {
	indexDir := t.TempDir()
	selectionDir := t.TempDir()
	outputDir := t.TempDir()
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.String() {
			case "https://example.test/xml/CVDR100_1.xml":
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewBufferString("<regeling id=\"CVDR100_1\"/>")),
					Header:     make(http.Header),
				}, nil
			case "https://example.test/xml/CVDR100_2.xml":
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewBufferString("<regeling id=\"CVDR100_2\"/>")),
					Header:     make(http.Header),
				}, nil
			default:
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Status:     "404 Not Found",
					Body:       io.NopCloser(bytes.NewBufferString("not found")),
					Header:     make(http.Header),
				}, nil
			}
		}),
	}

	mustWriteWorkFile(t, indexDir, WorkFile{
		Work: WorkTimeline{
			CVDRID:              "CVDR100",
			Creator:             "'s-Gravenhage",
			RepresentativeTitle: "Verordening toeristenbelasting",
		},
		Versions: []Record{
			{
				CVDRID:            "CVDR100",
				Identifier:        "CVDR100_1",
				Creator:           "'s-Gravenhage",
				Issued:            "2023-12-20",
				EffectiveFrom:     "2024-01-01",
				EffectiveTo:       "2025-01-01",
				ChangeCategory:    "amendment",
				PublicationXMLURL: "https://example.test/xml/CVDR100_1.xml",
			},
			{
				CVDRID:            "CVDR100",
				Identifier:        "CVDR100_2",
				Creator:           "'s-Gravenhage",
				Issued:            "2024-11-14",
				EffectiveFrom:     "2025-01-01",
				PublicationXMLURL: "https://example.test/xml/CVDR100_2.xml",
			},
		},
	})
	mustWriteSelectionManifest(t, selectionDir, SelectionManifest{
		SelectedAt:               "2026-04-17T00:00:00Z",
		IndexDir:                 indexDir,
		AsOfDate:                 "2026-04-17",
		WorkCount:                1,
		CurrentCandidateCount:    1,
		HistoricalCandidateCount: 1,
		Works: []WorkSelection{
			{
				CVDRID:              "CVDR100",
				Creator:             "'s-Gravenhage",
				RepresentativeTitle: "Verordening toeristenbelasting",
				CurrentCandidate: Candidate{
					Identifier:       "CVDR100_2",
					SelectionReasons: []string{"latest_active_version"},
				},
				HistoricalCandidates: []Candidate{
					{
						Identifier:       "CVDR100_1",
						SelectionReasons: []string{"earliest_version"},
					},
				},
			},
		},
	})

	result, err := ExtractStubs(ExtractionRequest{
		SelectionDir:      selectionDir,
		OutputDir:         outputDir,
		IncludeHistorical: true,
	}, Client{HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("unexpected extraction error: %v", err)
	}

	if result.BundleCount != 2 || result.HistoricalBundleCount != 1 {
		t.Fatalf("unexpected extraction counts: %+v", result)
	}

	assertFileExists(t, filepath.Join(outputDir, "bundles", "CVDR100", "CVDR100_1", "draft.json"))
	var draft DraftStub
	readJSONFile(t, filepath.Join(outputDir, "bundles", "CVDR100", "CVDR100_1", "draft.json"), &draft)
	if draft.Selection.Role != "historical" {
		t.Fatalf("expected historical role, got %s", draft.Selection.Role)
	}
	if draft.SuggestedFixturePath != "core/fixtures/regulation/nl/gemeentelijke_verordening/s_gravenhage/2024-01-01.json" {
		t.Fatalf("unexpected suggested fixture path: %s", draft.SuggestedFixturePath)
	}
	if len(draft.TODO) < 4 {
		t.Fatalf("expected amendment TODOs, got %+v", draft.TODO)
	}
}

func mustWriteSelectionManifest(t *testing.T, dir string, selection SelectionManifest) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "works"), 0o755); err != nil {
		t.Fatalf("mkdir selection works: %v", err)
	}
	manifest := SelectionExportManifest{
		SelectedAt:               selection.SelectedAt,
		IndexDir:                 selection.IndexDir,
		AsOfDate:                 selection.AsOfDate,
		MaxHistoricalPerWork:     selection.MaxHistoricalPerWork,
		WorkCount:                selection.WorkCount,
		CurrentCandidateCount:    selection.CurrentCandidateCount,
		HistoricalCandidateCount: selection.HistoricalCandidateCount,
	}
	if err := writeJSONFile(filepath.Join(dir, "manifest.json"), manifest); err != nil {
		t.Fatalf("write selection manifest: %v", err)
	}
	for _, work := range selection.Works {
		if err := writeJSONFile(filepath.Join(dir, "works", work.CVDRID+".json"), work); err != nil {
			t.Fatalf("write selection work: %v", err)
		}
	}
}
