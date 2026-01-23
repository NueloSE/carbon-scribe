package calculation

import (
	"context"
	"fmt"
	"math"
	"time"
)

// VM0007Methodology implements VM0007 - Improved Forest Management
type VM0007Methodology struct{}

// NewVM0007Methodology creates a new VM0007 methodology instance
func NewVM0007Methodology() *VM0007Methodology {
	return &VM0007Methodology{}
}

func (m *VM0007Methodology) GetCode() string {
	return "VM0007"
}

func (m *VM0007Methodology) GetName() string {
	return "Improved Forest Management"
}

func (m *VM0007Methodology) GetMinimumMonitoringPeriod() time.Duration {
	return 365 * 24 * time.Hour // 1 year
}

func (m *VM0007Methodology) Validate(ctx context.Context, req *CalculationInput) (*ValidationResult, error) {
	result := &ValidationResult{
		IsValid:      true,
		Errors:       []string{},
		Warnings:     []string{},
		QualityScore: req.DataQualityScore,
		Requirements: []RequirementCheck{},
	}
	
	// Check minimum monitoring period
	monitoringPeriod := req.CalculationPeriodEnd.Sub(req.CalculationPeriodStart)
	if monitoringPeriod < m.GetMinimumMonitoringPeriod() {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Monitoring period (%v) is less than minimum required (%v)", 
			monitoringPeriod, m.GetMinimumMonitoringPeriod()))
		result.Requirements = append(result.Requirements, RequirementCheck{
			Requirement: "Minimum monitoring period",
			Met:         false,
			Details:     "At least 1 year of monitoring data required",
		})
	} else {
		result.Requirements = append(result.Requirements, RequirementCheck{
			Requirement: "Minimum monitoring period",
			Met:         true,
			Details:     fmt.Sprintf("Monitoring period: %v", monitoringPeriod),
		})
	}
	
	// Check data availability
	if len(req.MonitoringData.SatelliteData) == 0 && len(req.MonitoringData.IoTSensorData) == 0 {
		result.IsValid = false
		result.Errors = append(result.Errors, "No monitoring data available")
		result.Warnings = append(result.Warnings, "Consider adding satellite or IoT sensor data for better accuracy")
	}
	
	// Check baseline data
	if req.BaselineData.ReferenceScenario == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, "Baseline scenario not specified")
	}
	
	// Quality score validation
	if req.DataQualityScore < 0.3 {
		result.Warnings = append(result.Warnings, "Low data quality score may affect credit accuracy")
	}
	
	return result, nil
}

