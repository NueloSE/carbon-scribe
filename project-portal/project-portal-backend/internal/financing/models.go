package financing

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CreditStatus represents the status of a carbon credit
type CreditStatus string

const (
	CreditStatusCalculated CreditStatus = "calculated"
	CreditStatusVerified  CreditStatus = "verified"
	CreditStatusMinting   CreditStatus = "minting"
	CreditStatusMinted    CreditStatus = "minted"
	CreditStatusRetired   CreditStatus = "retired"
)

// CarbonCredit represents a calculated carbon credit
type CarbonCredit struct {
	ID                   uuid.UUID  `json:"id" db:"id"`
	ProjectID            uuid.UUID  `json:"project_id" db:"project_id"`
	VintageYear          int        `json:"vintage_year" db:"vintage_year"`
	CalculationPeriodStart time.Time `json:"calculation_period_start" db:"calculation_period_start"`
	CalculationPeriodEnd   time.Time `json:"calculation_period_end" db:"calculation_period_end"`

	// Credit details
	MethodologyCode  string  `json:"methodology_code" db:"methodology_code"`
	CalculatedTons   float64 `json:"calculated_tons" db:"calculated_tons"`
	BufferedTons     float64 `json:"buffered_tons" db:"buffered_tons"`
	IssuedTons       *float64 `json:"issued_tons" db:"issued_tons"`
	DataQualityScore *float64 `json:"data_quality_score" db:"data_quality_score"`

	// Stellar integration
	StellarAssetCode      string            `json:"stellar_asset_code" db:"stellar_asset_code"`
	StellarAssetIssuer    string            `json:"stellar_asset_issuer" db:"stellar_asset_issuer"`
	TokenIDs              TokenIDArray      `json:"token_ids" db:"token_ids"`
	MintTransactionHash   *string           `json:"mint_transaction_hash" db:"mint_transaction_hash"`
	MintedAt              *time.Time        `json:"minted_at" db:"minted_at"`

	// Status
	Status        CreditStatus `json:"status" db:"status"`
	VerificationID *uuid.UUID  `json:"verification_id" db:"verification_id"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TokenIDArray represents an array of token IDs for database storage
type TokenIDArray []string

// Value implements driver.Valuer interface
func (t TokenIDArray) Value() (driver.Value, error) {
	return json.Marshal(t)
}

// Scan implements sql.Scanner interface
func (t *TokenIDArray) Scan(value interface{}) error {
	if value == nil {
		*t = TokenIDArray{}
		return nil
	}
	
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, t)
	case string:
		return json.Unmarshal([]byte(v), t)
	}
	return nil
}

// ForwardSaleStatus represents the status of a forward sale agreement
type ForwardSaleStatus string

const (
	ForwardSaleStatusPending   ForwardSaleStatus = "pending"
	ForwardSaleStatusActive    ForwardSaleStatus = "active"
	ForwardSaleStatusCompleted ForwardSaleStatus = "completed"
	ForwardSaleStatusCancelled ForwardSaleStatus = "cancelled"
)

// ForwardSaleAgreement represents a forward sale agreement
type ForwardSaleAgreement struct {
	ID            uuid.UUID          `json:"id" db:"id"`
	ProjectID     uuid.UUID          `json:"project_id" db:"project_id"`
	BuyerID       uuid.UUID          `json:"buyer_id" db:"buyer_id"`
	VintageYear   int                `json:"vintage_year" db:"vintage_year"`

	// Terms
	TonsCommitted  float64            `json:"tons_committed" db:"tons_committed"`
	PricePerTon    float64            `json:"price_per_ton" db:"price_per_ton"`
	Currency       string             `json:"currency" db:"currency"`
	TotalAmount    float64            `json:"total_amount" db:"total_amount"`
	DeliveryDate   time.Time          `json:"delivery_date" db:"delivery_date"`

	// Payment
	DepositPercent      float64            `json:"deposit_percent" db:"deposit_percent"`
	DepositPaid         bool               `json:"deposit_paid" db:"deposit_paid"`
	DepositTransactionID *string           `json:"deposit_transaction_id" db:"deposit_transaction_id"`
	PaymentSchedule     PaymentSchedule    `json:"payment_schedule" db:"payment_schedule"`

	// Legal
	ContractHash       *string     `json:"contract_hash" db:"contract_hash"`
	SignedBySellerAt   *time.Time  `json:"signed_by_seller_at" db:"signed_by_seller_at"`
	SignedByBuyerAt    *time.Time  `json:"signed_by_buyer_at" db:"signed_by_buyer_at"`

	// Status
	Status ForwardSaleStatus `json:"status" db:"status"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// PaymentSchedule represents milestone payment schedule
type PaymentSchedule []PaymentMilestone

// PaymentMilestone represents a single payment milestone
type PaymentMilestone struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	DueDate     time.Time `json:"due_date"`
	Amount      float64   `json:"amount"`
	Percentage  float64   `json:"percentage"`
	Paid        bool      `json:"paid"`
	PaidAt      *time.Time `json:"paid_at"`
}

// DistributionType represents the type of revenue distribution
type DistributionType string

