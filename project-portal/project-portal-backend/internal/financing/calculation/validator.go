package calculation

import (
	"context"
	"fmt"
	"time"
)

// Validator handles validation of calculation inputs and results
type Validator struct {
	rules []ValidationRule
}

// ValidationRule defines a validation rule interface
type ValidationRule interface {
	Validate(ctx context.Context, req *CalculationInput) *RuleResult
	GetName() string
	GetDescription() string
}

// RuleResult represents the result of a validation rule
type RuleResult struct {
	Passed   bool
	Message  string
	Severity string // "error", "warning", "info"
	Details  map[string]interface{}
}

// NewValidator creates a new validator with default rules
func NewValidator() *Validator {
	validator := &Validator{
		rules: []ValidationRule{
			&MonitoringPeriodRule{},
			&DataAvailabilityRule{},
			&DataQualityRule{},
			&MethodologyCompatibilityRule{},
			&TemporalConsistencyRule{},
			&GeographicCoverageRule{},
			&BaselineDataRule{},
		},
	}
	return validator
}

// Validate performs comprehensive validation
func (v *Validator) Validate(ctx context.Context, req *CalculationInput) (*ValidationResult, error) {
	result := &ValidationResult{
		IsValid:      true,
		Errors:       []string{},
		Warnings:     []string{},
		QualityScore: req.DataQualityScore,
		Requirements: []RequirementCheck{},
	}
	
	for _, rule := range v.rules {
		ruleResult := rule.Validate(ctx, req)
		
		// Convert rule result to validation result
		if !ruleResult.Passed {
			if ruleResult.Severity == "error" {
				result.IsValid = false
				result.Errors = append(result.Errors, ruleResult.Message)
			} else if ruleResult.Severity == "warning" {
				result.Warnings = append(result.Warnings, ruleResult.Message)
			}
		}
		
		result.Requirements = append(result.Requirements, RequirementCheck{
			Requirement: rule.GetName(),
			Met:         ruleResult.Passed,
			Details:     ruleResult.Message,
		})
	}
	
	return result, nil
}

// AddRule adds a custom validation rule
func (v *Validator) AddRule(rule ValidationRule) {
	v.rules = append(v.rules, rule)
}

// MonitoringPeriodRule validates minimum monitoring period
type MonitoringPeriodRule struct{}

func (r *MonitoringPeriodRule) GetName() string {
	return "Monitoring Period"
}

func (r *MonitoringPeriodRule) GetDescription() string {
	return "Validates minimum monitoring period requirements"
}

func (r *MonitoringPeriodRule) Validate(ctx context.Context, req *CalculationInput) *RuleResult {
	monitoringPeriod := req.CalculationPeriodEnd.Sub(req.CalculationPeriodStart)
	
	// Different methodologies have different requirements
	var minPeriod time.Duration
	switch req.MethodologyCode {
	case "VM0007", "VM0015":
		minPeriod = 365 * 24 * time.Hour // 1 year
	case "VM0033":
		minPeriod = 730 * 24 * time.Hour // 2 years
	default:
		minPeriod = 365 * 24 * time.Hour // Default 1 year
	}
	
	if monitoringPeriod < minPeriod {
		return &RuleResult{
			Passed:   false,
			Message:  fmt.Sprintf("Monitoring period (%v) is less than minimum required (%v)", monitoringPeriod, minPeriod),
			Severity: "error",
			Details: map[string]interface{}{
				"actual_period":    monitoringPeriod,
				"required_period":  minPeriod,
				"methodology":      req.MethodologyCode,
			},
		}
	}
	
	return &RuleResult{
		Passed:   true,
		Message:  fmt.Sprintf("Monitoring period (%v) meets requirements", monitoringPeriod),
		Severity: "info",
		Details: map[string]interface{}{
			"monitoring_period": monitoringPeriod,
		},
	}
}

// DataAvailabilityRule validates data availability
type DataAvailabilityRule struct{}

