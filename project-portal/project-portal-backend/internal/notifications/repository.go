package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	awspkg "carbon-scribe/project-portal/project-portal-backend/pkg/aws"
)

// TableNames holds the DynamoDB table names
type TableNames struct {
	Templates   string
	Rules       string
	Preferences string
	Connections string
	DeliveryLogs string
}

// DefaultTableNames returns the default table names
func DefaultTableNames() TableNames {
	return TableNames{
		Templates:    "NotificationTemplates",
		Rules:        "NotificationRules",
		Preferences:  "UserPreferences",
		Connections:  "WebSocketConnections",
		DeliveryLogs: "DeliveryLogs",
	}
}

// Repository handles data access for notifications
type Repository struct {
	db     *awspkg.DynamoDBClient
	tables TableNames
}

// NewRepository creates a new notification repository
func NewRepository(db *awspkg.DynamoDBClient, tables TableNames) *Repository {
	return &Repository{
		db:     db,
		tables: tables,
	}
}

// ================================
// Template Operations
// ================================

// CreateTemplate creates a new notification template
func (r *Repository) CreateTemplate(ctx context.Context, template *NotificationTemplate) error {
	now := time.Now().UTC().Format(time.RFC3339)
	template.CreatedAt = now
	template.UpdatedAt = now

	item := map[string]types.AttributeValue{
		"PK":        &types.AttributeValueMemberS{Value: template.PK},
		"SK":        &types.AttributeValueMemberS{Value: template.SK},
		"name":      &types.AttributeValueMemberS{Value: template.Name},
		"subject":   &types.AttributeValueMemberS{Value: template.Subject},
		"body":      &types.AttributeValueMemberS{Value: template.Body},
		"isActive":  &types.AttributeValueMemberBOOL{Value: template.IsActive},
		"createdAt": &types.AttributeValueMemberS{Value: template.CreatedAt},
		"updatedAt": &types.AttributeValueMemberS{Value: template.UpdatedAt},
	}

	if len(template.Variables) > 0 {
		vars := make([]types.AttributeValue, len(template.Variables))
		for i, v := range template.Variables {
			vars[i] = &types.AttributeValueMemberS{Value: v}
		}
		item["variables"] = &types.AttributeValueMemberL{Value: vars}
	}

	if len(template.Metadata) > 0 {
		meta := make(map[string]types.AttributeValue)
		for k, v := range template.Metadata {
			meta[k] = &types.AttributeValueMemberS{Value: v}
		}
		item["metadata"] = &types.AttributeValueMemberM{Value: meta}
	}

	return r.db.PutItem(ctx, r.tables.Templates, item)
}

// GetTemplate retrieves a template by type, language, and version
func (r *Repository) GetTemplate(ctx context.Context, templateType, language, version string) (*NotificationTemplate, error) {
	pk := fmt.Sprintf("TEMPLATE#%s#%s", templateType, language)
	sk := fmt.Sprintf("VERSION#%s", version)

	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: pk},
		"SK": &types.AttributeValueMemberS{Value: sk},
	}

	item, err := r.db.GetItem(ctx, r.tables.Templates, key)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}

	return r.parseTemplate(item), nil
}

// GetActiveTemplate retrieves the active version of a template
func (r *Repository) GetActiveTemplate(ctx context.Context, templateType, language string) (*NotificationTemplate, error) {
	pk := fmt.Sprintf("TEMPLATE#%s#%s", templateType, language)

	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tables.Templates),
		KeyConditionExpression: aws.String("PK = :pk"),
		FilterExpression:       aws.String("isActive = :active"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     &types.AttributeValueMemberS{Value: pk},
			":active": &types.AttributeValueMemberBOOL{Value: true},
		},
		Limit:            aws.Int32(1),
		ScanIndexForward: aws.Bool(false), // Get latest version
	}

	result, err := r.db.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		return nil, nil
	}

	return r.parseTemplate(result.Items[0]), nil
}

