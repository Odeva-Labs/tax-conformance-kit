package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/odeva-labs/tax-conformance-kit/engines/go/internal/engine"
	"github.com/odeva-labs/tax-conformance-kit/engines/go/internal/ingest/cvdr"
	"github.com/odeva-labs/tax-conformance-kit/engines/go/internal/model"
	"github.com/odeva-labs/tax-conformance-kit/engines/go/internal/runtimeapi"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "discover-cvdr":
		discoverCVDRCmd(os.Args[2:])
	case "harvest-cvdr":
		harvestCVDRCmd(os.Args[2:])
	case "select-cvdr-candidates":
		selectCVDRCandidatesCmd(os.Args[2:])
	case "import-cbs-municipality-codes":
		importCBSMunicipalityCodesCmd(os.Args[2:])
	case "backfill-municipality-codes":
		backfillMunicipalityCodesCmd(os.Args[2:])
	case "report-cvdr-coverage":
		reportCVDRCoverageCmd(os.Args[2:])
	case "extract-cvdr-stubs":
		extractCVDRStubsCmd(os.Args[2:])
	case "analyze-cvdr-stubs":
		analyzeCVDRStubsCmd(os.Args[2:])
	case "generate-draft-fixtures":
		generateDraftFixturesCmd(os.Args[2:])
	case "evaluate":
		evaluateCmd(os.Args[2:])
	case "evaluate-assessment":
		evaluateAssessmentCmd(os.Args[2:])
	case "runtime-validate":
		runtimeValidateCmd(os.Args[2:])
	case "runtime-evaluate":
		runtimeEvaluateCmd(os.Args[2:])
	case "runtime-evaluate-assessment":
		runtimeEvaluateAssessmentCmd(os.Args[2:])
	case "validate":
		validateCmd(os.Args[2:])
	case "export":
		exportCmd(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: taxctl <discover-cvdr|harvest-cvdr|select-cvdr-candidates|report-cvdr-coverage|import-cbs-municipality-codes|backfill-municipality-codes|extract-cvdr-stubs|analyze-cvdr-stubs|generate-draft-fixtures|evaluate|evaluate-assessment|runtime-validate|runtime-evaluate|runtime-evaluate-assessment|validate|export> [flags]")
}

func evaluateCmd(args []string) {
	fs := flag.NewFlagSet("evaluate", flag.ExitOnError)
	casePath := fs.String("case", "", "path to conformance case json")
	_ = fs.Parse(args)

	if *casePath == "" {
		fail("-case is required")
	}

	cc := mustReadCase(*casePath)
	rulePath := filepath.Join(filepath.Dir(*casePath), cc.RuleSetPath)
	rs := mustReadRuleSet(rulePath)
	registry := mustReadKindRegistry(findRepoFile("core/schemas/kind-registry.v1.json"))

	if err := engine.ValidateRuleSet(rs, registry); err != nil {
		fail(err.Error())
	}

	result, err := engine.Evaluate(cc.BookingInput, rs)
	if err != nil {
		fail(err.Error())
	}

	pass := result.TotalTax == cc.Expected.TotalTax && slices.Equal(result.MatchedRuleIDs, cc.Expected.MatchedRuleIDs)
	payload := map[string]any{
		"case_id":    cc.ID,
		"pass":       pass,
		"computed":   result,
		"expected":   cc.Expected,
		"rule_count": len(rs.Rules),
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		fail(err.Error())
	}

	if !pass {
		os.Exit(2)
	}
}

func validateCmd(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	rulesPath := fs.String("rules", "", "path to ruleset json")
	_ = fs.Parse(args)

	if *rulesPath == "" {
		fail("-rules is required")
	}

	rs := mustReadRuleSet(*rulesPath)
	registry := mustReadKindRegistry(findRepoFile("core/schemas/kind-registry.v1.json"))

	if err := engine.ValidateRuleSet(rs, registry); err != nil {
		fail(err.Error())
	}

	fmt.Printf("OK: %d rules validated\n", len(rs.Rules))
}

func runtimeValidateCmd(args []string) {
	fs := flag.NewFlagSet("runtime-validate", flag.ExitOnError)
	inputPath := fs.String("input", "-", "path to runtime request json ('-' for stdin)")
	registryPath := fs.String("registry", "", "path to kind registry json")
	_ = fs.Parse(args)

	request, err := readJSONInput[model.RuntimeValidateRequest](*inputPath)
	if err != nil {
		writeRuntimeResponse(model.RuntimeValidateResponse{
			APIVersion: model.RuntimeAPIVersion,
			OK:         false,
			Error:      &model.RuntimeError{Message: err.Error()},
		}, false)
		return
	}

	response := runtimeapi.Validate(request, readOptionalRegistry(*registryPath))
	writeRuntimeResponse(response, response.OK)
}

