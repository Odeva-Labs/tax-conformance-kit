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

type RuntimeEvaluateAssessmentResponse struct {
	APIVersion string                `json:"api_version"`
	OK         bool                  `json:"ok"`
	RuleCount  int                   `json:"rule_count,omitempty"`
	Result     *AssessmentCaseResult `json:"result,omitempty"`
	Error      *RuntimeError         `json:"error,omitempty"`
}