func (m *VM0007Methodology) Calculate(ctx context.Context, req *CalculationInput) (*CalculationResult, error) {
	result := &CalculationResult{
		CalculationSteps: []CalculationStep{},
		Warnings:         []string{},
	}
	
	// Step 1: Calculate baseline emissions
	baselineEmissions := req.BaselineData.BaselineEmissions
	result.CalculationSteps = append(result.CalculationSteps, CalculationStep{
		StepName:    "Baseline Emissions",
		Formula:     "BE = Baseline Emissions",
		InputValues: map[string]interface{}{"baseline_emissions": baselineEmissions},
		Result:      baselineEmissions,
		Unit:        "tCO2e/year",
		Timestamp:   time.Now(),
	})
	
	// Step 2: Calculate project emissions from monitoring data
	projectEmissions := m.calculateProjectEmissions(req.MonitoringData)
	result.CalculationSteps = append(result.CalculationSteps, CalculationStep{
		StepName:    "Project Emissions",
		Formula:     "PE = Σ(Monitoring Data)",
		InputValues: map[string]interface{}{"monitoring_data_points": len(req.MonitoringData.IoTSensorData)},
		Result:      projectEmissions,
		Unit:        "tCO2e/year",
		Timestamp:   time.Now(),
	})
	
	// Step 3: Calculate carbon removals/sequestration
	carbonRemovals := m.calculateCarbonRemovals(req.MonitoringData)
	result.CalculationSteps = append(result.CalculationSteps, CalculationStep{
		StepName:    "Carbon Removals",
		Formula:     "CR = Biomass Growth × Carbon Factor",
		InputValues: map[string]interface{}{"satellite_measurements": len(req.MonitoringData.SatelliteData)},
		Result:      carbonRemovals,
		Unit:        "tCO2e/year",
		Timestamp:   time.Now(),
	})
	
	// Step 4: Apply conservativeness factor
	conservativenessFactor := req.BaselineData.ConservativenessFactor
	if conservativenessFactor == 0 {
		conservativenessFactor = 0.9 // Default 90% conservativeness
	}
	
	adjustedRemovals := carbonRemovals * conservativenessFactor
	result.CalculationSteps = append(result.CalculationSteps, CalculationStep{
		StepName:    "Conservativeness Adjustment",
		Formula:     "AR = CR × CF",
		InputValues: map[string]interface{}{"carbon_removals": carbonRemovals, "conservativeness_factor": conservativenessFactor},
		Result:      adjustedRemovals,
		Unit:        "tCO2e/year",
		Timestamp:   time.Now(),
	})
	
	// Step 5: Calculate net carbon benefits
	netBenefits := (baselineEmissions - projectEmissions) + adjustedRemovals
	result.CalculationSteps = append(result.CalculationSteps, CalculationStep{
		StepName:    "Net Carbon Benefits",
		Formula:     "NCB = (BE - PE) + AR",
		InputValues: map[string]interface{}{"baseline_emissions": baselineEmissions, "project_emissions": projectEmissions, "adjusted_removals": adjustedRemovals},
		Result:      netBenefits,
		Unit:        "tCO2e/year",
		Timestamp:   time.Now(),
	})
	
	// Step 6: Annualize based on calculation period
	periodYears := req.CalculationPeriodEnd.Sub(req.CalculationPeriodStart).Hours() / (365.25 * 24)
	annualCredits := netBenefits / periodYears
	result.CalculationSteps = append(result.CalculationSteps, CalculationStep{
		StepName:    "Annualized Credits",
		Formula:     "AC = NCB / Years",
		InputValues: map[string]interface{}{"net_benefits": netBenefits, "period_years": periodYears},
		Result:      annualCredits,
		Unit:        "tCO2e/year",
		Timestamp:   time.Now(),
	})
	
	result.CalculatedTons = math.Max(0, annualCredits)
	result.DataQualityScore = req.DataQualityScore
	result.ConfidenceLevel = m.calculateConfidenceLevel(req)
	
	return result, nil
}

func (m *VM0007Methodology) calculateProjectEmissions(data MonitoringData) float64 {
	var totalEmissions float64
	
	// Calculate emissions from IoT sensor data
	for _, measurement := range data.IoTSensorData {
		// CO2 emissions from concentration measurements
		co2Emissions := measurement.CO2Concentration * 0.0001 // Simplified conversion
		ch4Emissions := measurement.CH4Concentration * 0.000025 // CH4 has higher GWP
		
		totalEmissions += co2Emissions + ch4Emissions
	}
	
	return totalEmissions
}

func (m *VM0007Methodology) calculateCarbonRemovals(data MonitoringData) float64 {
	var totalRemovals float64
	
	// Calculate biomass growth from satellite data
	for _, measurement := range data.SatelliteData {
		// Use NDVI and LAI to estimate biomass growth
		ndviFactor := math.Max(0, measurement.NDVI)
		laiFactor := measurement.LAI
		
		// Simplified biomass calculation
		biomassGrowth := ndviFactor * laiFactor * 2.5 // Simplified factor
		carbonSequestration := biomassGrowth * 0.47 // Carbon is ~47% of biomass
		
		totalRemovals += carbonSequestration
	}
	
	// Add manual measurements if available
	for _, measurement := range data.ManualMeasurements {
		// Tree biomass calculation
		treeBiomass := m.calculateTreeBiomass(measurement.TreeHeight, measurement.DBH)
		carbonContent := treeBiomass * 0.47
		
		totalRemovals += carbonContent
	}
	
	return totalRemovals
}

