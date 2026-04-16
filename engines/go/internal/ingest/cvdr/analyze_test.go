package cvdr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ramones/tax-conformance-kit/engines/go/internal/model"
)

func TestAnalyzeExtractedBundlesInfersPercentageRuleAndAssessmentPolicy(t *testing.T) {
	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "bundles", "CVDR750921", "CVDR750921_1")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}

	draft := DraftStub{
		Source: DraftSource{
			CVDRID:        "CVDR750921",
			Identifier:    "CVDR750921_1",
			EffectiveFrom: "2026-01-01",
			Issued:        "2025-12-15",
		},
		Jurisdiction: DraftJurisdiction{
			MunicipalityName: "Amsterdam",
		},
		SuggestedFixturePath: "core/fixtures/regulation/nl/gemeentelijke_verordening/amsterdam/2026-01-01.json",
	}
	if err := writeJSONFile(filepath.Join(bundleDir, "draft.json"), draft); err != nil {
		t.Fatalf("write draft: %v", err)
	}

	xml := `<?xml version="1.0" encoding="utf-8"?><cvdr><body><regeling><regeling-tekst>` +
		`<artikel><kop><nr>1</nr><titel>Belastbaar feit</titel></kop><al>Onder de naam toeristenbelasting wordt een directe belasting geheven door personen die niet als ingezetene met een adres in de gemeente in de basisregistratie personen zijn ingeschreven.</al></artikel>` +
		`<artikel><kop><nr>2</nr><titel>Vrijstellingen</titel></kop><al>De belasting wordt niet geheven voor het verblijf:</al><lijst><li nr="1."><al>van degene die verblijft in een instelling als bedoeld in artikel 4 van de Wet toetreding zorgaanbieders.</al></li><li nr="2."><al>van een vreemdeling die verblijf houdt onder verantwoordelijkheid van het Centraal Orgaan opvang Asielzoekers.</al></li></lijst></artikel>` +
		`<artikel><kop><nr>3</nr><titel>Maatstaf van heffing</titel></kop><al>De belasting wordt berekend over de vergoeding voor het verblijf die in rekening wordt gebracht, de toeristenbelasting daaronder niet begrepen.</al></artikel>` +
		`<artikel><kop><nr>4</nr><titel>Tarief</titel></kop><al>Het tarief bedraagt 7 procent van de heffingsmaatstaf.</al></artikel>` +
		`<artikel><kop><nr>5</nr><titel>Belastingtijdvak</titel></kop><al>Het belastingtijdvak is gelijk aan een kalenderkwartaal.</al></artikel>` +
		`<artikel><kop><nr>6</nr><titel>Aanslaggrens</titel></kop><al>Belastingaanslagen van minder dan € 25,00 worden niet opgelegd.</al></artikel>` +
		`</regeling-tekst></regeling></body></cvdr>`
	if err := os.WriteFile(filepath.Join(bundleDir, "publication.xml"), []byte(xml), 0o644); err != nil {
		t.Fatalf("write xml: %v", err)
	}

	manifest, err := AnalyzeExtractedBundles(AnalyzeRequest{ExtractionDir: dir})
	if err != nil {
		t.Fatalf("unexpected analyze error: %v", err)
	}
	if manifest.AnalyzedBundleCount != 1 || manifest.TotalCandidateRuleCount != 1 || manifest.AssessmentPolicyBundleCount != 1 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}

	var analysis BundleAnalysis
	readJSONFile(t, filepath.Join(bundleDir, "analysis.json"), &analysis)
	if len(analysis.CandidateRules) != 1 {
		t.Fatalf("expected 1 candidate rule, got %+v", analysis.CandidateRules)
	}
	if analysis.CandidateRules[0].Calculation.Kind != "generic.percentage_of_base" {
		t.Fatalf("unexpected calculation kind: %+v", analysis.CandidateRules[0].Calculation)
	}
	if analysis.CandidateRules[0].Calculation.Params["base"] != "accommodation_fee_exclusive_of_tax" {
		t.Fatalf("unexpected base params: %+v", analysis.CandidateRules[0].Calculation.Params)
	}
	if analysis.AssessmentPolicyCandidate == nil || analysis.AssessmentPolicyCandidate.Period != "calendar_quarter" {
		t.Fatalf("unexpected assessment policy: %+v", analysis.AssessmentPolicyCandidate)
	}
	if analysis.AssessmentPolicyCandidate.MinimumAssessmentAmount == nil || analysis.AssessmentPolicyCandidate.MinimumAssessmentAmount.Amount != 25 {
		t.Fatalf("unexpected minimum assessment amount: %+v", analysis.AssessmentPolicyCandidate)
	}
	if len(analysis.GlobalExemptions) != 3 {
		t.Fatalf("expected 3 global exemptions, got %+v", analysis.GlobalExemptions)
	}
}