func runtimeEvaluateCmd(args []string) {
	fs := flag.NewFlagSet("runtime-evaluate", flag.ExitOnError)
	inputPath := fs.String("input", "-", "path to runtime request json ('-' for stdin)")
	registryPath := fs.String("registry", "", "path to kind registry json")
	_ = fs.Parse(args)

	request, err := readJSONInput[model.RuntimeEvaluateRequest](*inputPath)
	if err != nil {
		writeRuntimeResponse(model.RuntimeEvaluateResponse{
			APIVersion: model.RuntimeAPIVersion,
			OK:         false,
			Error:      &model.RuntimeError{Message: err.Error()},
		}, false)
		return
	}

	response := runtimeapi.Evaluate(request, readOptionalRegistry(*registryPath))
	writeRuntimeResponse(response, response.OK)
}

func runtimeEvaluateAssessmentCmd(args []string) {
	fs := flag.NewFlagSet("runtime-evaluate-assessment", flag.ExitOnError)
	inputPath := fs.String("input", "-", "path to runtime request json ('-' for stdin)")
	registryPath := fs.String("registry", "", "path to kind registry json")
	_ = fs.Parse(args)

	request, err := readJSONInput[model.RuntimeEvaluateAssessmentRequest](*inputPath)
	if err != nil {
		writeRuntimeResponse(model.RuntimeEvaluateAssessmentResponse{
			APIVersion: model.RuntimeAPIVersion,
			OK:         false,
			Error:      &model.RuntimeError{Message: err.Error()},
		}, false)
		return
	}

	response := runtimeapi.EvaluateAssessment(request, readOptionalRegistry(*registryPath))
	writeRuntimeResponse(response, response.OK)
}

func evaluateAssessmentCmd(args []string) {
	fs := flag.NewFlagSet("evaluate-assessment", flag.ExitOnError)
	casePath := fs.String("case", "", "path to assessment conformance case json")
	_ = fs.Parse(args)

	if *casePath == "" {
		fail("-case is required")
	}

	ac := mustReadAssessmentCase(*casePath)
	rulePath := filepath.Join(filepath.Dir(*casePath), ac.RuleSetPath)
	rs := mustReadRuleSet(rulePath)
	registry := mustReadKindRegistry(findRepoFile("core/schemas/kind-registry.v1.json"))

	if err := engine.ValidateRuleSet(rs, registry); err != nil {
		fail(err.Error())
	}

	result, err := engine.EvaluateAssessment(ac.AssessmentInput, rs)
	if err != nil {
		fail(err.Error())
	}

	pass := result.TotalBookingTax == ac.Expected.TotalBookingTax &&
		result.TotalAssessmentTax == ac.Expected.TotalAssessmentTax &&
		slices.EqualFunc(result.BookingResults, ac.Expected.BookingResults, func(a, b model.EvaluationResult) bool {
			return a.TotalTax == b.TotalTax && slices.Equal(a.MatchedRuleIDs, b.MatchedRuleIDs)
		})

	payload := map[string]any{
		"case_id":    ac.ID,
		"pass":       pass,
		"computed":   result,
		"expected":   ac.Expected,
		"rule_count": len(rs.Rules),
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		fail(err.Error())
	}

	if !pass {
		os.Exit(2)
	}
}

func exportCmd(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	rulesPath := fs.String("rules", "", "path to ruleset json")
	format := fs.String("format", "json", "json|csv")
	_ = fs.Parse(args)

	if *rulesPath == "" {
		fail("-rules is required")
	}

	rs := mustReadRuleSet(*rulesPath)
	switch *format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rs); err != nil {
			fail(err.Error())
		}
	case "csv":
		w := csv.NewWriter(os.Stdout)
		_ = w.Write([]string{"id", "municipality_code", "kind", "params", "valid_from", "valid_to", "confidence"})
		for _, r := range rs.Rules {
			validTo := ""
			if r.ValidTo != nil {
				validTo = *r.ValidTo
			}
			params, err := json.Marshal(r.Calculation.Params)
			if err != nil {
				fail(err.Error())
			}
			_ = w.Write([]string{
				r.ID,
				r.MunicipalityCode,
				r.Calculation.Kind,
				string(params),
				r.ValidFrom,
				validTo,
				r.Confidence,
			})
		}
		w.Flush()
		if err := w.Error(); err != nil {
			fail(err.Error())
		}
	default:
		fail("-format must be json or csv")
	}
}