func (m *VM0007Methodology) calculateTreeBiomass(height, dbh float64) float64 {
	// Simplified allometric equation for tropical trees
	// Biomass = 0.25 × ρ × DBH^2 × H
	// where ρ is wood density (default 0.6 g/cm3 for tropical trees)
	woodDensity := 0.6
	biomass := 0.25 * woodDensity * math.Pow(dbh, 2) * height
	return biomass
}

func (m *VM0007Methodology) calculateConfidenceLevel(req *CalculationInput) float64 {
	confidence := req.DataQualityScore
	
	// Adjust based on data sources
	if len(req.MonitoringData.SatelliteData) > 0 {
		confidence += 0.1
	}
	if len(req.MonitoringData.IoTSensorData) > 0 {
		confidence += 0.1
	}
	if req.ThirdPartyVerification {
		confidence += 0.15
	}
	
	return math.Min(1.0, confidence)
}

// VM0015Methodology implements VM0015 - Avoided Deforestation
type VM0015Methodology struct{}

// NewVM0015Methodology creates a new VM0015 methodology instance
func NewVM0015Methodology() *VM0015Methodology {
	return &VM0015Methodology{}
}

func (m *VM0015Methodology) GetCode() string {
	return "VM0015"
}

func (m *VM0015Methodology) GetName() string {
	return "Avoided Deforestation"
}

func (m *VM0015Methodology) GetMinimumMonitoringPeriod() time.Duration {
	return 365 * 24 * time.Hour // 1 year
}

func (m *VM0015Methodology) Validate(ctx context.Context, req *CalculationInput) (*ValidationResult, error) {
	result := &ValidationResult{
		IsValid:      true,
		Errors:       []string{},
		Warnings:     []string{},
		QualityScore: req.DataQualityScore,
		Requirements: []RequirementCheck{},
	}
	
	// Similar validation as VM0007 but with specific requirements for deforestation
	monitoringPeriod := req.CalculationPeriodEnd.Sub(req.CalculationPeriodStart)
	if monitoringPeriod < m.GetMinimumMonitoringPeriod() {
		result.IsValid = false
		result.Errors = append(result.Errors, "Insufficient monitoring period for deforestation assessment")
	}
	
	// Check for forest cover data
	if len(req.MonitoringData.SatelliteData) == 0 {
		result.IsValid = false
		result.Errors = append(result.Errors, "Satellite data required for deforestation monitoring")
	}
	
	return result, nil
}

func (m *VM0015Methodology) Calculate(ctx context.Context, req *CalculationInput) (*CalculationResult, error) {
	result := &CalculationResult{
		CalculationSteps: []CalculationStep{},
		Warnings:         []string{},
	}
	
	// Step 1: Calculate baseline deforestation rate
	baselineDeforestation := m.calculateBaselineDeforestation(req.MonitoringData)
	result.CalculationSteps = append(result.CalculationSteps, CalculationStep{
		StepName:    "Baseline Deforestation",
		Formula:     "BD = Historical Forest Loss Rate",
		InputValues: map[string]interface{}{"satellite_data_points": len(req.MonitoringData.SatelliteData)},
		Result:      baselineDeforestation,
		Unit:        "ha/year",
		Timestamp:   time.Now(),
	})
	
	// Step 2: Calculate actual deforestation
	actualDeforestation := m.calculateActualDeforestation(req.MonitoringData)
	result.CalculationSteps = append(result.CalculationSteps, CalculationStep{
		StepName:    "Actual Deforestation",
		Formula:     "AD = Current Forest Loss",
		InputValues: map[string]interface{}{"monitoring_period": req.CalculationPeriodEnd.Sub(req.CalculationPeriodStart)},
		Result:      actualDeforestation,
		Unit:        "ha/year",
		Timestamp:   time.Now(),
	})
	
	// Step 3: Calculate avoided deforestation
	avoidedDeforestation := baselineDeforestation - actualDeforestation
	result.CalculationSteps = append(result.CalculationSteps, CalculationStep{
		StepName:    "Avoided Deforestation",
		Formula:     "ADv = BD - AD",
		InputValues: map[string]interface{}{"baseline": baselineDeforestation, "actual": actualDeforestation},
		Result:      avoidedDeforestation,
		Unit:        "ha/year",
		Timestamp:   time.Now(),
	})
	
	// Step 4: Convert to CO2e
	co2PerHectare := 200.0 // Average carbon stock per hectare
	avoidedEmissions := avoidedDeforestation * co2PerHectare
	result.CalculationSteps = append(result.CalculationSteps, CalculationStep{
		StepName:    "Avoided Emissions",
		Formula:     "AE = ADv × CO2/ha",
		InputValues: map[string]interface{}{"avoided_deforestation": avoidedDeforestation, "co2_per_hectare": co2PerHectare},
		Result:      avoidedEmissions,
		Unit:        "tCO2e/year",
		Timestamp:   time.Now(),
	})
	
	result.CalculatedTons = math.Max(0, avoidedEmissions)
	result.DataQualityScore = req.DataQualityScore
	result.ConfidenceLevel = m.calculateConfidenceLevel(req)
	
	return result, nil
}