func (r *DataAvailabilityRule) GetName() string {
	return "Data Availability"
}

func (r *DataAvailabilityRule) GetDescription() string {
	return "Validates availability of required monitoring data"
}

func (r *DataAvailabilityRule) Validate(ctx context.Context, req *CalculationInput) *RuleResult {
	totalDataPoints := len(req.MonitoringData.SatelliteData) + 
		len(req.MonitoringData.IoTSensorData) + 
		len(req.MonitoringData.ManualMeasurements)
	
	if totalDataPoints == 0 {
		return &RuleResult{
			Passed:   false,
			Message:  "No monitoring data available",
			Severity: "error",
			Details: map[string]interface{}{
				"satellite_points": len(req.MonitoringData.SatelliteData),
				"iot_points":       len(req.MonitoringData.IoTSensorData),
				"manual_points":    len(req.MonitoringData.ManualMeasurements),
			},
		}
	}
	
	// Check methodology-specific requirements
	switch req.MethodologyCode {
	case "VM0015": // Avoided deforestation requires satellite data
		if len(req.MonitoringData.SatelliteData) == 0 {
			return &RuleResult{
				Passed:   false,
				Message:  "Satellite data required for deforestation monitoring",
				Severity: "error",
				Details: map[string]interface{}{
					"required_data": "satellite",
					"methodology":   "VM0015",
				},
			}
		}
	case "VM0033": // Soil carbon requires soil data
		if len(req.MonitoringData.SoilData) == 0 {
			return &RuleResult{
				Passed:   false,
				Message:  "Soil measurements required for soil carbon calculation",
				Severity: "error",
				Details: map[string]interface{}{
					"required_data": "soil",
					"methodology":   "VM0033",
				},
			}
		}
	}
	
	return &RuleResult{
		Passed:   true,
		Message:  fmt.Sprintf("Data availability adequate (%d data points)", totalDataPoints),
		Severity: "info",
		Details: map[string]interface{}{
			"total_data_points": totalDataPoints,
		},
	}
}

// DataQualityRule validates data quality scores
type DataQualityRule struct{}

func (r *DataQualityRule) GetName() string {
	return "Data Quality"
}

func (r *DataQualityRule) GetDescription() string {
	return "Validates minimum data quality requirements"
}

func (r *DataQualityRule) Validate(ctx context.Context, req *CalculationInput) *RuleResult {
	if req.DataQualityScore < 0.3 {
		return &RuleResult{
			Passed:   false,
			Message:  fmt.Sprintf("Data quality score (%.2f) below minimum threshold (0.30)", req.DataQualityScore),
			Severity: "error",
			Details: map[string]interface{}{
				"quality_score": req.DataQualityScore,
				"threshold":     0.30,
			},
		}
	}
	
	if req.DataQualityScore < 0.6 {
		return &RuleResult{
			Passed:   true,
			Message:  fmt.Sprintf("Data quality score (%.2f) is low, consider improving data collection", req.DataQualityScore),
			Severity: "warning",
			Details: map[string]interface{}{
				"quality_score": req.DataQualityScore,
				"recommended":   0.60,
			},
		}
	}
	
	return &RuleResult{
		Passed:   true,
		Message:  fmt.Sprintf("Data quality score (%.2f) meets requirements", req.DataQualityScore),
		Severity: "info",
		Details: map[string]interface{}{
			"quality_score": req.DataQualityScore,
		},
	}
}

// MethodologyCompatibilityRule validates methodology compatibility
type MethodologyCompatibilityRule struct{}

func (r *MethodologyCompatibilityRule) GetName() string {
	return "Methodology Compatibility"
}

func (r *MethodologyCompatibilityRule) GetDescription() string {
	return "Validates methodology compatibility with project type"
}

