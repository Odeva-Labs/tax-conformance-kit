package model

type RuleSet struct {
	ID               string            `json:"id"`
	Domain           string            `json:"domain"`
	Lifecycle        string            `json:"lifecycle"`
	Jurisdiction     Jurisdiction      `json:"jurisdiction"`
	Notes            string            `json:"notes"`
	AssessmentPolicy *AssessmentPolicy `json:"assessment_policy"`
	Rules            []Rule            `json:"rules"`
}

type Jurisdiction struct {
	CountryCode string `json:"country_code"`
	CountryName string `json:"country_name"`
	RegionCode  string `json:"region_code"`
	RegionName  string `json:"region_name"`
}

type AssessmentPolicy struct {
	Period                  string               `json:"period"`
	MinimumAssessmentAmount *MinimumAmountPolicy `json:"minimum_assessment_amount"`
	Notes                   string               `json:"notes"`
}

type MinimumAmountPolicy struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type Rule struct {
	ID               string      `json:"id"`
	MunicipalityCode string      `json:"municipality_code"`
	MunicipalityName string      `json:"municipality_name"`
	RegionCode       string      `json:"region_code"`
	ValidFrom        string      `json:"valid_from"`
	ValidTo          *string     `json:"valid_to"`
	AppliesTo        AppliesTo   `json:"applies_to"`
	Calculation      Calculation `json:"calculation"`
	Predicates       []Predicate `json:"predicates"`
	Exemptions       []Predicate `json:"exemptions"`
	Source           Source      `json:"source"`
	Confidence       string      `json:"confidence"`
	Notes            string      `json:"notes"`
}

type AppliesTo struct {
	AccommodationTypes []string `json:"accommodation_types"`
}

type Calculation struct {
	Kind     string         `json:"kind"`
	Params   map[string]any `json:"params"`
	Currency string         `json:"currency"`
}

type Predicate struct {
	Kind   string         `json:"kind"`
	Params map[string]any `json:"params"`
}

type Source struct {
	SourceURL  string  `json:"source_url"`
	CVDRID     string  `json:"cvdr_id"`
	ScrapedAt  *string `json:"scraped_at"`
	ReviewedAt *string `json:"reviewed_at"`
	Reviewer   string  `json:"reviewer"`
}

type Guest struct {
	Age  *int   `json:"age"`
	Role string `json:"role"`
}

type BookingInput struct {
	StayDate                  string   `json:"stay_date"`
	Nights                    int      `json:"nights"`
	Adults                    int      `json:"adults"`
	Children                  int      `json:"children"`
	Guests                    []Guest  `json:"guests"`
	MainGuestMunicipalityCode *string  `json:"main_guest_municipality_code"`
	PropertyMunicipalityCode  string   `json:"property_municipality_code"`
	AccommodationType         string   `json:"accommodation_type"`
	Subtotal                  float64  `json:"subtotal"`
	StayPurpose               string   `json:"stay_purpose"`
	AccommodationBroughtBy    string   `json:"accommodation_brought_by"`
	PricingArrangement        string   `json:"pricing_arrangement"`
	WTZACareInstitution       bool     `json:"wtza_care_institution"`
	COAAsylumHousing          bool     `json:"coa_asylum_housing"`
	PitchCount                int      `json:"pitch_count"`
	AlreadySubjectTo          []string `json:"already_subject_to"`
}

type ConformanceCase struct {
	ID           string       `json:"id"`
	Description  string       `json:"description"`
	RuleSetPath  string       `json:"rule_set_path"`
	BookingInput BookingInput `json:"booking_input"`
	Expected     Expected     `json:"expected"`
}

type AssessmentCase struct {
	ID              string               `json:"id"`
	Description     string               `json:"description"`
	RuleSetPath     string               `json:"rule_set_path"`
	AssessmentInput AssessmentInput      `json:"assessment_input"`
	Expected        AssessmentCaseResult `json:"expected"`
}

type AssessmentInput struct {
	PeriodStart string         `json:"period_start"`
	PeriodEnd   string         `json:"period_end"`
	Bookings    []BookingInput `json:"bookings"`
}

type Expected struct {
	TotalTax       float64  `json:"total_tax"`
	MatchedRuleIDs []string `json:"matched_rule_ids"`
}

type EvaluationResult struct {
	TotalTax       float64  `json:"total_tax"`
	MatchedRuleIDs []string `json:"matched_rule_ids"`
}

type AssessmentCaseResult struct {
	TotalBookingTax    float64            `json:"total_booking_tax"`
	TotalAssessmentTax float64            `json:"total_assessment_tax"`
	BookingResults     []EvaluationResult `json:"booking_results"`
}

type KindRegistry struct {
	RegistryVersion string               `json:"registry_version"`
	Calculations    map[string]KindEntry `json:"calculations"`
	Predicates      map[string]KindEntry `json:"predicates"`
}

type KindEntry struct {
	Description  string           `json:"description"`
	Since        string           `json:"since"`
	ParamsSchema map[string]any   `json:"params_schema"`
	Examples     []map[string]any `json:"examples"`
}