const (
	DistributionTypeCreditSale  DistributionType = "credit_sale"
	DistributionTypeForwardSale DistributionType = "forward_sale"
	DistributionTypeRoyalty     DistributionType = "royalty"
)

// PaymentStatus represents the status of payment processing
type PaymentStatus string

const (
	PaymentStatusPending    PaymentStatus = "pending"
	PaymentStatusProcessing PaymentStatus = "processing"
	PaymentStatusCompleted  PaymentStatus = "completed"
	PaymentStatusFailed     PaymentStatus = "failed"
)

// RevenueDistribution represents revenue distribution to stakeholders
type RevenueDistribution struct {
	ID               uuid.UUID        `json:"id" db:"id"`
	CreditSaleID     uuid.UUID        `json:"credit_sale_id" db:"credit_sale_id"`
	DistributionType DistributionType `json:"distribution_type" db:"distribution_type"`

	// Amounts
	TotalReceived       float64            `json:"total_received" db:"total_received"`
	Currency            string             `json:"currency" db:"currency"`
	PlatformFeePercent  float64            `json:"platform_fee_percent" db:"platform_fee_percent"`
	PlatformFeeAmount   float64            `json:"platform_fee_amount" db:"platform_fee_amount"`
	NetAmount           float64            `json:"net_amount" db:"net_amount"`

	// Distribution splits
	Beneficiaries BeneficiaryArray `json:"beneficiaries" db:"beneficiaries"`

	// Payment execution
	PaymentBatchID   *string      `json:"payment_batch_id" db:"payment_batch_id"`
	PaymentStatus    PaymentStatus `json:"payment_status" db:"payment_status"`
	PaymentProcessedAt *time.Time  `json:"payment_processed_at" db:"payment_processed_at"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// BeneficiaryArray represents an array of beneficiaries for database storage
type BeneficiaryArray []Beneficiary

// Value implements driver.Valuer interface
func (b BeneficiaryArray) Value() (driver.Value, error) {
	return json.Marshal(b)
}

// Scan implements sql.Scanner interface
func (b *BeneficiaryArray) Scan(value interface{}) error {
	if value == nil {
		*b = BeneficiaryArray{}
		return nil
	}
	
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, b)
	case string:
		return json.Unmarshal([]byte(v), b)
	}
	return nil
}

// Beneficiary represents a revenue distribution beneficiary
type Beneficiary struct {
	UserID       uuid.UUID `json:"user_id"`
	Percent      float64   `json:"percent"`
	Amount       float64   `json:"amount"`
	TaxWithheld  float64   `json:"tax_withheld"`
	NetAmount    float64   `json:"net_amount"`
	PaymentStatus PaymentStatus `json:"payment_status"`
}

// PaymentTransaction represents a payment transaction
type PaymentTransaction struct {
	ID        uuid.UUID `json:"id" db:"id"`
	ExternalID *string  `json:"external_id" db:"external_id"`
	UserID    *uuid.UUID `json:"user_id" db:"user_id"`
	ProjectID *uuid.UUID `json:"project_id" db:"project_id"`

	// Payment details
	Amount         float64 `json:"amount" db:"amount"`
	Currency       string  `json:"currency" db:"currency"`
	PaymentMethod  string  `json:"payment_method" db:"payment_method"`
	PaymentProvider string  `json:"payment_provider" db:"payment_provider"`

	// Status
	Status         PaymentStatus `json:"status" db:"status"`
	ProviderStatus ProviderStatus `json:"provider_status" db:"provider_status"`
	FailureReason  *string       `json:"failure_reason" db:"failure_reason"`

	// Blockchain specifics (for Stellar payments)
	StellarTransactionHash *string `json:"stellar_transaction_hash" db:"stellar_transaction_hash"`
	StellarAssetCode       *string `json:"stellar_asset_code" db:"stellar_asset_code"`
	StellarAssetIssuer     *string `json:"stellar_asset_issuer" db:"stellar_asset_issuer"`

	Metadata json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt time.Time      `json:"updated_at" db:"updated_at"`
}

// ProviderStatus represents raw status from payment provider
type ProviderStatus map[string]interface{}

// CreditPricingModel represents pricing model for carbon credits
type CreditPricingModel struct {
	ID                uuid.UUID         `json:"id" db:"id"`
	MethodologyCode   string            `json:"methodology_code" db:"methodology_code"`
	RegionCode        *string           `json:"region_code" db:"region_code"`
	VintageYear       *int              `json:"vintage_year" db:"vintage_year"`

	// Pricing factors
	BasePrice         float64           `json:"base_price" db:"base_price"`
	QualityMultiplier QualityMultiplier `json:"quality_multiplier" db:"quality_multiplier"`
	MarketMultiplier  float64           `json:"market_multiplier" db:"market_multiplier"`

	// Validity
	ValidFrom time.Time  `json:"valid_from" db:"valid_from"`
	ValidUntil *time.Time `json:"valid_until" db:"valid_until"`
	IsActive  bool       `json:"is_active" db:"is_active"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// QualityMultiplier represents quality-based pricing multipliers
type QualityMultiplier map[string]float64

// CalculationRequest represents a credit calculation request
type CalculationRequest struct {
	ProjectID              uuid.UUID `json:"project_id" binding:"required"`
	VintageYear            int       `json:"vintage_year" binding:"required"`
	CalculationPeriodStart time.Time `json:"calculation_period_start" binding:"required"`
	CalculationPeriodEnd   time.Time `json:"calculation_period_end" binding:"required"`
	MethodologyCode        string    `json:"methodology_code" binding:"required"`
	ForceRecalculate       bool      `json:"force_recalculate"`
}

// CalculationResponse represents the response from credit calculation
type CalculationResponse struct {
	CreditID          uuid.UUID `json:"credit_id"`
	CalculatedTons    float64   `json:"calculated_tons"`
	BufferedTons      float64   `json:"buffered_tons"`
	DataQualityScore  float64   `json:"data_quality_score"`
	UncertaintyBuffer float64   `json:"uncertainty_buffer"`
	ValidationResults []string  `json:"validation_results"`
	Warnings          []string  `json:"warnings"`
}

// MintRequest represents a token minting request
type MintRequest struct {
	CreditIDs []uuid.UUID `json:"credit_ids" binding:"required"`
	BatchSize int         `json:"batch_size"`
}

// MintResponse represents the response from token minting
type MintResponse struct {
	BatchID       string    `json:"batch_id"`
	TransactionID string    `json:"transaction_id"`
	Status        string    `json:"status"`
	EstimatedTime int       `json:"estimated_time_seconds"`
	CreatedAt     time.Time `json:"created_at"`
}

// ForwardSaleRequest represents a forward sale creation request
type ForwardSaleRequest struct {
	ProjectID       uuid.UUID `json:"project_id" binding:"required"`
	BuyerID         uuid.UUID `json:"buyer_id" binding:"required"`
	VintageYear     int       `json:"vintage_year" binding:"required"`
	TonsCommitted   float64   `json:"tons_committed" binding:"required,gt=0"`
	PricePerTon     float64   `json:"price_per_ton" binding:"required,gt=0"`
	Currency        string    `json:"currency" binding:"required"`
	DeliveryDate    time.Time `json:"delivery_date" binding:"required"`
	DepositPercent  float64   `json:"deposit_percent" binding:"required,min=0,max=100"`
	PaymentSchedule PaymentSchedule `json:"payment_schedule"`
}

// PricingQuoteRequest represents a pricing quote request
type PricingQuoteRequest struct {
	MethodologyCode string     `json:"methodology_code" binding:"required"`
	RegionCode      string     `json:"region_code"`
	VintageYear     int        `json:"vintage_year"`
	Tons            float64    `json:"tons" binding:"required,gt=0"`
	QualityFactors  QualityMultiplier `json:"quality_factors"`
}

// PricingQuoteResponse represents a pricing quote response
type PricingQuoteResponse struct {
	BasePrice         float64   `json:"base_price"`
	AdjustedPrice     float64   `json:"adjusted_price"`
	TotalPrice        float64   `json:"total_price"`
	Currency          string    `json:"currency"`
	ValidUntil        time.Time `json:"valid_until"`
	PricingFactors    []string  `json:"pricing_factors"`
	MarketConditions  string    `json:"market_conditions"`
}

// PaymentInitiationRequest represents a payment initiation request
type PaymentInitiationRequest struct {
	Amount         float64            `json:"amount" binding:"required,gt=0"`
	Currency       string             `json:"currency" binding:"required"`
	PaymentMethod  string             `json:"payment_method" binding:"required"`
	UserID         uuid.UUID          `json:"user_id" binding:"required"`
	ProjectID      *uuid.UUID         `json:"project_id"`
	PaymentDetails PaymentMethodDetails `json:"payment_details"`
	ReturnURL      string             `json:"return_url"`
	WebhookURL     string             `json:"webhook_url"`
}

// PaymentMethodDetails represents details specific to payment method
type PaymentMethodDetails struct {
	// Credit card details (tokenized)
	CardToken *string `json:"card_token"`
	
	// Bank transfer details
	BankAccount *BankAccountDetails `json:"bank_account"`
	
	// Stellar payment details
	StellarAddress *string `json:"stellar_address"`
	
	// M-Pesa details
	MobileNumber *string `json:"mobile_number"`
}

// BankAccountDetails represents bank account information
type BankAccountDetails struct {
	AccountNumber string `json:"account_number"`
	RoutingNumber string `json:"routing_number"`
	BankName      string `json:"bank_name"`
	AccountType   string `json:"account_type"`
}

// RevenueDistributionRequest represents a revenue distribution request
type RevenueDistributionRequest struct {
	CreditSaleID     uuid.UUID     `json:"credit_sale_id" binding:"required"`
	DistributionType DistributionType `json:"distribution_type" binding:"required"`
	TotalReceived    float64       `json:"total_received" binding:"required,gt=0"`
	Currency         string        `json:"currency" binding:"required"`
	PlatformFeePercent float64     `json:"platform_fee_percent" binding:"required,min=0,max=100"`
	Beneficiaries    []Beneficiary `json:"beneficiaries" binding:"required,min=1"`
}