func (r *MethodologyCompatibilityRule) Validate(ctx context.Context, req *CalculationInput) *RuleResult {
	// This would typically check project type against methodology
	// For now, we'll just validate the methodology code is supported
	supportedMethodologies := map[string]bool{
		"VM0007": true,
		"VM0015": true,
		"VM0033": true,
	}
	
	if !supportedMethodologies[req.MethodologyCode] {
		return &RuleResult{
			Passed:   false,
			Message:  fmt.Sprintf("Unsupported methodology: %s", req.MethodologyCode),
			Severity: "error",
			Details: map[string]interface{}{
				"methodology": req.MethodologyCode,
				"supported":   []string{"VM0007", "VM0015", "VM0033"},
			},
		}
	}
	
	return &RuleResult{
		Passed:   true,
		Message:  fmt.Sprintf("Methodology %s is supported", req.MethodologyCode),
		Severity: "info",
		Details: map[string]interface{}{
			"methodology": req.MethodologyCode,
		},
	}
}

// TemporalConsistencyRule validates temporal consistency of data
type TemporalConsistencyRule struct{}

func (r *TemporalConsistencyRule) GetName() string {
	return "Temporal Consistency"
}

func (r *TemporalConsistencyRule) GetDescription() string {
	return "Validates temporal consistency of monitoring data"
}

func (r *TemporalConsistencyRule) Validate(ctx context.Context, req *CalculationInput) *RuleResult {
	// Check if data timestamps are within calculation period
	var outOfRangeCount int
	var allTimestamps []time.Time
	
	// Collect all timestamps
	for _, data := range req.MonitoringData.SatelliteData {
		allTimestamps = append(allTimestamps, data.Timestamp)
	}
	for _, data := range req.MonitoringData.IoTSensorData {
		allTimestamps = append(allTimestamps, data.Timestamp)
	}
	for _, data := range req.MonitoringData.ManualMeasurements {
		allTimestamps = append(allTimestamps, data.Timestamp)
	}
	
	// Check each timestamp
	for _, timestamp := range allTimestamps {
		if timestamp.Before(req.CalculationPeriodStart) || timestamp.After(req.CalculationPeriodEnd) {
			outOfRangeCount++
		}
	}
	
	if len(allTimestamps) > 0 {
		outOfRangePercent := float64(outOfRangeCount) / float64(len(allTimestamps))
		
		if outOfRangePercent > 0.2 { // More than 20% out of range
			return &RuleResult{
				Passed:   false,
				Message:  fmt.Sprintf("%.1f%% of data points outside calculation period", outOfRangePercent*100),
				Severity: "error",
				Details: map[string]interface{}{
					"out_of_range_count": outOfRangeCount,
					"total_points":       len(allTimestamps),
					"percentage":         outOfRangePercent,
				},
			}
		}
		
		if outOfRangePercent > 0.05 { // More than 5% out of range
			return &RuleResult{
				Passed:   true,
				Message:  fmt.Sprintf("%.1f%% of data points outside calculation period", outOfRangePercent*100),
				Severity: "warning",
				Details: map[string]interface{}{
					"out_of_range_count": outOfRangeCount,
					"total_points":       len(allTimestamps),
					"percentage":         outOfRangePercent,
				},
			}
		}
	}
	
	return &RuleResult{
		Passed:   true,
		Message:  "Data timestamps are within calculation period",
		Severity: "info",
		Details: map[string]interface{}{
			"total_points": len(allTimestamps),
		},
	}
}

// GeographicCoverageRule validates geographic coverage
type GeographicCoverageRule struct{}

func (r *GeographicCoverageRule) GetName() string {
	return "Geographic Coverage"
}

func (r *GeographicCoverageRule) GetDescription() string {
	return "Validates geographic coverage of monitoring data"
}