// ListTemplates lists all templates
func (r *Repository) ListTemplates(ctx context.Context, limit int32, cursor string) ([]NotificationTemplate, string, error) {
	input := &dynamodb.ScanInput{
		TableName: aws.String(r.tables.Templates),
		Limit:     aws.Int32(limit),
	}

	if cursor != "" {
		input.ExclusiveStartKey = map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: cursor},
		}
	}

	result, err := r.db.Scan(ctx, input)
	if err != nil {
		return nil, "", err
	}

	templates := make([]NotificationTemplate, 0, len(result.Items))
	for _, item := range result.Items {
		templates = append(templates, *r.parseTemplate(item))
	}

	var nextCursor string
	if result.LastEvaluatedKey != nil {
		if pk, ok := result.LastEvaluatedKey["PK"].(*types.AttributeValueMemberS); ok {
			nextCursor = pk.Value
		}
	}

	return templates, nextCursor, nil
}

func (r *Repository) parseTemplate(item map[string]types.AttributeValue) *NotificationTemplate {
	template := &NotificationTemplate{}

	if v, ok := item["PK"].(*types.AttributeValueMemberS); ok {
		template.PK = v.Value
	}
	if v, ok := item["SK"].(*types.AttributeValueMemberS); ok {
		template.SK = v.Value
	}
	if v, ok := item["name"].(*types.AttributeValueMemberS); ok {
		template.Name = v.Value
	}
	if v, ok := item["subject"].(*types.AttributeValueMemberS); ok {
		template.Subject = v.Value
	}
	if v, ok := item["body"].(*types.AttributeValueMemberS); ok {
		template.Body = v.Value
	}
	if v, ok := item["isActive"].(*types.AttributeValueMemberBOOL); ok {
		template.IsActive = v.Value
	}
	if v, ok := item["createdAt"].(*types.AttributeValueMemberS); ok {
		template.CreatedAt = v.Value
	}
	if v, ok := item["updatedAt"].(*types.AttributeValueMemberS); ok {
		template.UpdatedAt = v.Value
	}
	if v, ok := item["variables"].(*types.AttributeValueMemberL); ok {
		for _, av := range v.Value {
			if s, ok := av.(*types.AttributeValueMemberS); ok {
				template.Variables = append(template.Variables, s.Value)
			}
		}
	}
	if v, ok := item["metadata"].(*types.AttributeValueMemberM); ok {
		template.Metadata = make(map[string]string)
		for k, av := range v.Value {
			if s, ok := av.(*types.AttributeValueMemberS); ok {
				template.Metadata[k] = s.Value
			}
		}
	}

	return template
}

// ================================
// Rule Operations
// ================================

// CreateRule creates a new notification rule
func (r *Repository) CreateRule(ctx context.Context, rule *NotificationRule) error {
	now := time.Now().UTC().Format(time.RFC3339)
	rule.CreatedAt = now
	rule.UpdatedAt = now
	rule.PK = fmt.Sprintf("RULE#%s", rule.ProjectID)
	rule.SK = fmt.Sprintf("RULE#%s", rule.RuleID)

	item := map[string]types.AttributeValue{
		"PK":           &types.AttributeValueMemberS{Value: rule.PK},
		"SK":           &types.AttributeValueMemberS{Value: rule.SK},
		"ruleId":       &types.AttributeValueMemberS{Value: rule.RuleID},
		"projectId":    &types.AttributeValueMemberS{Value: rule.ProjectID},
		"name":         &types.AttributeValueMemberS{Value: rule.Name},
		"description":  &types.AttributeValueMemberS{Value: rule.Description},
		"isActive":     &types.AttributeValueMemberBOOL{Value: rule.IsActive},
		"triggerCount": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", rule.TriggerCount)},
		"createdAt":    &types.AttributeValueMemberS{Value: rule.CreatedAt},
		"updatedAt":    &types.AttributeValueMemberS{Value: rule.UpdatedAt},
	}

	return r.db.PutItem(ctx, r.tables.Rules, item)
}

// GetRule retrieves a rule by project ID and rule ID
func (r *Repository) GetRule(ctx context.Context, projectID, ruleID string) (*NotificationRule, error) {
	pk := fmt.Sprintf("RULE#%s", projectID)
	sk := fmt.Sprintf("RULE#%s", ruleID)

	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: pk},
		"SK": &types.AttributeValueMemberS{Value: sk},
	}

	item, err := r.db.GetItem(ctx, r.tables.Rules, key)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}

	return r.parseRule(item), nil
}

