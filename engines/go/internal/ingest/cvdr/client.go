package cvdr

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://zoekdienst.overheid.nl/sru/Search"

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

type DiscoverRequest struct {
	Query          string
	StartRecord    int
	MaximumRecords int
}

type DiscoverResponse struct {
	NumberOfRecords    int      `json:"number_of_records"`
	NextRecordPosition int      `json:"next_record_position,omitempty"`
	Records            []Record `json:"records"`
	RawURL             string   `json:"raw_url"`
}

type HarvestRequest struct {
	BaseQuery           string
	StartDate           string
	EndDateExclusive    string
	MaxWindowRecords    int
	PageSize            int
	UseModifiedResidual bool
}

type HarvestResponse struct {
	BaseQuery               string         `json:"base_query"`
	StartDate               string         `json:"start_date"`
	EndDateExclusive        string         `json:"end_date_exclusive"`
	WindowRecordLimit       int            `json:"window_record_limit"`
	PageSize                int            `json:"page_size"`
	IssuedMatchingRecords   int            `json:"issued_matching_records"`
	ModifiedMatchingRecords int            `json:"modified_matching_records,omitempty"`
	IssuedPartitionSum      int            `json:"issued_partition_sum"`
	ResidualStrategy        string         `json:"residual_strategy,omitempty"`
	ResidualVersionCount    int            `json:"residual_version_count,omitempty"`
	UniqueWorkCount         int            `json:"unique_work_count"`
	UniqueVersionCount      int            `json:"unique_version_count"`
	IssuedPartitions        []Partition    `json:"issued_partitions"`
	ResidualPartitions      []Partition    `json:"residual_partitions,omitempty"`
	Works                   []WorkTimeline `json:"works,omitempty"`
	Records                 []Record       `json:"records,omitempty"`
	PartitioningComplete    bool           `json:"partitioning_complete"`
}

type Partition struct {
	Field            string `json:"field,omitempty"`
	StartDate        string `json:"start_date"`
	EndDateExclusive string `json:"end_date_exclusive"`
	Query            string `json:"query"`
	RecordCount      int    `json:"record_count"`
	PageCount        int    `json:"page_count,omitempty"`
}

type WorkTimeline struct {
	CVDRID                string   `json:"cvdr_id"`
	Creator               string   `json:"creator,omitempty"`
	RepresentativeTitle   string   `json:"representative_title,omitempty"`
	VersionCount          int      `json:"version_count"`
	EarliestIssued        string   `json:"earliest_issued,omitempty"`
	LatestIssued          string   `json:"latest_issued,omitempty"`
	EarliestEffectiveFrom string   `json:"earliest_effective_from,omitempty"`
	LatestEffectiveTo     string   `json:"latest_effective_to,omitempty"`
	ChangeCategories      []string `json:"change_categories,omitempty"`
	LatestChangeCategory  string   `json:"latest_change_category,omitempty"`
	LatestVersionURL      string   `json:"latest_version_url,omitempty"`
}

type Record struct {
	RecordPosition    int      `json:"record_position,omitempty"`
	Identifier        string   `json:"identifier,omitempty"`
	CVDRID            string   `json:"cvdr_id,omitempty"`
	Title             string   `json:"title,omitempty"`
	Alternative       string   `json:"alternative,omitempty"`
	Creator           string   `json:"creator,omitempty"`
	RatifiedBy        string   `json:"ratified_by,omitempty"`
	Type              string   `json:"type,omitempty"`
	Modified          string   `json:"modified,omitempty"`
	Issued            string   `json:"issued,omitempty"`
	EffectiveFrom     string   `json:"effective_from,omitempty"`
	EffectiveTo       string   `json:"effective_to,omitempty"`
	ChangeNature      string   `json:"change_nature,omitempty"`
	ChangeCategory    string   `json:"change_category,omitempty"`
	PreferredURL      string   `json:"preferred_url,omitempty"`
	PreferredWorkURL  string   `json:"preferred_work_url,omitempty"`
	PublicationXMLURL string   `json:"publication_xml_url,omitempty"`
	Subjects          []string `json:"subjects,omitempty"`
	RawXML            string   `json:"raw_xml,omitempty"`
}