func discoverCVDRCmd(args []string) {
	fs := flag.NewFlagSet("discover-cvdr", flag.ExitOnError)
	query := fs.String("query", `title all "toeristenbelasting"`, "SRU CQL query")
	startRecord := fs.Int("start-record", 1, "starting record position")
	maximumRecords := fs.Int("maximum-records", 10, "maximum records to request")
	baseURL := fs.String("base-url", cvdr.DefaultBaseURL, "SRU endpoint")
	_ = fs.Parse(args)

	client := cvdr.Client{BaseURL: *baseURL}
	result, err := client.Discover(cvdr.DiscoverRequest{
		Query:          *query,
		StartRecord:    *startRecord,
		MaximumRecords: *maximumRecords,
	})
	if err != nil {
		fail(err.Error())
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fail(err.Error())
	}
}

func reportCVDRCoverageCmd(args []string) {
	fs := flag.NewFlagSet("report-cvdr-coverage", flag.ExitOnError)
	selectionDir := fs.String("selection-dir", "", "path to selection export directory")
	fixtureRoot := fs.String("fixture-root", findRepoFile("core/fixtures/regulation/nl/gemeentelijke_verordening"), "path to regulation fixture root")
	_ = fs.Parse(args)

	if *selectionDir == "" {
		fail("-selection-dir is required")
	}

	report, err := cvdr.BuildCoverageReport(cvdr.CoverageReportRequest{
		SelectionDir: *selectionDir,
		FixtureRoot:  *fixtureRoot,
	})
	if err != nil {
		fail(err.Error())
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fail(err.Error())
	}
}

func importCBSMunicipalityCodesCmd(args []string) {
	fs := flag.NewFlagSet("import-cbs-municipality-codes", flag.ExitOnError)
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(findRepoFile("core/schemas/rules.schema.json"))))
	archivePath := fs.String("archive", "", "path to the official CBS 86247NED zip archive")
	outputPath := fs.String("output", cvdr.DefaultMunicipalityCatalogPath(repoRoot), "output path for the municipality catalog json")
	referenceYear := fs.String("reference-year", "2026", "reference year for the imported catalog")
	sourceURL := fs.String("source-url", cvdr.DefaultCBSMunicipalityDatasetURL, "official source URL for the imported catalog")
	_ = fs.Parse(args)

	if *archivePath == "" {
		fail("-archive is required")
	}
	catalog, err := cvdr.ImportMunicipalityCatalog(cvdr.ImportMunicipalityCatalogRequest{
		ArchivePath:   *archivePath,
		ReferenceYear: *referenceYear,
		SourceURL:     *sourceURL,
	})
	if err != nil {
		fail(err.Error())
	}
	if err := os.MkdirAll(filepath.Dir(*outputPath), 0o755); err != nil {
		fail(err.Error())
	}
	if err := cvdr.WriteJSONFile(*outputPath, catalog); err != nil {
		fail(err.Error())
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{
		"output_path":        filepath.ToSlash(*outputPath),
		"municipality_count": len(catalog.Municipalities),
		"reference_year":     catalog.Source.ReferenceYear,
	}); err != nil {
		fail(err.Error())
	}
}

func backfillMunicipalityCodesCmd(args []string) {
	fs := flag.NewFlagSet("backfill-municipality-codes", flag.ExitOnError)
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(findRepoFile("core/schemas/rules.schema.json"))))
	catalogPath := fs.String("catalog", cvdr.DefaultMunicipalityCatalogPath(repoRoot), "path to municipality catalog json")
	fixtureRoot := fs.String("fixture-root", findRepoFile("core/fixtures/regulation/nl/gemeentelijke_verordening"), "root directory of regulation fixtures")
	_ = fs.Parse(args)

	result, err := cvdr.BackfillMunicipalityCodes(cvdr.BackfillMunicipalityCodesRequest{
		CatalogPath: *catalogPath,
		FixtureRoot: *fixtureRoot,
	})
	if err != nil {
		fail(err.Error())
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fail(err.Error())
	}
}