// ListRulesByProject lists all rules for a project
func (r *Repository) ListRulesByProject(ctx context.Context, projectID string) ([]NotificationRule, error) {
	pk := fmt.Sprintf("RULE#%s", projectID)

	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tables.Rules),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
	}

	result, err := r.db.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	rules := make([]NotificationRule, 0, len(result.Items))
	for _, item := range result.Items {
		rules = append(rules, *r.parseRule(item))
	}

	return rules, nil
}

// DeleteRule deletes a rule
func (r *Repository) DeleteRule(ctx context.Context, projectID, ruleID string) error {
	pk := fmt.Sprintf("RULE#%s", projectID)
	sk := fmt.Sprintf("RULE#%s", ruleID)

	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: pk},
		"SK": &types.AttributeValueMemberS{Value: sk},
	}

	return r.db.DeleteItem(ctx, r.tables.Rules, key)
}

func (r *Repository) parseRule(item map[string]types.AttributeValue) *NotificationRule {
	rule := &NotificationRule{}

	if v, ok := item["PK"].(*types.AttributeValueMemberS); ok {
		rule.PK = v.Value
	}
	if v, ok := item["SK"].(*types.AttributeValueMemberS); ok {
		rule.SK = v.Value
	}
	if v, ok := item["ruleId"].(*types.AttributeValueMemberS); ok {
		rule.RuleID = v.Value
	}
	if v, ok := item["projectId"].(*types.AttributeValueMemberS); ok {
		rule.ProjectID = v.Value
	}
	if v, ok := item["name"].(*types.AttributeValueMemberS); ok {
		rule.Name = v.Value
	}
	if v, ok := item["description"].(*types.AttributeValueMemberS); ok {
		rule.Description = v.Value
	}
	if v, ok := item["isActive"].(*types.AttributeValueMemberBOOL); ok {
		rule.IsActive = v.Value
	}
	if v, ok := item["createdAt"].(*types.AttributeValueMemberS); ok {
		rule.CreatedAt = v.Value
	}
	if v, ok := item["updatedAt"].(*types.AttributeValueMemberS); ok {
		rule.UpdatedAt = v.Value
	}

	return rule
}

// ================================
// Preference Operations
// ================================

// SavePreference saves a user preference
func (r *Repository) SavePreference(ctx context.Context, pref *UserPreference) error {
	now := time.Now().UTC().Format(time.RFC3339)
	pref.UpdatedAt = now
	pref.PK = fmt.Sprintf("USER#%s", pref.UserID)
	pref.SK = fmt.Sprintf("PREF#%s#%s", pref.Channel, pref.Category)

	item := map[string]types.AttributeValue{
		"PK":              &types.AttributeValueMemberS{Value: pref.PK},
		"SK":              &types.AttributeValueMemberS{Value: pref.SK},
		"userId":          &types.AttributeValueMemberS{Value: pref.UserID},
		"channel":         &types.AttributeValueMemberS{Value: string(pref.Channel)},
		"category":        &types.AttributeValueMemberS{Value: string(pref.Category)},
		"enabled":         &types.AttributeValueMemberBOOL{Value: pref.Enabled},
		"quietHoursStart": &types.AttributeValueMemberS{Value: pref.QuietHoursStart},
		"quietHoursEnd":   &types.AttributeValueMemberS{Value: pref.QuietHoursEnd},
		"updatedAt":       &types.AttributeValueMemberS{Value: pref.UpdatedAt},
	}

	return r.db.PutItem(ctx, r.tables.Preferences, item)
}

// GetUserPreferences retrieves all preferences for a user
func (r *Repository) GetUserPreferences(ctx context.Context, userID string) ([]UserPreference, error) {
	pk := fmt.Sprintf("USER#%s", userID)

	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tables.Preferences),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
	}

	result, err := r.db.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	prefs := make([]UserPreference, 0, len(result.Items))
	for _, item := range result.Items {
		prefs = append(prefs, *r.parsePreference(item))
	}

	return prefs, nil
}

