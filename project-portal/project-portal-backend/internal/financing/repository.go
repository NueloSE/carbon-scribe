package financing

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// SQLRepository implements the Repository interface using SQL database
type SQLRepository struct {
	db *sqlx.DB
}

// NewSQLRepository creates a new SQL repository
func NewSQLRepository(db *sqlx.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

// Carbon Credit operations

func (r *SQLRepository) GetCarbonCredits(ctx context.Context, creditIDs []uuid.UUID) ([]*CarbonCredit, error) {
	if len(creditIDs) == 0 {
		return []*CarbonCredit{}, nil
	}

	query := `
		SELECT id, project_id, vintage_year, calculation_period_start, calculation_period_end,
			   methodology_code, calculated_tons, buffered_tons, issued_tons, data_quality_score,
			   stellar_asset_code, stellar_asset_issuer, token_ids, mint_transaction_hash, minted_at,
			   status, verification_id, created_at, updated_at
		FROM carbon_credits 
		WHERE id = ANY($1)
	`

	var credits []*CarbonCredit
	err := r.db.SelectContext(ctx, &credits, query, creditIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get carbon credits: %w", err)
	}

	return credits, nil
}

func (r *SQLRepository) CreateCarbonCredit(ctx context.Context, credit *CarbonCredit) error {
	query := `
		INSERT INTO carbon_credits (
			id, project_id, vintage_year, calculation_period_start, calculation_period_end,
			methodology_code, calculated_tons, buffered_tons, issued_tons, data_quality_score,
			stellar_asset_code, stellar_asset_issuer, token_ids, mint_transaction_hash, minted_at,
			status, verification_id, created_at, updated_at
		) VALUES (
			:id, :project_id, :vintage_year, :calculation_period_start, :calculation_period_end,
			:methodology_code, :calculated_tons, :buffered_tons, :issued_tons, :data_quality_score,
			:stellar_asset_code, :stellar_asset_issuer, :token_ids, :mint_transaction_hash, :minted_at,
			:status, :verification_id, :created_at, :updated_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, credit)
	if err != nil {
		return fmt.Errorf("failed to create carbon credit: %w", err)
	}

	return nil
}

func (r *SQLRepository) UpdateCarbonCredit(ctx context.Context, credit *CarbonCredit) error {
	query := `
		UPDATE carbon_credits SET
			project_id = :project_id,
			vintage_year = :vintage_year,
			calculation_period_start = :calculation_period_start,
			calculation_period_end = :calculation_period_end,
			methodology_code = :methodology_code,
			calculated_tons = :calculated_tons,
			buffered_tons = :buffered_tons,
			issued_tons = :issued_tons,
			data_quality_score = :data_quality_score,
			stellar_asset_code = :stellar_asset_code,
			stellar_asset_issuer = :stellar_asset_issuer,
			token_ids = :token_ids,
			mint_transaction_hash = :mint_transaction_hash,
			minted_at = :minted_at,
			status = :status,
			verification_id = :verification_id,
			updated_at = :updated_at
		WHERE id = :id
	`

	_, err := r.db.NamedExecContext(ctx, query, credit)
	if err != nil {
		return fmt.Errorf("failed to update carbon credit: %w", err)
	}

	return nil
}

func (r *SQLRepository) ListCarbonCredits(ctx context.Context, filters *CreditFilters) ([]*CarbonCredit, error) {
	query := `
		SELECT id, project_id, vintage_year, calculation_period_start, calculation_period_end,
			   methodology_code, calculated_tons, buffered_tons, issued_tons, data_quality_score,
			   stellar_asset_code, stellar_asset_issuer, token_ids, mint_transaction_hash, minted_at,
			   status, verification_id, created_at, updated_at
		FROM carbon_credits
		WHERE 1=1
	`

	args := make(map[string]interface{})
	argCount := 0

	if filters.ProjectID != nil {
		argCount++
		query += fmt.Sprintf(" AND project_id = $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.ProjectID
	}

	if len(filters.Status) > 0 {
		argCount++
		query += fmt.Sprintf(" AND status = ANY($%d)", argCount)
		args[fmt.Sprintf("$%d", argCount)] = filters.Status
	}

	if filters.Methodology != nil {
		argCount++
		query += fmt.Sprintf(" AND methodology_code = $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.Methodology
	}

	if filters.VintageYear != nil {
		argCount++
		query += fmt.Sprintf(" AND vintage_year = $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.VintageYear
	}

	if filters.CreatedAfter != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.CreatedAfter
	}

	if filters.CreatedBefore != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.CreatedBefore
	}

	query += " ORDER BY created_at DESC"

	if filters.Limit != nil {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.Limit
	}

	if filters.Offset != nil {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.Offset
	}

	var credits []*CarbonCredit
	rows, err := r.db.NamedQueryContext(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("failed to list carbon credits: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var credit CarbonCredit
		if err := rows.StructScan(&credit); err != nil {
			return nil, fmt.Errorf("failed to scan carbon credit: %w", err)
		}
		credits = append(credits, &credit)
	}

	return credits, nil
}

func (r *SQLRepository) GetProjectCredits(ctx context.Context, projectID uuid.UUID) ([]*CarbonCredit, error) {
	query := `
		SELECT id, project_id, vintage_year, calculation_period_start, calculation_period_end,
			   methodology_code, calculated_tons, buffered_tons, issued_tons, data_quality_score,
			   stellar_asset_code, stellar_asset_issuer, token_ids, mint_transaction_hash, minted_at,
			   status, verification_id, created_at, updated_at
		FROM carbon_credits 
		WHERE project_id = $1
		ORDER BY created_at DESC
	`

	var credits []*CarbonCredit
	err := r.db.SelectContext(ctx, &credits, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project credits: %w", err)
	}

	return credits, nil
}

// Forward Sale operations

func (r *SQLRepository) GetForwardSale(ctx context.Context, saleID uuid.UUID) (*ForwardSaleAgreement, error) {
	query := `
		SELECT id, project_id, buyer_id, vintage_year, tons_committed, price_per_ton, currency,
			   total_amount, delivery_date, deposit_percent, deposit_paid, deposit_transaction_id,
			   payment_schedule, contract_hash, signed_by_seller_at, signed_by_buyer_at,
			   status, created_at, updated_at
		FROM forward_sale_agreements 
		WHERE id = $1
	`

	var sale ForwardSaleAgreement
	err := r.db.GetContext(ctx, &sale, query, saleID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("forward sale not found")
		}
		return nil, fmt.Errorf("failed to get forward sale: %w", err)
	}

	return &sale, nil
}

func (r *SQLRepository) CreateForwardSale(ctx context.Context, sale *ForwardSaleAgreement) error {
	query := `
		INSERT INTO forward_sale_agreements (
			id, project_id, buyer_id, vintage_year, tons_committed, price_per_ton, currency,
			total_amount, delivery_date, deposit_percent, deposit_paid, deposit_transaction_id,
			payment_schedule, contract_hash, signed_by_seller_at, signed_by_buyer_at,
			status, created_at, updated_at
		) VALUES (
			:id, :project_id, :buyer_id, :vintage_year, :tons_committed, :price_per_ton, :currency,
			:total_amount, :delivery_date, :deposit_percent, :deposit_paid, :deposit_transaction_id,
			:payment_schedule, :contract_hash, :signed_by_seller_at, :signed_by_buyer_at,
			:status, :created_at, :updated_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, sale)
	if err != nil {
		return fmt.Errorf("failed to create forward sale: %w", err)
	}

	return nil
}

func (r *SQLRepository) UpdateForwardSale(ctx context.Context, sale *ForwardSaleAgreement) error {
	query := `
		UPDATE forward_sale_agreements SET
			project_id = :project_id,
			buyer_id = :buyer_id,
			vintage_year = :vintage_year,
			tons_committed = :tons_committed,
			price_per_ton = :price_per_ton,
			currency = :currency,
			total_amount = :total_amount,
			delivery_date = :delivery_date,
			deposit_percent = :deposit_percent,
			deposit_paid = :deposit_paid,
			deposit_transaction_id = :deposit_transaction_id,
			payment_schedule = :payment_schedule,
			contract_hash = :contract_hash,
			signed_by_seller_at = :signed_by_seller_at,
			signed_by_buyer_at = :signed_by_buyer_at,
			status = :status,
			updated_at = :updated_at
		WHERE id = :id
	`

	_, err := r.db.NamedExecContext(ctx, query, sale)
	if err != nil {
		return fmt.Errorf("failed to update forward sale: %w", err)
	}

	return nil
}

func (r *SQLRepository) ListForwardSales(ctx context.Context, filters *ForwardSaleFilters) ([]*ForwardSaleAgreement, error) {
	query := `
		SELECT id, project_id, buyer_id, vintage_year, tons_committed, price_per_ton, currency,
			   total_amount, delivery_date, deposit_percent, deposit_paid, deposit_transaction_id,
			   payment_schedule, contract_hash, signed_by_seller_at, signed_by_buyer_at,
			   status, created_at, updated_at
		FROM forward_sale_agreements
		WHERE 1=1
	`

	args := make(map[string]interface{})
	argCount := 0

	if filters.ProjectID != nil {
		argCount++
		query += fmt.Sprintf(" AND project_id = $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.ProjectID
	}

	if filters.BuyerID != nil {
		argCount++
		query += fmt.Sprintf(" AND buyer_id = $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.BuyerID
	}

	if len(filters.Status) > 0 {
		argCount++
		query += fmt.Sprintf(" AND status = ANY($%d)", argCount)
		args[fmt.Sprintf("$%d", argCount)] = filters.Status
	}

	if filters.VintageYear != nil {
		argCount++
		query += fmt.Sprintf(" AND vintage_year = $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.VintageYear
	}

	if filters.CreatedAfter != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.CreatedAfter
	}

	if filters.CreatedBefore != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.CreatedBefore
	}

	query += " ORDER BY created_at DESC"

	if filters.Limit != nil {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.Limit
	}

	if filters.Offset != nil {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.Offset
	}

	var sales []*ForwardSaleAgreement
	rows, err := r.db.NamedQueryContext(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("failed to list forward sales: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var sale ForwardSaleAgreement
		if err := rows.StructScan(&sale); err != nil {
			return nil, fmt.Errorf("failed to scan forward sale: %w", err)
		}
		sales = append(sales, &sale)
	}

	return sales, nil
}

// Revenue Distribution operations

func (r *SQLRepository) GetRevenueDistribution(ctx context.Context, distributionID uuid.UUID) (*RevenueDistribution, error) {
	query := `
		SELECT id, credit_sale_id, distribution_type, total_received, currency,
			   platform_fee_percent, platform_fee_amount, net_amount, beneficiaries,
			   payment_batch_id, payment_status, payment_processed_at, created_at
		FROM revenue_distributions 
		WHERE id = $1
	`

	var distribution RevenueDistribution
	err := r.db.GetContext(ctx, &distribution, query, distributionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("revenue distribution not found")
		}
		return nil, fmt.Errorf("failed to get revenue distribution: %w", err)
	}

	return &distribution, nil
}

func (r *SQLRepository) CreateRevenueDistribution(ctx context.Context, distribution *RevenueDistribution) error {
	query := `
		INSERT INTO revenue_distributions (
			id, credit_sale_id, distribution_type, total_received, currency,
			platform_fee_percent, platform_fee_amount, net_amount, beneficiaries,
			payment_batch_id, payment_status, payment_processed_at, created_at
		) VALUES (
			:id, :credit_sale_id, :distribution_type, :total_received, :currency,
			:platform_fee_percent, :platform_fee_amount, :net_amount, :beneficiaries,
			:payment_batch_id, :payment_status, :payment_processed_at, :created_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, distribution)
	if err != nil {
		return fmt.Errorf("failed to create revenue distribution: %w", err)
	}

	return nil
}

func (r *SQLRepository) UpdateRevenueDistribution(ctx context.Context, distribution *RevenueDistribution) error {
	query := `
		UPDATE revenue_distributions SET
			credit_sale_id = :credit_sale_id,
			distribution_type = :distribution_type,
			total_received = :total_received,
			currency = :currency,
			platform_fee_percent = :platform_fee_percent,
			platform_fee_amount = :platform_fee_amount,
			net_amount = :net_amount,
			beneficiaries = :beneficiaries,
			payment_batch_id = :payment_batch_id,
			payment_status = :payment_status,
			payment_processed_at = :payment_processed_at
		WHERE id = :id
	`

	_, err := r.db.NamedExecContext(ctx, query, distribution)
	if err != nil {
		return fmt.Errorf("failed to update revenue distribution: %w", err)
	}

	return nil
}

func (r *SQLRepository) ListRevenueDistributions(ctx context.Context, filters *DistributionFilters) ([]*RevenueDistribution, error) {
	query := `
		SELECT id, credit_sale_id, distribution_type, total_received, currency,
			   platform_fee_percent, platform_fee_amount, net_amount, beneficiaries,
			   payment_batch_id, payment_status, payment_processed_at, created_at
		FROM revenue_distributions
		WHERE 1=1
	`

	args := make(map[string]interface{})
	argCount := 0

	if filters.ProjectID != nil {
		// This would require joining with credit sales table
		// For now, skip project filtering
	}

	if filters.UserID != nil {
		// This would require searching within beneficiaries JSON
		// For now, skip user filtering
	}

	if filters.DistributionType != nil {
		argCount++
		query += fmt.Sprintf(" AND distribution_type = $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.DistributionType
	}

	if filters.Status != nil {
		argCount++
		query += fmt.Sprintf(" AND payment_status = $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.Status
	}

	if filters.CreatedAfter != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.CreatedAfter
	}

	if filters.CreatedBefore != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.CreatedBefore
	}

	query += " ORDER BY created_at DESC"

	if filters.Limit != nil {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.Limit
	}

	if filters.Offset != nil {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args[fmt.Sprintf("$%d", argCount)] = *filters.Offset
	}

	var distributions []*RevenueDistribution
	rows, err := r.db.NamedQueryContext(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("failed to list revenue distributions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var distribution RevenueDistribution
		if err := rows.StructScan(&distribution); err != nil {
			return nil, fmt.Errorf("failed to scan revenue distribution: %w", err)
		}
		distributions = append(distributions, &distribution)
	}

	return distributions, nil
}

// Payment Transaction operations

func (r *SQLRepository) GetPaymentTransaction(ctx context.Context, transactionID uuid.UUID) (*PaymentTransaction, error) {
	query := `
		SELECT id, external_id, user_id, project_id, amount, currency, payment_method, payment_provider,
			   status, provider_status, failure_reason, stellar_transaction_hash, stellar_asset_code,
			  	stellar_asset_issuer, metadata, created_at, updated_at
		FROM payment_transactions 
		WHERE id = $1
	`

	var transaction PaymentTransaction
	err := r.db.GetContext(ctx, &transaction, query, transactionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("payment transaction not found")
		}
		return nil, fmt.Errorf("failed to get payment transaction: %w", err)
	}

	return &transaction, nil
}

func (r *SQLRepository) CreatePaymentTransaction(ctx context.Context, transaction *PaymentTransaction) error {
	query := `
		INSERT INTO payment_transactions (
			id, external_id, user_id, project_id, amount, currency, payment_method, payment_provider,
			status, provider_status, failure_reason, stellar_transaction_hash, stellar_asset_code,
			stellar_asset_issuer, metadata, created_at, updated_at
		) VALUES (
			:id, :external_id, :user_id, :project_id, :amount, :currency, :payment_method, :payment_provider,
			:status, :provider_status, :failure_reason, :stellar_transaction_hash, :stellar_asset_code,
			:stellar_asset_issuer, :metadata, :created_at, :updated_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, transaction)
	if err != nil {
		return fmt.Errorf("failed to create payment transaction: %w", err)
	}

	return nil
}

func (r *SQLRepository) UpdatePaymentTransaction(ctx context.Context, transaction *PaymentTransaction) error {
	query := `
		UPDATE payment_transactions SET
			external_id = :external_id,
			user_id = :user_id,
			project_id = :project_id,
			amount = :amount,
			currency = :currency,
			payment_method = :payment_method,
			payment_provider = :payment_provider,
			status = :status,
			provider_status = :provider_status,
			failure_reason = :failure_reason,
			stellar_transaction_hash = :stellar_transaction_hash,
			stellar_asset_code = :stellar_asset_code,
			stellar_asset_issuer = :stellar_asset_issuer,
			metadata = :metadata,
			updated_at = :updated_at
		WHERE id = :id
	`

	_, err := r.db.NamedExecContext(ctx, query, transaction)
	if err != nil {
		return fmt.Errorf("failed to update payment transaction: %w", err)
	}

	return nil
}

// Project and User operations (mock implementations)

func (r *SQLRepository) GetProject(ctx context.Context, projectID uuid.UUID) (*Project, error) {
	// Mock implementation - in real code, this would query projects table
	return &Project{
		ID:          projectID,
		Name:        "Sample Carbon Project",
		Description: "A sample carbon credit project",
		Region:      "East Africa",
		Country:     "Kenya",
		Methodology: "VM0007",
		Status:      "active",
		CreatedAt:   time.Now().AddDate(-1, 0, 0),
		UpdatedAt:   time.Now(),
	}, nil
}

func (r *SQLRepository) GetUser(ctx context.Context, userID uuid.UUID) (*User, error) {
	// Mock implementation - in real code, this would query users table
	return &User{
		ID:        userID,
		Email:     "user@example.com",
		Name:      "Sample User",
		Role:      "farmer",
		Country:   "Kenya",
		CreatedAt: time.Now().AddDate(-1, 0, 0),
		UpdatedAt: time.Now(),
	}, nil
}
