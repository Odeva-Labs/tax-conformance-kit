package cvdr

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExportHarvestIndexWritesManifestAndWorkFiles(t *testing.T) {
	dir := t.TempDir()
	response := HarvestResponse{
		BaseQuery:             `title all "toeristenbelasting"`,
		StartDate:             "2026-01-01",
		EndDateExclusive:      "2026-01-10",
		WindowRecordLimit:     4000,
		PageSize:              100,
		IssuedMatchingRecords: 3,
		IssuedPartitionSum:    3,
		UniqueWorkCount:       2,
		UniqueVersionCount:    3,
		IssuedPartitions: []Partition{
			{Field: "issued", StartDate: "2026-01-01", EndDateExclusive: "2026-01-10", Query: "q", RecordCount: 3, PageCount: 1},
		},
		PartitioningComplete: true,
		Works: []WorkTimeline{
			{CVDRID: "CVDR100", VersionCount: 2, Creator: "Amsterdam"},
			{CVDRID: "CVDR200", VersionCount: 1, Creator: "Breda"},
		},
		Records: []Record{
			{CVDRID: "CVDR100", Identifier: "CVDR100_1", PreferredURL: "https://example/CVDR100/1"},
			{CVDRID: "CVDR100", Identifier: "CVDR100_2", PreferredURL: "https://example/CVDR100/2"},
			{CVDRID: "CVDR200", Identifier: "CVDR200_1", PreferredURL: "https://example/CVDR200/1"},
		},
	}

	if err := ExportHarvestIndex(dir, response); err != nil {
		t.Fatalf("unexpected export error: %v", err)
	}

	assertFileExists(t, filepath.Join(dir, "manifest.json"))
	assertFileExists(t, filepath.Join(dir, "work_index.json"))
	assertFileExists(t, filepath.Join(dir, "works", "CVDR100.json"))
	assertFileExists(t, filepath.Join(dir, "works", "CVDR200.json"))

	var manifest ExportManifest
	readJSONFile(t, filepath.Join(dir, "manifest.json"), &manifest)
	if manifest.UniqueWorkCount != 2 {
		t.Fatalf("expected manifest unique work count 2, got %d", manifest.UniqueWorkCount)
	}

	var workFile WorkFile
	readJSONFile(t, filepath.Join(dir, "works", "CVDR100.json"), &workFile)
	if workFile.Work.CVDRID != "CVDR100" || len(workFile.Versions) != 2 {
		t.Fatalf("unexpected work file: %+v", workFile)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
}

func readJSONFile(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
}