func TestAnalyzeExtractedBundlesInfersPerNightRulesAndAgeExemption(t *testing.T) {
	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "bundles", "CVDR381749", "CVDR381749_8")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}

	draft := DraftStub{
		Source: DraftSource{
			CVDRID:        "CVDR381749",
			Identifier:    "CVDR381749_8",
			EffectiveFrom: "2025-01-01",
		},
		Jurisdiction: DraftJurisdiction{
			MunicipalityName: "'s-Gravenhage",
		},
		SuggestedFixturePath: "core/fixtures/regulation/nl/gemeentelijke_verordening/s_gravenhage/2025-01-01.json",
	}
	if err := writeJSONFile(filepath.Join(bundleDir, "draft.json"), draft); err != nil {
		t.Fatalf("write draft: %v", err)
	}

	xml := `<?xml version="1.0" encoding="utf-8"?><cvdr><body><regeling><regeling-tekst>` +
		`<artikel><kop><nr>2</nr><titel>Belastbaar feit</titel></kop><al>Onder de naam toeristenbelasting wordt een directe belasting geheven door personen die niet als ingezetene met een adres in de gemeente zijn ingeschreven.</al></artikel>` +
		`<artikel><kop><nr>4</nr><titel>Vrijstellingen</titel></kop><al>De belasting wordt niet geheven ter zake van het houden van verblijf met overnachten:</al><lijst><li nr="a."><al>van degene die verblijft in een instelling als bedoeld in artikel 4 van de Wet toetreding zorgaanbieders;</al></li><li nr="b."><al>van een vreemdeling die verblijf houdt onder verantwoordelijkheid van het Centraal Orgaan opvang Asielzoekers;</al></li><li nr="c."><al>door degene die op de dag waarop de eerste overnachting plaatsvindt, nog niet de leeftijd van dertien jaren heeft bereikt.</al></li></lijst></artikel>` +
		`<artikel><kop><nr>6</nr><titel>Belastingtarief</titel></kop><lid><lidnr>1.</lidnr><al>Het tarief bedraagt per persoon per overnachting € 6,20.</al></lid><lid><lidnr>2.</lidnr><al>In afwijking van het eerste lid bedraagt het tarief, indien overnacht wordt:</al><lijst><li nr="a."><al>op een camping, per persoon per overnachting € 2,85;</al></li><li nr="b."><al>in een haven per persoon per overnachting € 2,85.</al></li></lijst></lid></artikel>` +
		`<artikel><kop><nr>7</nr><titel>Belastingtijdvak</titel></kop><lid><lidnr>1.</lidnr><al>Het belastingtijdvak is gelijk aan het kalenderkwartaal.</al></lid><lid><lidnr>2.</lidnr><al>In afwijking van het eerste lid is het belastingtijdvak gelijk aan het kalenderjaar indien er sprake is van vakantieverhuur.</al></lid></artikel>` +
		`</regeling-tekst></regeling></body></cvdr>`
	if err := os.WriteFile(filepath.Join(bundleDir, "publication.xml"), []byte(xml), 0o644); err != nil {
		t.Fatalf("write xml: %v", err)
	}

	_, err := AnalyzeExtractedBundles(AnalyzeRequest{ExtractionDir: dir})
	if err != nil {
		t.Fatalf("unexpected analyze error: %v", err)
	}

	var analysis BundleAnalysis
	readJSONFile(t, filepath.Join(bundleDir, "analysis.json"), &analysis)
	if len(analysis.CandidateRules) != 3 {
		t.Fatalf("expected 3 candidate rules, got %+v", analysis.CandidateRules)
	}
	if !containsPredicate(analysis.GlobalExemptions, "guest.age_below") {
		t.Fatalf("expected guest.age_below exemption, got %+v", analysis.GlobalExemptions)
	}
	if analysis.AssessmentPolicyCandidate != nil {
		t.Fatalf("expected no simple assessment policy candidate for conditional period, got %+v", analysis.AssessmentPolicyCandidate)
	}
	if !containsWarning(analysis.Warnings, "multiple periods") {
		t.Fatalf("expected conditional-period warning, got %+v", analysis.Warnings)
	}
}

