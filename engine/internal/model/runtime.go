package model

const RuntimeAPIVersion = "v1"

type RuntimeValidateRequest struct {
	RuleSet      RuleSet       `json:"ruleset"`
	KindRegistry *KindRegistry `json:"kind_registry,omitempty"`
}

type RuntimeEvaluateRequest struct {
	RuleSet      RuleSet       `json:"ruleset"`
	BookingInput BookingInput  `json:"booking_input"`
	KindRegistry *KindRegistry `json:"kind_registry,omitempty"`
}

type RuntimeEvaluateAssessmentRequest struct {
	RuleSet         RuleSet         `json:"ruleset"`
	AssessmentInput AssessmentInput `json:"assessment_input"`
	KindRegistry    *KindRegistry   `json:"kind_registry,omitempty"`
}

type RuntimeResolveEvaluateRequest struct {
	FixtureRoot  string        `json:"fixture_root,omitempty"`
	Domain       string        `json:"domain,omitempty"`
	BookingInput BookingInput  `json:"booking_input"`
	KindRegistry *KindRegistry `json:"kind_registry,omitempty"`
}

type RuntimeResolveEvaluateAssessmentRequest struct {
	FixtureRoot     string          `json:"fixture_root,omitempty"`
	Domain          string          `json:"domain,omitempty"`
	AssessmentInput AssessmentInput `json:"assessment_input"`
	KindRegistry    *KindRegistry   `json:"kind_registry,omitempty"`
}

type RuntimeError struct {
	Message string `json:"message"`
}

type RuntimeValidateResponse struct {
	APIVersion string        `json:"api_version"`
	OK         bool          `json:"ok"`
	RuleCount  int           `json:"rule_count,omitempty"`
	Error      *RuntimeError `json:"error,omitempty"`
}

type RuntimeEvaluateResponse struct {
	APIVersion string            `json:"api_version"`
	OK         bool              `json:"ok"`
	RuleCount  int               `json:"rule_count,omitempty"`
	Result     *EvaluationResult `json:"result,omitempty"`
	Error      *RuntimeError     `json:"error,omitempty"`
}

type RuntimeResolveEvaluateResponse struct {
	APIVersion          string            `json:"api_version"`
	OK                  bool              `json:"ok"`
	RuleCount           int               `json:"rule_count,omitempty"`
	ResolvedRuleSetID   string            `json:"resolved_ruleset_id,omitempty"`
	ResolvedRuleSetPath string            `json:"resolved_ruleset_path,omitempty"`
	Result              *EvaluationResult `json:"result,omitempty"`
	Error               *RuntimeError     `json:"error,omitempty"`
}

type ResolvedAssessmentResult struct {
	ResolvedRuleSetID   string               `json:"resolved_ruleset_id"`
	ResolvedRuleSetPath string               `json:"resolved_ruleset_path"`
	RuleCount           int                  `json:"rule_count"`
	BookingCount        int                  `json:"booking_count"`
	Result              AssessmentCaseResult `json:"result"`
}

type RuntimeResolveEvaluateAssessmentResponse struct {
	APIVersion          string                     `json:"api_version"`
	OK                  bool                       `json:"ok"`
	GroupCount          int                        `json:"group_count,omitempty"`
	TotalBookingTax     float64                    `json:"total_booking_tax,omitempty"`
	TotalAssessmentTax  float64                    `json:"total_assessment_tax,omitempty"`
	ResolvedAssessments []ResolvedAssessmentResult `json:"resolved_assessments,omitempty"`
	Error               *RuntimeError              `json:"error,omitempty"`
}

type RuntimeEvaluateAssessmentResponse struct {
	APIVersion string                `json:"api_version"`
	OK         bool                  `json:"ok"`
	RuleCount  int                   `json:"rule_count,omitempty"`
	Result     *AssessmentCaseResult `json:"result,omitempty"`
	Error      *RuntimeError         `json:"error,omitempty"`
}