// GetPreference retrieves a specific preference
func (r *Repository) GetPreference(ctx context.Context, userID string, channel Channel, category NotificationCategory) (*UserPreference, error) {
	pk := fmt.Sprintf("USER#%s", userID)
	sk := fmt.Sprintf("PREF#%s#%s", channel, category)

	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: pk},
		"SK": &types.AttributeValueMemberS{Value: sk},
	}

	item, err := r.db.GetItem(ctx, r.tables.Preferences, key)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}

	return r.parsePreference(item), nil
}

func (r *Repository) parsePreference(item map[string]types.AttributeValue) *UserPreference {
	pref := &UserPreference{}

	if v, ok := item["PK"].(*types.AttributeValueMemberS); ok {
		pref.PK = v.Value
	}
	if v, ok := item["SK"].(*types.AttributeValueMemberS); ok {
		pref.SK = v.Value
	}
	if v, ok := item["userId"].(*types.AttributeValueMemberS); ok {
		pref.UserID = v.Value
	}
	if v, ok := item["channel"].(*types.AttributeValueMemberS); ok {
		pref.Channel = Channel(v.Value)
	}
	if v, ok := item["category"].(*types.AttributeValueMemberS); ok {
		pref.Category = NotificationCategory(v.Value)
	}
	if v, ok := item["enabled"].(*types.AttributeValueMemberBOOL); ok {
		pref.Enabled = v.Value
	}
	if v, ok := item["quietHoursStart"].(*types.AttributeValueMemberS); ok {
		pref.QuietHoursStart = v.Value
	}
	if v, ok := item["quietHoursEnd"].(*types.AttributeValueMemberS); ok {
		pref.QuietHoursEnd = v.Value
	}
	if v, ok := item["updatedAt"].(*types.AttributeValueMemberS); ok {
		pref.UpdatedAt = v.Value
	}

	return pref
}

// ================================
// WebSocket Connection Operations
// ================================

// SaveConnection saves a WebSocket connection
func (r *Repository) SaveConnection(ctx context.Context, conn *WebSocketConnection) error {
	conn.PK = fmt.Sprintf("CONNECTION#%s", conn.ConnectionID)
	// Set TTL to 24 hours from now for auto-cleanup
	conn.TTL = time.Now().Add(24 * time.Hour).Unix()

	item := map[string]types.AttributeValue{
		"PK":           &types.AttributeValueMemberS{Value: conn.PK},
		"connectionId": &types.AttributeValueMemberS{Value: conn.ConnectionID},
		"userId":       &types.AttributeValueMemberS{Value: conn.UserID},
		"connectedAt":  &types.AttributeValueMemberS{Value: conn.ConnectedAt},
		"lastActivity": &types.AttributeValueMemberS{Value: conn.LastActivity},
		"userAgent":    &types.AttributeValueMemberS{Value: conn.UserAgent},
		"ipAddress":    &types.AttributeValueMemberS{Value: conn.IPAddress},
		"ttl":          &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", conn.TTL)},
	}

	if len(conn.ProjectIDs) > 0 {
		pids := make([]types.AttributeValue, len(conn.ProjectIDs))
		for i, pid := range conn.ProjectIDs {
			pids[i] = &types.AttributeValueMemberS{Value: pid}
		}
		item["projectIds"] = &types.AttributeValueMemberL{Value: pids}
	}

	if len(conn.Channels) > 0 {
		channels := make([]types.AttributeValue, len(conn.Channels))
		for i, ch := range conn.Channels {
			channels[i] = &types.AttributeValueMemberS{Value: ch}
		}
		item["channels"] = &types.AttributeValueMemberL{Value: channels}
	}

	return r.db.PutItem(ctx, r.tables.Connections, item)
}

// GetConnection retrieves a connection by ID
func (r *Repository) GetConnection(ctx context.Context, connectionID string) (*WebSocketConnection, error) {
	pk := fmt.Sprintf("CONNECTION#%s", connectionID)

	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: pk},
	}

	item, err := r.db.GetItem(ctx, r.tables.Connections, key)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}

	return r.parseConnection(item), nil
}