func TestAnalyzeExtractedBundlesHandlesEuroBeforeUnitAndBelastingjaar(t *testing.T) {
	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "bundles", "CVDR754024", "CVDR754024_1")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}

	draft := DraftStub{
		Source: DraftSource{
			CVDRID:        "CVDR754024",
			Identifier:    "CVDR754024_1",
			EffectiveFrom: "2026-01-01",
		},
		Jurisdiction: DraftJurisdiction{
			MunicipalityName: "Stede Broec",
		},
		SuggestedFixturePath: "core/fixtures/regulation/nl/gemeentelijke_verordening/stede_broec/2026-01-01.json",
	}
	if err := writeJSONFile(filepath.Join(bundleDir, "draft.json"), draft); err != nil {
		t.Fatalf("write draft: %v", err)
	}

	xml := `<?xml version="1.0" encoding="utf-8"?><cvdr><body><regeling><regeling-tekst>` +
		`<artikel><kop><nr>2</nr><titel>Belastbaar feit</titel></kop><al>Onder de naam toeristenbelasting wordt een directe belasting geheven door personen die niet als ingezetene met een adres in de gemeente in de basisregistratie personen zijn ingeschreven.</al></artikel>` +
		`<artikel><kop><nr>4</nr><titel>Vrijstellingen</titel></kop><al>De belasting wordt niet geheven ter zake van het verblijf:</al><lijst><li nr="1."><al>van degene die verblijft in een toegelaten instelling als bedoeld in artikel 5 van de Wet Toelating Zorginstellingen;</al></li><li nr="2."><al>van degene die verblijf houdt in een gemeubileerde woning voor welk verblijf hij forensenbelasting verschuldigd is.</al></li></lijst></artikel>` +
		`<artikel><kop><nr>5</nr><titel>Maatstaf van heffing</titel></kop><al>De belasting wordt geheven naar het aantal overnachtingen in het belastingjaar.</al></artikel>` +
		`<artikel><kop><nr>6</nr><titel>Belastingtarief</titel></kop><al>Het tarief bedraagt € 2,00 per persoon, per overnachting.</al></artikel>` +
		`<artikel><kop><nr>7</nr><titel>Belastingjaar</titel></kop><al>Het belastingjaar is gelijk aan het kalenderjaar.</al></artikel>` +
		`</regeling-tekst></regeling></body></cvdr>`
	if err := os.WriteFile(filepath.Join(bundleDir, "publication.xml"), []byte(xml), 0o644); err != nil {
		t.Fatalf("write xml: %v", err)
	}

	_, err := AnalyzeExtractedBundles(AnalyzeRequest{ExtractionDir: dir})
	if err != nil {
		t.Fatalf("unexpected analyze error: %v", err)
	}

	var analysis BundleAnalysis
	readJSONFile(t, filepath.Join(bundleDir, "analysis.json"), &analysis)
	if len(analysis.CandidateRules) != 1 {
		t.Fatalf("expected 1 candidate rule, got %+v", analysis.CandidateRules)
	}
	if analysis.CandidateRules[0].Calculation.Kind != "generic.per_person_per_night" {
		t.Fatalf("unexpected calculation kind: %+v", analysis.CandidateRules[0].Calculation)
	}
	if analysis.AssessmentPolicyCandidate == nil || analysis.AssessmentPolicyCandidate.Period != "calendar_year" {
		t.Fatalf("expected calendar-year assessment policy, got %+v", analysis.AssessmentPolicyCandidate)
	}
	if !containsPredicate(analysis.GlobalExemptions, "cross_tax.already_subject_to") {
		t.Fatalf("expected cross-tax exemption, got %+v", analysis.GlobalExemptions)
	}
}

