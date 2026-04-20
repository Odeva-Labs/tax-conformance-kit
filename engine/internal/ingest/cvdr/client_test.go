package cvdr

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDiscoverParsesSRUResponse(t *testing.T) {
	client := Client{
		BaseURL: "https://example.invalid/sru/Search",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/xml"}},
					Body: io.NopCloser(strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<searchRetrieveResponse xmlns="http://www.loc.gov/zing/srw/">
  <numberOfRecords>342</numberOfRecords>
  <nextRecordPosition>3</nextRecordPosition>
  <records>
    <record>
      <recordData>
        <gzd>
          <originalData>
            <meta>
              <identifier>https://lokaleregelgeving.overheid.nl/CVDR750921/1</identifier>
              <title>Verordening op de heffing en invordering van toeristenbelasting 2026</title>
              <creator>Amsterdam</creator>
              <isRatifiedBy>gemeenteraad</isRatifiedBy>
              <modified>2025-12-19</modified>
              <issued>2025-12-19</issued>
              <type>regeling</type>
              <subject>Toeristenbelasting</subject>
            </meta>
          </originalData>
        </gzd>
      </recordData>
      <recordPosition>1</recordPosition>
    </record>
    <record>
      <recordData>
        <gzd>
          <originalData>
            <meta>
              <identifier>https://lokaleregelgeving.overheid.nl/CVDR741837/1</identifier>
              <title>Verordening toeristenbelasting 2023</title>
              <creator>Breda</creator>
            </meta>
          </originalData>
        </gzd>
      </recordData>
      <recordPosition>2</recordPosition>
    </record>
  </records>
</searchRetrieveResponse>`)),
				}, nil
			}),
		},
	}

	got, err := client.Discover(DiscoverRequest{
		Query:          `title all "toeristenbelasting"`,
		StartRecord:    1,
		MaximumRecords: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.NumberOfRecords != 342 {
		t.Fatalf("expected 342 records, got %d", got.NumberOfRecords)
	}
	if got.NextRecordPosition != 3 {
		t.Fatalf("expected next position 3, got %d", got.NextRecordPosition)
	}
	if len(got.Records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(got.Records))
	}
	if got.Records[0].CVDRID != "CVDR750921" {
		t.Fatalf("unexpected cvdr id: %q", got.Records[0].CVDRID)
	}
	if got.Records[0].RatifiedBy != "gemeenteraad" {
		t.Fatalf("unexpected ratified by: %q", got.Records[0].RatifiedBy)
	}
	if got.Records[0].PreferredWorkURL != "" {
		t.Fatalf("expected empty preferred work url in sample, got %q", got.Records[0].PreferredWorkURL)
	}
	if got.Records[0].Title == "" || got.Records[0].Creator != "Amsterdam" || got.Records[0].EffectiveFrom != "" {
		t.Fatalf("unexpected first record: %+v", got.Records[0])
	}
}

func TestHarvestSplitsIssuedDateRangeAndFetchesPartitions(t *testing.T) {
	client := Client{
		BaseURL: "https://example.invalid/sru/Search",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				query := req.URL.Query().Get("query")
				startRecord := req.URL.Query().Get("startRecord")

				switch {
				case query == `(title all "toeristenbelasting") and (issued>="2026-01-01" and issued<"2026-01-10")`:
					return xmlResponse(3, 0, nil), nil
				case query == `(title all "toeristenbelasting") and (modified>="2026-01-01" and modified<"2026-01-10")`:
					return xmlResponse(4, 0, nil), nil
				case query == `(title all "toeristenbelasting") and (modified>="2026-01-01" and modified<"2026-01-05")`:
					return xmlResponse(3, 0, nil), nil
				case strings.Contains(query, `issued>="2026-01-01"`) && strings.Contains(query, `issued<"2026-01-05"`):
					return xmlResponse(2, 0, []string{
						recordXML("CVDR100_1", "Amsterdam", "Nieuwe regeling", "2026-01-01", "2026-01-01", "https://lokaleregelgeving.overheid.nl/CVDR100/1", "https://lokaleregelgeving.overheid.nl/CVDR100"),
						recordXML("CVDR100_2", "Amsterdam", "wijziging", "2026-01-03", "2026-01-03", "https://lokaleregelgeving.overheid.nl/CVDR100/2", "https://lokaleregelgeving.overheid.nl/CVDR100"),
					}), nil
				case strings.Contains(query, `issued>="2026-01-05"`) && strings.Contains(query, `issued<"2026-01-10"`):
					return xmlResponse(1, 0, []string{
						recordXML("CVDR200_1", "Breda", "Nieuwe regeling", "2026-01-06", "2026-01-06", "https://lokaleregelgeving.overheid.nl/CVDR200/1", "https://lokaleregelgeving.overheid.nl/CVDR200"),
					}), nil
				case strings.Contains(query, `modified>="2026-01-01"`) && strings.Contains(query, `modified<"2026-01-03"`):
					return xmlResponse(1, 0, []string{
						recordXML("CVDR100_1", "Amsterdam", "Nieuwe regeling", "2026-01-01", "2026-01-01", "https://lokaleregelgeving.overheid.nl/CVDR100/1", "https://lokaleregelgeving.overheid.nl/CVDR100"),
					}), nil
				case strings.Contains(query, `modified>="2026-01-03"`) && strings.Contains(query, `modified<"2026-01-05"`):
					return xmlResponse(2, 0, []string{
						recordXML("CVDR100_1", "Amsterdam", "Nieuwe regeling", "2026-01-01", "2026-01-01", "https://lokaleregelgeving.overheid.nl/CVDR100/1", "https://lokaleregelgeving.overheid.nl/CVDR100"),
						recordXML("CVDR100_2", "Amsterdam", "wijziging", "2026-01-03", "2026-01-03", "https://lokaleregelgeving.overheid.nl/CVDR100/2", "https://lokaleregelgeving.overheid.nl/CVDR100"),
						recordXML("CVDR300_1", "Leiden", "Nieuwe regeling", "", "2026-01-04", "https://lokaleregelgeving.overheid.nl/CVDR300/1", "https://lokaleregelgeving.overheid.nl/CVDR300"),
					}), nil
				case strings.Contains(query, `modified>="2026-01-05"`) && strings.Contains(query, `modified<"2026-01-10"`):
					return xmlResponse(1, 0, []string{
						recordXML("CVDR200_1", "Breda", "Nieuwe regeling", "2026-01-06", "2026-01-06", "https://lokaleregelgeving.overheid.nl/CVDR200/1", "https://lokaleregelgeving.overheid.nl/CVDR200"),
					}), nil
				default:
					t.Fatalf("unexpected query: %s startRecord=%s", query, startRecord)
					return nil, nil
				}
			}),
		},
	}

	got, err := client.Harvest(HarvestRequest{
		BaseQuery:           `title all "toeristenbelasting"`,
		StartDate:           "2026-01-01",
		EndDateExclusive:    "2026-01-10",
		MaxWindowRecords:    2,
		PageSize:            100,
		UseModifiedResidual: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.IssuedPartitions) != 2 {
		t.Fatalf("expected 2 issued partitions, got %d", len(got.IssuedPartitions))
	}
	if got.IssuedPartitionSum != 3 {
		t.Fatalf("expected issued partition sum 3, got %d", got.IssuedPartitionSum)
	}
	if got.ModifiedMatchingRecords != 4 {
		t.Fatalf("expected modified matching records 4, got %d", got.ModifiedMatchingRecords)
	}
	if got.ResidualVersionCount != 1 {
		t.Fatalf("expected residual version count 1, got %d", got.ResidualVersionCount)
	}
	if got.UniqueWorkCount != 3 {
		t.Fatalf("expected 3 unique works, got %d", got.UniqueWorkCount)
	}
	if got.UniqueVersionCount != 4 {
		t.Fatalf("expected 4 unique versions, got %d", got.UniqueVersionCount)
	}
	if len(got.Works) != 3 {
		t.Fatalf("expected 3 work timelines, got %d", len(got.Works))
	}
	if got.ResidualStrategy != "modified" {
		t.Fatalf("expected residual strategy modified, got %q", got.ResidualStrategy)
	}
	if !got.PartitioningComplete {
		t.Fatalf("expected partitioning complete to be true")
	}
}

func TestBuildDateRangeQuery(t *testing.T) {
	start := mustDate(t, "2026-01-01")
	end := mustDate(t, "2027-01-01")
	got := buildDateRangeQuery(`title all "toeristenbelasting"`, "issued", start, end)
	want := `(title all "toeristenbelasting") and (issued>="2026-01-01" and issued<"2027-01-01")`
	if got != want {
		t.Fatalf("unexpected query: %s", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func xmlResponse(numberOfRecords, nextRecordPosition int, records []string) *http.Response {
	xml := `<?xml version="1.0" encoding="UTF-8"?><searchRetrieveResponse xmlns="http://www.loc.gov/zing/srw/">` +
		`<numberOfRecords>` + strconv.Itoa(numberOfRecords) + `</numberOfRecords>` +
		`<nextRecordPosition>` + strconv.Itoa(nextRecordPosition) + `</nextRecordPosition><records>`
	for i, record := range records {
		xml += "<record><recordData>" + record + "</recordData><recordPosition>" + strconv.Itoa(i+1) + "</recordPosition></record>"
	}
	xml += "</records></searchRetrieveResponse>"
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/xml"}},
		Body:       io.NopCloser(strings.NewReader(xml)),
	}
}

func recordXML(identifier, creator, changeNature, issued, modified, preferredURL, preferredWorkURL string) string {
	return `<gzd><originalData><meta><identifier>` + identifier + `</identifier><title>Verordening toeristenbelasting</title><creator>` + creator + `</creator><issued>` + issued + `</issued><modified>` + modified + `</modified><betreft>` + changeNature + `</betreft></meta></originalData><enrichedData><preferred_url>` + preferredURL + `</preferred_url><preferred_work_url>` + preferredWorkURL + `</preferred_work_url></enrichedData></gzd>`
}

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatalf("unexpected date parse error: %v", err)
	}
	return d
}