type searchRetrieveResponse struct {
	NumberOfRecords    int              `xml:"numberOfRecords"`
	NextRecordPosition int              `xml:"nextRecordPosition"`
	Records            []responseRecord `xml:"records>record"`
}

type responseRecord struct {
	RecordPosition int           `xml:"recordPosition"`
	RecordData     recordDataXML `xml:"recordData"`
}

type recordDataXML struct {
	InnerXML string `xml:",innerxml"`
}

func (c *Client) Discover(req DiscoverRequest) (DiscoverResponse, error) {
	if req.Query == "" {
		return DiscoverResponse{}, fmt.Errorf("query is required")
	}
	if req.StartRecord <= 0 {
		req.StartRecord = 1
	}
	if req.MaximumRecords <= 0 {
		req.MaximumRecords = 10
	}

	rawURL := buildSearchURL(c.baseURL(), req)
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}

	resp, err := httpClient.Get(rawURL)
	if err != nil {
		return DiscoverResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return DiscoverResponse{}, fmt.Errorf("cvdr discover failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return DiscoverResponse{}, err
	}

	var parsed searchRetrieveResponse
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return DiscoverResponse{}, err
	}

	out := DiscoverResponse{
		NumberOfRecords:    parsed.NumberOfRecords,
		NextRecordPosition: parsed.NextRecordPosition,
		Records:            make([]Record, 0, len(parsed.Records)),
		RawURL:             rawURL,
	}
	for _, rawRecord := range parsed.Records {
		record, err := parseRecordData(rawRecord.RecordData.InnerXML)
		if err != nil {
			return DiscoverResponse{}, err
		}
		record.RecordPosition = rawRecord.RecordPosition
		record.RawXML = strings.TrimSpace(rawRecord.RecordData.InnerXML)
		if record.CVDRID == "" {
			record.CVDRID = extractCVDRID(record.Identifier)
		}
		out.Records = append(out.Records, record)
	}

	return out, nil
}