func harvestCVDRCmd(args []string) {
	fs := flag.NewFlagSet("harvest-cvdr", flag.ExitOnError)
	query := fs.String("query", `title all "toeristenbelasting"`, "SRU CQL query")
	startDate := fs.String("start-date", "1900-01-01", "inclusive issued-date lower bound")
	endDateExclusive := fs.String("end-date-exclusive", "", "exclusive issued-date upper bound")
	maxWindowRecords := fs.Int("max-window-records", 4000, "maximum SRU hit count allowed per partition")
	pageSize := fs.Int("page-size", 100, "records to fetch per page within a partition")
	outputDir := fs.String("output-dir", "", "write a normalized harvest index to this directory")
	includeWorks := fs.Bool("include-works", false, "include grouped work timelines in the JSON output")
	includeRecords := fs.Bool("include-records", false, "include harvested records in the JSON output")
	useModifiedResidual := fs.Bool("use-modified-residual", true, "run a modified-date residual pass to catch records missed by issued-date harvesting")
	baseURL := fs.String("base-url", cvdr.DefaultBaseURL, "SRU endpoint")
	_ = fs.Parse(args)

	client := cvdr.Client{BaseURL: *baseURL}
	result, err := client.Harvest(cvdr.HarvestRequest{
		BaseQuery:           *query,
		StartDate:           *startDate,
		EndDateExclusive:    *endDateExclusive,
		MaxWindowRecords:    *maxWindowRecords,
		PageSize:            *pageSize,
		UseModifiedResidual: *useModifiedResidual,
	})
	if err != nil {
		fail(err.Error())
	}
	if *outputDir != "" {
		if err := cvdr.ExportHarvestIndex(*outputDir, result); err != nil {
			fail(err.Error())
		}
	}
	if !*includeWorks {
		result.Works = nil
	}
	if !*includeRecords {
		result.Records = nil
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fail(err.Error())
	}
}

func selectCVDRCandidatesCmd(args []string) {
	fs := flag.NewFlagSet("select-cvdr-candidates", flag.ExitOnError)
	indexDir := fs.String("index-dir", "", "path to a harvested CVDR index directory")
	outputDir := fs.String("output-dir", "", "write selection manifest/work files to this directory")
	asOfDate := fs.String("as-of-date", "", "select current candidates as of this date (YYYY-MM-DD)")
	maxHistoricalPerWork := fs.Int("max-historical-per-work", 3, "maximum historical candidates per work")
	includeWorks := fs.Bool("include-works", false, "include work selections in stdout JSON")
	_ = fs.Parse(args)

	if *indexDir == "" {
		fail("-index-dir is required")
	}

	result, err := cvdr.SelectCandidates(cvdr.SelectionRequest{
		IndexDir:             *indexDir,
		AsOfDate:             *asOfDate,
		MaxHistoricalPerWork: *maxHistoricalPerWork,
	})
	if err != nil {
		fail(err.Error())
	}
	if *outputDir != "" {
		if err := cvdr.ExportSelection(*outputDir, result); err != nil {
			fail(err.Error())
		}
	}
	if !*includeWorks {
		result.Works = nil
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fail(err.Error())
	}
}

func extractCVDRStubsCmd(args []string) {
	fs := flag.NewFlagSet("extract-cvdr-stubs", flag.ExitOnError)
	selectionDir := fs.String("selection-dir", "", "path to a selected CVDR candidate directory")
	indexDir := fs.String("index-dir", "", "path to a harvested CVDR index directory (defaults from selection manifest)")
	outputDir := fs.String("output-dir", "", "write extraction bundles to this directory")
	includeHistorical := fs.Bool("include-historical", false, "also export historical candidate bundles")
	baseURL := fs.String("base-url", cvdr.DefaultBaseURL, "SRU endpoint")
	_ = fs.Parse(args)

	if *selectionDir == "" {
		fail("-selection-dir is required")
	}
	if *outputDir == "" {
		fail("-output-dir is required")
	}

	client := cvdr.Client{BaseURL: *baseURL}
	result, err := cvdr.ExtractStubs(cvdr.ExtractionRequest{
		SelectionDir:      *selectionDir,
		IndexDir:          *indexDir,
		OutputDir:         *outputDir,
		IncludeHistorical: *includeHistorical,
	}, client)
	if err != nil {
		fail(err.Error())
	}

	result.Works = nil
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fail(err.Error())
	}
}