func TestAnalyzeExtractedBundlesDoesNotTreatStandplaatsAnnualAmountsAsNightlyRates(t *testing.T) {
	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "bundles", "CVDR755247", "CVDR755247_1")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}

	draft := DraftStub{
		Source: DraftSource{
			CVDRID:        "CVDR755247",
			Identifier:    "CVDR755247_1",
			EffectiveFrom: "2026-01-01",
		},
		Jurisdiction: DraftJurisdiction{
			MunicipalityName: "Veldhoven",
		},
		SuggestedFixturePath: "core/fixtures/regulation/nl/gemeentelijke_verordening/veldhoven/2026-01-01.json",
	}
	if err := writeJSONFile(filepath.Join(bundleDir, "draft.json"), draft); err != nil {
		t.Fatalf("write draft: %v", err)
	}

	xml := `<?xml version="1.0" encoding="utf-8"?><cvdr><body><regeling><regeling-tekst>` +
		`<artikel><kop><nr>1.</nr><titel>Belastbaar feit</titel></kop><al>Onder de naam toeristenbelasting wordt een directe belasting geheven door personen die niet als ingezetene met een adres in de gemeente zijn ingeschreven.</al></artikel>` +
		`<artikel><kop><nr>4.</nr><titel>Vrijstellingen</titel></kop><al>De belasting wordt niet geheven ter zake van verblijf:</al><lijst><li nr="2."><al>van een vreemdeling die verblijf houdt onder verantwoordelijkheid van het Centraal Orgaan opvang Asielzoekers.</al></li></lijst></artikel>` +
		`<artikel><kop><nr>6.</nr><titel>Maatstaf van heffing</titel></kop><lijst><li nr="1."><al>De belasting wordt geheven naar het aantal overnachtingen in het belastingjaar. Het aantal overnachtingen wordt gesteld op het aantal overnachtende personen vermenigvuldigd met het aantal nachten.</al></li><li nr="2."><al>De belasting voor het houden van verblijf op een kampeerterrein kan naar vaste bedragen per standplaats worden geheven.</al></li></lijst></artikel>` +
		`<artikel><kop><nr>7.</nr><titel>Belastingtarief</titel></kop><lijst><li nr="1."><al>Het tarief bedraagt per persoon per overnachting:</al></li><li nr="a."><al>in hotels € 2,70</al></li><li nr="b."><al>in groepsaccommodaties € 1,60</al></li><li nr="c."><al>op kampeerterreinen € 1,60</al></li><li nr="2."><al>Het tarief bedraagt voor vaste standplaatsen;</al></li><li nr="a."><al>per maand € 40,00</al></li><li nr="b."><al>per seizoen € 200,00</al></li><li nr="c."><al>per jaar € 300,00</al></li></lijst></artikel>` +
		`<artikel><kop><nr>8.</nr><titel>Belastingjaar</titel></kop><al>Het belastingjaar is gelijk aan het kalenderjaar.</al></artikel>` +
		`</regeling-tekst></regeling></body></cvdr>`
	if err := os.WriteFile(filepath.Join(bundleDir, "publication.xml"), []byte(xml), 0o644); err != nil {
		t.Fatalf("write xml: %v", err)
	}

	_, err := AnalyzeExtractedBundles(AnalyzeRequest{ExtractionDir: dir})
	if err != nil {
		t.Fatalf("unexpected analyze error: %v", err)
	}

	var analysis BundleAnalysis
	readJSONFile(t, filepath.Join(bundleDir, "analysis.json"), &analysis)
	if len(analysis.CandidateRules) != 3 {
		t.Fatalf("expected only the three nightly accommodation rules, got %+v", analysis.CandidateRules)
	}
	for _, candidate := range analysis.CandidateRules {
		if candidate.Calculation.Params["amount"] == 40.0 || candidate.Calculation.Params["amount"] == 200.0 || candidate.Calculation.Params["amount"] == 300.0 {
			t.Fatalf("unexpected standplaats annual amount promoted as nightly rate: %+v", candidate)
		}
	}
}

func containsPredicate(predicates []model.Predicate, kind string) bool {
	for _, predicate := range predicates {
		if predicate.Kind == kind {
			return true
		}
	}
	return false
}

func containsWarning(warnings []string, fragment string) bool {
	for _, warning := range warnings {
		if strings.Contains(strings.ToLower(warning), strings.ToLower(fragment)) {
			return true
		}
	}
	return false
}