func (c *Client) Harvest(req HarvestRequest) (HarvestResponse, error) {
	if req.BaseQuery == "" {
		return HarvestResponse{}, fmt.Errorf("base query is required")
	}
	start, end, err := parseDateRange(req.StartDate, req.EndDateExclusive)
	if err != nil {
		return HarvestResponse{}, err
	}
	if req.MaxWindowRecords <= 0 {
		req.MaxWindowRecords = 4000
	}
	if req.PageSize <= 0 {
		req.PageSize = 100
	}

	issuedRangeQuery := buildDateRangeQuery(req.BaseQuery, "issued", start, end)
	issuedTotalProbe, err := c.Discover(DiscoverRequest{
		Query:          issuedRangeQuery,
		StartRecord:    1,
		MaximumRecords: 1,
	})
	if err != nil {
		return HarvestResponse{}, err
	}

	issuedPartitions, err := c.partitionDateRange(req.BaseQuery, "issued", start, end, req.MaxWindowRecords)
	if err != nil {
		return HarvestResponse{}, err
	}

	versionSeen := map[string]struct{}{}
	workSeen := map[string]struct{}{}
	allRecords := make([]Record, 0)
	issuedPartitionSum := 0
	for i := range issuedPartitions {
		issuedPartitionSum += issuedPartitions[i].RecordCount
		if issuedPartitions[i].RecordCount == 0 {
			continue
		}

		records, pageCount, err := c.fetchPartitionRecords(issuedPartitions[i], req.PageSize)
		if err != nil {
			return HarvestResponse{}, err
		}
		issuedPartitions[i].PageCount = pageCount
		for _, record := range records {
			addRecord(record, versionSeen, workSeen, &allRecords)
		}
	}

	modifiedMatchingRecords := 0
	residualVersionCount := 0
	residualPartitions := []Partition{}
	if req.UseModifiedResidual {
		modifiedRangeQuery := buildDateRangeQuery(req.BaseQuery, "modified", start, end)
		modifiedTotalProbe, err := c.Discover(DiscoverRequest{
			Query:          modifiedRangeQuery,
			StartRecord:    1,
			MaximumRecords: 1,
		})
		if err != nil {
			return HarvestResponse{}, err
		}
		modifiedMatchingRecords = modifiedTotalProbe.NumberOfRecords
		if modifiedTotalProbe.NumberOfRecords > issuedTotalProbe.NumberOfRecords {
			residualPartitions, err = c.partitionDateRange(req.BaseQuery, "modified", start, end, req.MaxWindowRecords)
			if err != nil {
				return HarvestResponse{}, err
			}
			for i := range residualPartitions {
				if residualPartitions[i].RecordCount == 0 {
					continue
				}
				records, pageCount, err := c.fetchPartitionRecords(residualPartitions[i], req.PageSize)
				if err != nil {
					return HarvestResponse{}, err
				}
				residualPartitions[i].PageCount = pageCount
				for _, record := range records {
					if addRecord(record, versionSeen, workSeen, &allRecords) {
						residualVersionCount++
					}
				}
			}
		}
	}

	sort.Slice(allRecords, func(i, j int) bool {
		if allRecords[i].CVDRID == allRecords[j].CVDRID {
			return allRecords[i].Identifier < allRecords[j].Identifier
		}
		return allRecords[i].CVDRID < allRecords[j].CVDRID
	})

	return HarvestResponse{
		BaseQuery:               req.BaseQuery,
		StartDate:               start.Format("2006-01-02"),
		EndDateExclusive:        end.Format("2006-01-02"),
		WindowRecordLimit:       req.MaxWindowRecords,
		PageSize:                req.PageSize,
		IssuedMatchingRecords:   issuedTotalProbe.NumberOfRecords,
		ModifiedMatchingRecords: modifiedMatchingRecords,
		IssuedPartitionSum:      issuedPartitionSum,
		ResidualStrategy:        residualStrategy(req.UseModifiedResidual, residualVersionCount),
		ResidualVersionCount:    residualVersionCount,
		UniqueWorkCount:         len(workSeen),
		UniqueVersionCount:      len(versionSeen),
		IssuedPartitions:        issuedPartitions,
		ResidualPartitions:      residualPartitions,
		Works:                   buildWorkTimelines(allRecords),
		Records:                 allRecords,
		PartitioningComplete:    issuedPartitionSum == issuedTotalProbe.NumberOfRecords,
	}, nil
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return DefaultBaseURL
}