func analyzeCVDRStubsCmd(args []string) {
	fs := flag.NewFlagSet("analyze-cvdr-stubs", flag.ExitOnError)
	extractionDir := fs.String("extraction-dir", "", "path to an extracted CVDR bundle directory")
	_ = fs.Parse(args)

	if *extractionDir == "" {
		fail("-extraction-dir is required")
	}

	result, err := cvdr.AnalyzeExtractedBundles(cvdr.AnalyzeRequest{
		ExtractionDir: *extractionDir,
	})
	if err != nil {
		fail(err.Error())
	}

	result.Bundles = nil
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fail(err.Error())
	}
}

func generateDraftFixturesCmd(args []string) {
	fs := flag.NewFlagSet("generate-draft-fixtures", flag.ExitOnError)
	extractionDir := fs.String("extraction-dir", "", "path to an analyzed CVDR extraction directory")
	strictWarnings := fs.Bool("strict-warnings", true, "skip bundles that still have analyzer warnings")
	overwrite := fs.Bool("overwrite", false, "overwrite existing generated targets")
	includePaths := fs.Bool("include-paths", false, "include generated fixture paths in stdout JSON")
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(findRepoFile("core/schemas/rules.schema.json"))))
	municipalityCatalogPath := fs.String("municipality-catalog", cvdr.DefaultMunicipalityCatalogPath(repoRoot), "path to municipality code catalog json")
	_ = fs.Parse(args)

	if *extractionDir == "" {
		fail("-extraction-dir is required")
	}

	result, err := cvdr.GenerateDraftFixtures(cvdr.GenerateDraftFixturesRequest{
		ExtractionDir:           *extractionDir,
		RepoRoot:                repoRoot,
		StrictWarnings:          *strictWarnings,
		Overwrite:               *overwrite,
		MunicipalityCatalogPath: *municipalityCatalogPath,
	})
	if err != nil {
		fail(err.Error())
	}
	if !*includePaths {
		result.GeneratedPaths = nil
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fail(err.Error())
	}
}

func mustReadRuleSet(path string) model.RuleSet {
	b, err := os.ReadFile(path)
	if err != nil {
		fail(err.Error())
	}
	var rs model.RuleSet
	if err := json.Unmarshal(b, &rs); err != nil {
		fail(err.Error())
	}
	return rs
}

func mustReadCase(path string) model.ConformanceCase {
	b, err := os.ReadFile(path)
	if err != nil {
		fail(err.Error())
	}
	var cc model.ConformanceCase
	if err := json.Unmarshal(b, &cc); err != nil {
		fail(err.Error())
	}
	return cc
}

func mustReadAssessmentCase(path string) model.AssessmentCase {
	b, err := os.ReadFile(path)
	if err != nil {
		fail(err.Error())
	}
	var ac model.AssessmentCase
	if err := json.Unmarshal(b, &ac); err != nil {
		fail(err.Error())
	}
	return ac
}

func mustReadKindRegistry(path string) model.KindRegistry {
	b, err := os.ReadFile(path)
	if err != nil {
		fail(err.Error())
	}
	var registry model.KindRegistry
	if err := json.Unmarshal(b, &registry); err != nil {
		fail(err.Error())
	}
	return registry
}

func readJSONInput[T any](path string) (T, error) {
	var zero T

	var reader io.Reader
	if path == "" || path == "-" {
		reader = os.Stdin
	} else {
		file, err := os.Open(path)
		if err != nil {
			return zero, err
		}
		defer file.Close()
		reader = file
	}

	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()

	var value T
	if err := decoder.Decode(&value); err != nil {
		return zero, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return zero, fmt.Errorf("input must contain exactly one JSON document")
	}
	return value, nil
}

func writeRuntimeResponse(payload any, ok bool) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		fail(err.Error())
	}
	if !ok {
		os.Exit(1)
	}
}

func readOptionalRegistry(path string) model.KindRegistry {
	if path != "" {
		return mustReadKindRegistry(path)
	}
	if defaultPath, ok := tryFindRepoFile("core/schemas/kind-registry.v1.json"); ok {
		return mustReadKindRegistry(defaultPath)
	}
	return model.KindRegistry{}
}

func tryFindRepoFile(rel string) (string, bool) {
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}

	dir := wd
	for {
		candidate := filepath.Join(dir, rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func findRepoFile(rel string) string {
	candidate, ok := tryFindRepoFile(rel)
	if !ok {
		fail("could not locate " + rel)
	}
	return candidate
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}