// DeleteConnection deletes a connection
func (r *Repository) DeleteConnection(ctx context.Context, connectionID string) error {
	pk := fmt.Sprintf("CONNECTION#%s", connectionID)

	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: pk},
	}

	return r.db.DeleteItem(ctx, r.tables.Connections, key)
}

// GetConnectionsByUser retrieves all connections for a user
func (r *Repository) GetConnectionsByUser(ctx context.Context, userID string) ([]WebSocketConnection, error) {
	input := &dynamodb.ScanInput{
		TableName:        aws.String(r.tables.Connections),
		FilterExpression: aws.String("userId = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
		},
	}

	result, err := r.db.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	conns := make([]WebSocketConnection, 0, len(result.Items))
	for _, item := range result.Items {
		conns = append(conns, *r.parseConnection(item))
	}

	return conns, nil
}

// GetConnectionsByChannel retrieves all connections subscribed to a channel
func (r *Repository) GetConnectionsByChannel(ctx context.Context, channel string) ([]WebSocketConnection, error) {
	input := &dynamodb.ScanInput{
		TableName:        aws.String(r.tables.Connections),
		FilterExpression: aws.String("contains(channels, :channel)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":channel": &types.AttributeValueMemberS{Value: channel},
		},
	}

	result, err := r.db.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	conns := make([]WebSocketConnection, 0, len(result.Items))
	for _, item := range result.Items {
		conns = append(conns, *r.parseConnection(item))
	}

	return conns, nil
}

func (r *Repository) parseConnection(item map[string]types.AttributeValue) *WebSocketConnection {
	conn := &WebSocketConnection{}

	if v, ok := item["PK"].(*types.AttributeValueMemberS); ok {
		conn.PK = v.Value
	}
	if v, ok := item["connectionId"].(*types.AttributeValueMemberS); ok {
		conn.ConnectionID = v.Value
	}
	if v, ok := item["userId"].(*types.AttributeValueMemberS); ok {
		conn.UserID = v.Value
	}
	if v, ok := item["connectedAt"].(*types.AttributeValueMemberS); ok {
		conn.ConnectedAt = v.Value
	}
	if v, ok := item["lastActivity"].(*types.AttributeValueMemberS); ok {
		conn.LastActivity = v.Value
	}
	if v, ok := item["userAgent"].(*types.AttributeValueMemberS); ok {
		conn.UserAgent = v.Value
	}
	if v, ok := item["ipAddress"].(*types.AttributeValueMemberS); ok {
		conn.IPAddress = v.Value
	}
	if v, ok := item["projectIds"].(*types.AttributeValueMemberL); ok {
		for _, av := range v.Value {
			if s, ok := av.(*types.AttributeValueMemberS); ok {
				conn.ProjectIDs = append(conn.ProjectIDs, s.Value)
			}
		}
	}
	if v, ok := item["channels"].(*types.AttributeValueMemberL); ok {
		for _, av := range v.Value {
			if s, ok := av.(*types.AttributeValueMemberS); ok {
				conn.Channels = append(conn.Channels, s.Value)
			}
		}
	}

	return conn
}

// ================================
// Delivery Log Operations
// ================================

// CreateDeliveryLog creates a new delivery log
func (r *Repository) CreateDeliveryLog(ctx context.Context, log *DeliveryLog) error {
	now := time.Now().UTC().Format(time.RFC3339)
	log.CreatedAt = now
	log.PK = fmt.Sprintf("NOTIFICATION#%s", log.NotificationID)
	log.SK = fmt.Sprintf("ATTEMPT#%s", now)

	item := map[string]types.AttributeValue{
		"PK":             &types.AttributeValueMemberS{Value: log.PK},
		"SK":             &types.AttributeValueMemberS{Value: log.SK},
		"notificationId": &types.AttributeValueMemberS{Value: log.NotificationID},
		"userId":         &types.AttributeValueMemberS{Value: log.UserID},
		"channel":        &types.AttributeValueMemberS{Value: string(log.Channel)},
		"templateId":     &types.AttributeValueMemberS{Value: log.TemplateID},
		"subject":        &types.AttributeValueMemberS{Value: log.Subject},
		"status":         &types.AttributeValueMemberS{Value: string(log.Status)},
		"retryCount":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", log.RetryCount)},
		"createdAt":      &types.AttributeValueMemberS{Value: log.CreatedAt},
	}

	if log.ProviderMessageID != "" {
		item["providerMessageId"] = &types.AttributeValueMemberS{Value: log.ProviderMessageID}
	}
	if log.ErrorMessage != "" {
		item["errorMessage"] = &types.AttributeValueMemberS{Value: log.ErrorMessage}
	}

	return r.db.PutItem(ctx, r.tables.DeliveryLogs, item)
}

