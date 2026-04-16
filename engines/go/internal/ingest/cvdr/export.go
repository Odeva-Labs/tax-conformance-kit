package cvdr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type ExportManifest struct {
	ExportedAt              string      `json:"exported_at"`
	BaseQuery               string      `json:"base_query"`
	StartDate               string      `json:"start_date"`
	EndDateExclusive        string      `json:"end_date_exclusive"`
	WindowRecordLimit       int         `json:"window_record_limit"`
	PageSize                int         `json:"page_size"`
	IssuedMatchingRecords   int         `json:"issued_matching_records"`
	ModifiedMatchingRecords int         `json:"modified_matching_records,omitempty"`
	IssuedPartitionSum      int         `json:"issued_partition_sum"`
	ResidualStrategy        string      `json:"residual_strategy,omitempty"`
	ResidualVersionCount    int         `json:"residual_version_count,omitempty"`
	UniqueWorkCount         int         `json:"unique_work_count"`
	UniqueVersionCount      int         `json:"unique_version_count"`
	PartitioningComplete    bool        `json:"partitioning_complete"`
	IssuedPartitions        []Partition `json:"issued_partitions"`
	ResidualPartitions      []Partition `json:"residual_partitions,omitempty"`
	WorkIndexPath           string      `json:"work_index_path"`
	WorksDirPath            string      `json:"works_dir_path"`
}

type WorkFile struct {
	Work     WorkTimeline `json:"work"`
	Versions []Record     `json:"versions"`
}

func ExportHarvestIndex(outputDir string, response HarvestResponse) error {
	if outputDir == "" {
		return fmt.Errorf("output directory is required")
	}
	if len(response.Records) == 0 {
		return fmt.Errorf("harvest response has no records to export")
	}

	if err := os.MkdirAll(filepath.Join(outputDir, "works"), 0o755); err != nil {
		return err
	}

	workFiles := buildWorkFiles(response)
	workIndex := make([]WorkTimeline, 0, len(workFiles))
	for _, workFile := range workFiles {
		workIndex = append(workIndex, workFile.Work)
		filename := filepath.Join(outputDir, "works", workFile.Work.CVDRID+".json")
		if err := writeJSONFile(filename, workFile); err != nil {
			return err
		}
	}
	sort.Slice(workIndex, func(i, j int) bool { return workIndex[i].CVDRID < workIndex[j].CVDRID })

	if err := writeJSONFile(filepath.Join(outputDir, "work_index.json"), workIndex); err != nil {
		return err
	}

	manifest := ExportManifest{
		ExportedAt:              time.Now().UTC().Format(time.RFC3339),
		BaseQuery:               response.BaseQuery,
		StartDate:               response.StartDate,
		EndDateExclusive:        response.EndDateExclusive,
		WindowRecordLimit:       response.WindowRecordLimit,
		PageSize:                response.PageSize,
		IssuedMatchingRecords:   response.IssuedMatchingRecords,
		ModifiedMatchingRecords: response.ModifiedMatchingRecords,
		IssuedPartitionSum:      response.IssuedPartitionSum,
		ResidualStrategy:        response.ResidualStrategy,
		ResidualVersionCount:    response.ResidualVersionCount,
		UniqueWorkCount:         response.UniqueWorkCount,
		UniqueVersionCount:      response.UniqueVersionCount,
		PartitioningComplete:    response.PartitioningComplete,
		IssuedPartitions:        response.IssuedPartitions,
		ResidualPartitions:      response.ResidualPartitions,
		WorkIndexPath:           "work_index.json",
		WorksDirPath:            "works",
	}
	return writeJSONFile(filepath.Join(outputDir, "manifest.json"), manifest)
}

func buildWorkFiles(response HarvestResponse) []WorkFile {
	recordsByWork := map[string][]Record{}
	workByID := map[string]WorkTimeline{}
	for _, work := range response.Works {
		workByID[work.CVDRID] = work
	}
	for _, record := range response.Records {
		key := record.CVDRID
		if key == "" {
			key = record.Identifier
		}
		recordsByWork[key] = append(recordsByWork[key], record)
	}

	files := make([]WorkFile, 0, len(recordsByWork))
	for cvdrID, versions := range recordsByWork {
		sort.Slice(versions, func(i, j int) bool {
			return compareTimelineDate(versions[i], versions[j])
		})
		work := workByID[cvdrID]
		if work.CVDRID == "" {
			work = buildWorkTimelines(versions)[0]
		}
		files = append(files, WorkFile{
			Work:     work,
			Versions: versions,
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Work.CVDRID < files[j].Work.CVDRID })
	return files
}

func writeJSONFile(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func WriteJSONFile(path string, value any) error {
	return writeJSONFile(path, value)
}