func (m *VM0015Methodology) calculateBaselineDeforestation(data MonitoringData) float64 {
	// Simplified baseline calculation based on historical trends
	// In practice, this would use historical satellite data
	return 5.0 // ha/year - simplified baseline
}

func (m *VM0015Methodology) calculateActualDeforestation(data MonitoringData) float64 {
	var forestLoss float64
	
	// Calculate forest loss from satellite data
	for _, measurement := range data.SatelliteData {
		// Use NDVI to detect forest loss
		if measurement.NDVI < 0.3 { // Threshold for non-forest
			forestLoss += 0.1 // Simplified calculation
		}
	}
	
	return forestLoss
}

func (m *VM0015Methodology) calculateConfidenceLevel(req *CalculationInput) float64 {
	confidence := req.DataQualityScore * 0.9 // Slightly lower confidence for deforestation projects
	
	if len(req.MonitoringData.SatelliteData) > 10 {
		confidence += 0.1
	}
	
	return math.Min(1.0, confidence)
}

// VM0033Methodology implements VM0033 - Soil Carbon Sequestration
type VM0033Methodology struct{}

// NewVM0033Methodology creates a new VM0033 methodology instance
func NewVM0033Methodology() *VM0033Methodology {
	return &VM0033Methodology{}
}

func (m *VM0033Methodology) GetCode() string {
	return "VM0033"
}

func (m *VM0033Methodology) GetName() string {
	return "Soil Carbon Sequestration"
}

func (m *VM0033Methodology) GetMinimumMonitoringPeriod() time.Duration {
	return 730 * 24 * time.Hour // 2 years
}

func (m *VM0033Methodology) Validate(ctx context.Context, req *CalculationInput) (*ValidationResult, error) {
	result := &ValidationResult{
		IsValid:      true,
		Errors:       []string{},
		Warnings:     []string{},
		QualityScore: req.DataQualityScore,
		Requirements: []RequirementCheck{},
	}
	
	// Check minimum monitoring period (longer for soil carbon)
	monitoringPeriod := req.CalculationPeriodEnd.Sub(req.CalculationPeriodStart)
	if monitoringPeriod < m.GetMinimumMonitoringPeriod() {
		result.IsValid = false
		result.Errors = append(result.Errors, "Soil carbon requires minimum 2 years monitoring")
	}
	
	// Check for soil data
	if len(req.MonitoringData.SoilData) == 0 {
		result.IsValid = false
		result.Errors = append(result.Errors, "Soil measurements required for soil carbon calculation")
	}
	
	return result, nil
}