// GetDeliveryLogs retrieves delivery logs for a notification
func (r *Repository) GetDeliveryLogs(ctx context.Context, notificationID string) ([]DeliveryLog, error) {
	pk := fmt.Sprintf("NOTIFICATION#%s", notificationID)

	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tables.DeliveryLogs),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
		ScanIndexForward: aws.Bool(false), // Latest first
	}

	result, err := r.db.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	logs := make([]DeliveryLog, 0, len(result.Items))
	for _, item := range result.Items {
		logs = append(logs, *r.parseDeliveryLog(item))
	}

	return logs, nil
}

// GetUserDeliveryLogs retrieves delivery logs for a user
func (r *Repository) GetUserDeliveryLogs(ctx context.Context, userID string, limit int32, cursor string) ([]DeliveryLog, string, error) {
	input := &dynamodb.ScanInput{
		TableName:        aws.String(r.tables.DeliveryLogs),
		FilterExpression: aws.String("userId = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
		},
		Limit: aws.Int32(limit),
	}

	result, err := r.db.Scan(ctx, input)
	if err != nil {
		return nil, "", err
	}

	logs := make([]DeliveryLog, 0, len(result.Items))
	for _, item := range result.Items {
		logs = append(logs, *r.parseDeliveryLog(item))
	}

	var nextCursor string
	if result.LastEvaluatedKey != nil {
		if pk, ok := result.LastEvaluatedKey["PK"].(*types.AttributeValueMemberS); ok {
			nextCursor = pk.Value
		}
	}

	return logs, nextCursor, nil
}

// UpdateDeliveryStatus updates the status of a delivery log
func (r *Repository) UpdateDeliveryStatus(ctx context.Context, notificationID, sk string, status DeliveryStatus, providerMessageID string) error {
	pk := fmt.Sprintf("NOTIFICATION#%s", notificationID)

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tables.DeliveryLogs),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression: aws.String("SET #status = :status, providerMessageId = :msgId"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: string(status)},
			":msgId":  &types.AttributeValueMemberS{Value: providerMessageID},
		},
	}

	_, err := r.db.UpdateItem(ctx, input)
	return err
}

func (r *Repository) parseDeliveryLog(item map[string]types.AttributeValue) *DeliveryLog {
	log := &DeliveryLog{}

	if v, ok := item["PK"].(*types.AttributeValueMemberS); ok {
		log.PK = v.Value
	}
	if v, ok := item["SK"].(*types.AttributeValueMemberS); ok {
		log.SK = v.Value
	}
	if v, ok := item["notificationId"].(*types.AttributeValueMemberS); ok {
		log.NotificationID = v.Value
	}
	if v, ok := item["userId"].(*types.AttributeValueMemberS); ok {
		log.UserID = v.Value
	}
	if v, ok := item["channel"].(*types.AttributeValueMemberS); ok {
		log.Channel = Channel(v.Value)
	}
	if v, ok := item["templateId"].(*types.AttributeValueMemberS); ok {
		log.TemplateID = v.Value
	}
	if v, ok := item["subject"].(*types.AttributeValueMemberS); ok {
		log.Subject = v.Value
	}
	if v, ok := item["status"].(*types.AttributeValueMemberS); ok {
		log.Status = DeliveryStatus(v.Value)
	}
	if v, ok := item["providerMessageId"].(*types.AttributeValueMemberS); ok {
		log.ProviderMessageID = v.Value
	}
	if v, ok := item["errorMessage"].(*types.AttributeValueMemberS); ok {
		log.ErrorMessage = v.Value
	}
	if v, ok := item["createdAt"].(*types.AttributeValueMemberS); ok {
		log.CreatedAt = v.Value
	}

	return log
}