func (c *Client) partitionDateRange(baseQuery, field string, start, end time.Time, maxWindowRecords int) ([]Partition, error) {
	if !start.Before(end) {
		return nil, nil
	}

	query := buildDateRangeQuery(baseQuery, field, start, end)
	probe, err := c.Discover(DiscoverRequest{
		Query:          query,
		StartRecord:    1,
		MaximumRecords: 1,
	})
	if err != nil {
		return nil, err
	}
	if probe.NumberOfRecords == 0 {
		return nil, nil
	}
	if probe.NumberOfRecords <= maxWindowRecords {
		return []Partition{{
			Field:            field,
			StartDate:        start.Format("2006-01-02"),
			EndDateExclusive: end.Format("2006-01-02"),
			Query:            query,
			RecordCount:      probe.NumberOfRecords,
		}}, nil
	}

	if end.Sub(start) <= 24*time.Hour {
		return nil, fmt.Errorf("single-day window %s exceeds record limit %d with %d records", start.Format("2006-01-02"), maxWindowRecords, probe.NumberOfRecords)
	}

	mid := start.Add(end.Sub(start) / 2)
	mid = time.Date(mid.Year(), mid.Month(), mid.Day(), 0, 0, 0, 0, time.UTC)
	if !mid.After(start) {
		mid = start.AddDate(0, 0, 1)
	}
	if !end.After(mid) {
		return nil, fmt.Errorf("could not split date window %s-%s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}

	left, err := c.partitionDateRange(baseQuery, field, start, mid, maxWindowRecords)
	if err != nil {
		return nil, err
	}
	right, err := c.partitionDateRange(baseQuery, field, mid, end, maxWindowRecords)
	if err != nil {
		return nil, err
	}
	return append(left, right...), nil
}

func (c *Client) fetchPartitionRecords(partition Partition, pageSize int) ([]Record, int, error) {
	records := make([]Record, 0, partition.RecordCount)
	pageCount := 0
	for startRecord := 1; startRecord <= partition.RecordCount; startRecord += pageSize {
		resp, err := c.Discover(DiscoverRequest{
			Query:          partition.Query,
			StartRecord:    startRecord,
			MaximumRecords: pageSize,
		})
		if err != nil {
			return nil, 0, err
		}
		pageCount++
		records = append(records, resp.Records...)
		if len(resp.Records) == 0 {
			break
		}
	}
	return records, pageCount, nil
}

func buildSearchURL(baseURL string, req DiscoverRequest) string {
	values := url.Values{}
	values.Set("version", "1.2")
	values.Set("operation", "searchRetrieve")
	values.Set("x-connection", "cvdr")
	values.Set("startRecord", strconv.Itoa(req.StartRecord))
	values.Set("maximumRecords", strconv.Itoa(req.MaximumRecords))
	values.Set("query", req.Query)
	return baseURL + "?" + values.Encode()
}

func buildDateRangeQuery(baseQuery, field string, start, end time.Time) string {
	return fmt.Sprintf("(%s) and (%s>=\"%s\" and %s<\"%s\")", baseQuery, field, start.Format("2006-01-02"), field, end.Format("2006-01-02"))
}

func parseDateRange(startDate, endDateExclusive string) (time.Time, time.Time, error) {
	if startDate == "" {
		startDate = "1900-01-01"
	}
	if endDateExclusive == "" {
		endDateExclusive = time.Now().UTC().AddDate(1, 0, 0).Format("2006-01-02")
	}

	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start date")
	}
	end, err := time.Parse("2006-01-02", endDateExclusive)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end date")
	}
	if !start.Before(end) {
		return time.Time{}, time.Time{}, fmt.Errorf("end date must be after start date")
	}
	return start, end, nil
}

type xmlNode struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Content  string     `xml:",chardata"`
	Children []xmlNode  `xml:",any"`
}

func parseRecordData(recordData string) (Record, error) {
	var root xmlNode
	if err := xml.Unmarshal([]byte(recordData), &root); err != nil {
		return Record{}, err
	}

	record := Record{
		Identifier:        firstText(root, "identifier"),
		Title:             firstText(root, "title"),
		Alternative:       firstText(root, "alternative"),
		Creator:           firstText(root, "creator"),
		RatifiedBy:        firstText(root, "isRatifiedBy"),
		Type:              firstText(root, "type"),
		Modified:          firstText(root, "modified"),
		Issued:            firstText(root, "issued"),
		EffectiveFrom:     firstText(root, "inwerkingtredingDatum"),
		EffectiveTo:       firstText(root, "uitwerkingtredingDatum"),
		ChangeNature:      firstText(root, "betreft"),
		PreferredURL:      firstText(root, "preferred_url"),
		PreferredWorkURL:  firstText(root, "preferred_work_url"),
		PublicationXMLURL: firstText(root, "publicatieurl_xml"),
		Subjects:          allText(root, "subject"),
	}
	record.CVDRID = extractCVDRID(record.PreferredWorkURL)
	if record.CVDRID == "" {
		record.CVDRID = extractCVDRID(record.Identifier)
	}
	record.ChangeCategory = classifyChangeNature(record.ChangeNature)
	return record, nil
}

func firstText(node xmlNode, local string) string {
	if node.XMLName.Local == local {
		return strings.TrimSpace(node.Content)
	}
	for _, child := range node.Children {
		if value := firstText(child, local); value != "" {
			return value
		}
	}
	return ""
}