func (m *VM0033Methodology) Calculate(ctx context.Context, req *CalculationInput) (*CalculationResult, error) {
	result := &CalculationResult{
		CalculationSteps: []CalculationStep{},
		Warnings:         []string{},
	}
	
	// Step 1: Calculate baseline soil carbon
	baselineSoilCarbon := m.calculateBaselineSoilCarbon(req.MonitoringData)
	result.CalculationSteps = append(result.CalculationSteps, CalculationStep{
		StepName:    "Baseline Soil Carbon",
		Formula:     "BSC = Initial Soil Carbon Measurements",
		InputValues: map[string]interface{}{"soil_samples": len(req.MonitoringData.SoilData)},
		Result:      baselineSoilCarbon,
		Unit:        "tC/ha",
		Timestamp:   time.Now(),
	})
	
	// Step 2: Calculate current soil carbon
	currentSoilCarbon := m.calculateCurrentSoilCarbon(req.MonitoringData)
	result.CalculationSteps = append(result.CalculationSteps, CalculationStep{
		StepName:    "Current Soil Carbon",
		Formula:     "CSC = Recent Soil Carbon Measurements",
		InputValues: map[string]interface{}{"soil_samples": len(req.MonitoringData.SoilData)},
		Result:      currentSoilCarbon,
		Unit:        "tC/ha",
		Timestamp:   time.Now(),
	})
	
	// Step 3: Calculate soil carbon change
	soilCarbonChange := currentSoilCarbon - baselineSoilCarbon
	result.CalculationSteps = append(result.CalculationSteps, CalculationStep{
		StepName:    "Soil Carbon Change",
		Formula:     "SCC = CSC - BSC",
		InputValues: map[string]interface{}{"current": currentSoilCarbon, "baseline": baselineSoilCarbon},
		Result:      soilCarbonChange,
		Unit:        "tC/ha/year",
		Timestamp:   time.Now(),
	})
	
	// Step 4: Convert to CO2e
	co2eChange := soilCarbonChange * 3.67 // CO2/C ratio
	result.CalculationSteps = append(result.CalculationSteps, CalculationStep{
		StepName:    "CO2e Sequestration",
		Formula:     "CO2e = SCC × 3.67",
		InputValues: map[string]interface{}{"soil_carbon_change": soilCarbonChange},
		Result:      co2eChange,
		Unit:        "tCO2e/ha/year",
		Timestamp:   time.Now(),
	})
	
	// Step 5: Apply area factor
	projectArea := 100.0 // ha - simplified
	totalSequestration := co2eChange * projectArea
	result.CalculationSteps = append(result.CalculationSteps, CalculationStep{
		StepName:    "Total Sequestration",
		Formula:     "TS = CO2e × Area",
		InputValues: map[string]interface{}{"co2e_per_hectare": co2eChange, "area": projectArea},
		Result:      totalSequestration,
		Unit:        "tCO2e/year",
		Timestamp:   time.Now(),
	})
	
	result.CalculatedTons = math.Max(0, totalSequestration)
	result.DataQualityScore = req.DataQualityScore
	result.ConfidenceLevel = m.calculateConfidenceLevel(req)
	
	return result, nil
}

func (m *VM0033Methodology) calculateBaselineSoilCarbon(data MonitoringData) float64 {
	// Use initial soil measurements as baseline
	if len(data.SoilData) > 0 {
		var totalCarbon float64
		for _, sample := range data.SoilData {
			totalCarbon += sample.SoilCarbon
		}
		return totalCarbon / float64(len(data.SoilData))
	}
	return 20.0 // Default baseline
}

func (m *VM0033Methodology) calculateCurrentSoilCarbon(data MonitoringData) float64 {
	// Use recent soil measurements
	if len(data.SoilData) > 0 {
		var totalCarbon float64
		for _, sample := range data.SoilData {
			totalCarbon += sample.SoilCarbon
		}
		return totalCarbon / float64(len(data.SoilData))
	}
	return 25.0 // Default current
}

func (m *VM0033Methodology) calculateConfidenceLevel(req *CalculationInput) float64 {
	confidence := req.DataQualityScore * 0.85 // Lower confidence for soil carbon
	
	if len(req.MonitoringData.SoilData) >= 5 {
		confidence += 0.1
	}
	
	return math.Min(1.0, confidence)
}