func (r *GeographicCoverageRule) Validate(ctx context.Context, req *CalculationInput) *RuleResult {
	// Check if we have geographic coordinates
	var coordinateCount int
	var latitudes, longitudes []float64
	
	for _, data := range req.MonitoringData.SatelliteData {
		if data.Latitude != 0 && data.Longitude != 0 {
			coordinateCount++
			latitudes = append(latitudes, data.Latitude)
			longitudes = append(longitudes, data.Longitude)
		}
	}
	
	for _, data := range req.MonitoringData.IoTSensorData {
		if data.Latitude != 0 && data.Longitude != 0 {
			coordinateCount++
			latitudes = append(latitudes, data.Latitude)
			longitudes = append(longitudes, data.Longitude)
		}
	}
	
	for _, data := range req.MonitoringData.ManualMeasurements {
		if data.Latitude != 0 && data.Longitude != 0 {
			coordinateCount++
			latitudes = append(latitudes, data.Latitude)
			longitudes = append(longitudes, data.Longitude)
		}
	}
	
	if coordinateCount == 0 {
		return &RuleResult{
			Passed:   false,
			Message:  "No geographic coordinates available in monitoring data",
			Severity: "error",
			Details: map[string]interface{}{
				"required": "latitude and longitude",
			},
		}
	}
	
	// Check coordinate validity
	var invalidCoordinates int
	for i := 0; i < len(latitudes); i++ {
		lat := latitudes[i]
		lon := longitudes[i]
		
		if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			invalidCoordinates++
		}
	}
	
	if invalidCoordinates > 0 {
		return &RuleResult{
			Passed:   false,
			Message:  fmt.Sprintf("%d invalid geographic coordinates found", invalidCoordinates),
			Severity: "error",
			Details: map[string]interface{}{
				"invalid_count": invalidCoordinates,
				"total_count":   coordinateCount,
			},
		}
	}
	
	return &RuleResult{
		Passed:   true,
		Message:  fmt.Sprintf("Geographic coverage adequate (%d coordinate points)", coordinateCount),
		Severity: "info",
		Details: map[string]interface{}{
			"coordinate_points": coordinateCount,
		},
	}
}

// BaselineDataRule validates baseline data requirements
type BaselineDataRule struct{}

func (r *BaselineDataRule) GetName() string {
	return "Baseline Data"
}

func (r *BaselineDataRule) GetDescription() string {
	return "Validates baseline data completeness and validity"
}

func (r *BaselineDataRule) Validate(ctx context.Context, req *CalculationInput) *RuleResult {
	baseline := req.BaselineData
	
	// Check baseline scenario
	if baseline.ReferenceScenario == "" {
		return &RuleResult{
			Passed:   false,
			Message:  "Baseline scenario not specified",
			Severity: "error",
			Details: map[string]interface{}{
				"required": "reference_scenario",
			},
		}
	}
	
	// Check baseline values
	if baseline.BaselineEmissions < 0 {
		return &RuleResult{
			Passed:   false,
			Message:  "Baseline emissions cannot be negative",
			Severity: "error",
			Details: map[string]interface{}{
				"baseline_emissions": baseline.BaselineEmissions,
			},
		}
	}
	
	if baseline.BaselineRemovals < 0 {
		return &RuleResult{
			Passed:   false,
			Message:  "Baseline removals cannot be negative",
			Severity: "error",
			Details: map[string]interface{}{
				"baseline_removals": baseline.BaselineRemovals,
			},
		}
	}
	
	// Check conservativeness factor
	if baseline.ConservativenessFactor <= 0 || baseline.ConservativenessFactor > 1 {
		return &RuleResult{
			Passed:   false,
			Message:  "Conservativeness factor must be between 0 and 1",
			Severity: "error",
			Details: map[string]interface{}{
				"conservativeness_factor": baseline.ConservativenessFactor,
				"valid_range":            "0.0 - 1.0",
			},
		}
	}
	
	return &RuleResult{
		Passed:   true,
		Message:  "Baseline data is complete and valid",
		Severity: "info",
		Details: map[string]interface{}{
			"reference_scenario": baseline.ReferenceScenario,
			"baseline_emissions": baseline.BaselineEmissions,
			"baseline_removals":  baseline.BaselineRemovals,
		},
	}
}