func allText(node xmlNode, local string) []string {
	values := []string{}
	if node.XMLName.Local == local {
		if text := strings.TrimSpace(node.Content); text != "" {
			values = append(values, text)
		}
	}
	for _, child := range node.Children {
		values = append(values, allText(child, local)...)
	}
	return values
}

func extractCVDRID(identifier string) string {
	if identifier == "" {
		return ""
	}
	idx := strings.Index(identifier, "CVDR")
	if idx == -1 {
		return ""
	}
	id := identifier[idx:]
	for i, r := range id {
		if !(r == '-' || r == '_' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			id = id[:i]
			break
		}
	}
	if underscore := strings.Index(id, "_"); underscore != -1 {
		suffix := id[underscore+1:]
		if suffix != "" && isDigits(suffix) {
			return id[:underscore]
		}
	}
	return id
}

func isDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value != ""
}

func classifyChangeNature(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case normalized == "":
		return "unknown"
	case strings.Contains(normalized, "nieuwe regeling"):
		return "new_regulation"
	case strings.Contains(normalized, "wijzig"):
		return "amendment"
	case strings.Contains(normalized, "intrekking"):
		return "repeal"
	case strings.Contains(normalized, "vervanging"):
		return "replacement"
	default:
		return "unknown"
	}
}

func addRecord(record Record, versionSeen, workSeen map[string]struct{}, allRecords *[]Record) bool {
	versionKey := record.PreferredURL
	if versionKey == "" {
		versionKey = record.Identifier
	}
	if _, ok := versionSeen[versionKey]; ok {
		return false
	}
	versionSeen[versionKey] = struct{}{}
	if record.CVDRID != "" {
		workSeen[record.CVDRID] = struct{}{}
	}
	*allRecords = append(*allRecords, record)
	return true
}

func buildWorkTimelines(records []Record) []WorkTimeline {
	grouped := map[string][]Record{}
	for _, record := range records {
		key := record.CVDRID
		if key == "" {
			key = record.Identifier
		}
		grouped[key] = append(grouped[key], record)
	}

	works := make([]WorkTimeline, 0, len(grouped))
	for cvdrID, versions := range grouped {
		sort.Slice(versions, func(i, j int) bool {
			return compareTimelineDate(versions[i], versions[j])
		})

		categorySet := map[string]struct{}{}
		for _, version := range versions {
			if version.ChangeCategory != "" {
				categorySet[version.ChangeCategory] = struct{}{}
			}
		}
		categories := make([]string, 0, len(categorySet))
		for category := range categorySet {
			categories = append(categories, category)
		}
		sort.Strings(categories)

		first := versions[0]
		last := versions[len(versions)-1]
		works = append(works, WorkTimeline{
			CVDRID:                cvdrID,
			Creator:               chooseNonEmpty(last.Creator, first.Creator),
			RepresentativeTitle:   chooseNonEmpty(last.Alternative, last.Title, first.Alternative, first.Title),
			VersionCount:          len(versions),
			EarliestIssued:        first.Issued,
			LatestIssued:          last.Issued,
			EarliestEffectiveFrom: first.EffectiveFrom,
			LatestEffectiveTo:     last.EffectiveTo,
			ChangeCategories:      categories,
			LatestChangeCategory:  last.ChangeCategory,
			LatestVersionURL:      chooseNonEmpty(last.PreferredURL, last.Identifier),
		})
	}

	sort.Slice(works, func(i, j int) bool {
		return works[i].CVDRID < works[j].CVDRID
	})
	return works
}

func compareTimelineDate(a, b Record) bool {
	ak := chooseNonEmpty(a.Issued, a.EffectiveFrom, a.Modified, a.Identifier)
	bk := chooseNonEmpty(b.Issued, b.EffectiveFrom, b.Modified, b.Identifier)
	if ak == bk {
		return a.Identifier < b.Identifier
	}
	return ak < bk
}

func chooseNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func residualStrategy(useModifiedResidual bool, residualVersionCount int) string {
	if !useModifiedResidual {
		return ""
	}
	if residualVersionCount > 0 {
		return "modified"
	}
	return "issued_only"
}
