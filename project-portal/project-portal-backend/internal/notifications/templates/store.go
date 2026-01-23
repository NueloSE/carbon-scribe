package templates

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"carbon-scribe/project-portal/project-portal-backend/internal/notifications"
	awspkg "carbon-scribe/project-portal/project-portal-backend/pkg/aws"
)

// Store handles template storage in DynamoDB
type Store struct {
	db        *awspkg.DynamoDBClient
	tableName string
}

// NewStore creates a new template store
func NewStore(db *awspkg.DynamoDBClient, tableName string) *Store {
	return &Store{
		db:        db,
		tableName: tableName,
	}
}

// CreateTemplate creates a new template in DynamoDB
func (s *Store) CreateTemplate(ctx context.Context, template *notifications.NotificationTemplate) error {
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

	return s.db.PutItem(ctx, s.tableName, item)
}

// GetTemplate retrieves a template by PK and SK
func (s *Store) GetTemplate(ctx context.Context, templateType, language, version string) (*notifications.NotificationTemplate, error) {
	pk := fmt.Sprintf("TEMPLATE#%s#%s", templateType, language)
	sk := fmt.Sprintf("VERSION#%s", version)

	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: pk},
		"SK": &types.AttributeValueMemberS{Value: sk},
	}

	item, err := s.db.GetItem(ctx, s.tableName, key)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}

	return s.parseTemplate(item), nil
}

// GetActiveTemplate retrieves the active template for a type and language
func (s *Store) GetActiveTemplate(ctx context.Context, templateType, language string) (*notifications.NotificationTemplate, error) {
	pk := fmt.Sprintf("TEMPLATE#%s#%s", templateType, language)

	input := &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		FilterExpression:       aws.String("isActive = :active"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     &types.AttributeValueMemberS{Value: pk},
			":active": &types.AttributeValueMemberBOOL{Value: true},
		},
		Limit:            aws.Int32(1),
		ScanIndexForward: aws.Bool(false),
	}

	result, err := s.db.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		return nil, nil
	}

	return s.parseTemplate(result.Items[0]), nil
}

// ListTemplates lists all templates with pagination
func (s *Store) ListTemplates(ctx context.Context, limit int32, cursor string) ([]notifications.NotificationTemplate, string, error) {
	input := &dynamodb.ScanInput{
		TableName: aws.String(s.tableName),
		Limit:     aws.Int32(limit),
	}

	if cursor != "" {
		input.ExclusiveStartKey = map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: cursor},
		}
	}

	result, err := s.db.Scan(ctx, input)
	if err != nil {
		return nil, "", err
	}

	templates := make([]notifications.NotificationTemplate, 0, len(result.Items))
	for _, item := range result.Items {
		templates = append(templates, *s.parseTemplate(item))
	}

	var nextCursor string
	if result.LastEvaluatedKey != nil {
		if pk, ok := result.LastEvaluatedKey["PK"].(*types.AttributeValueMemberS); ok {
			nextCursor = pk.Value
		}
	}

	return templates, nextCursor, nil
}

// UpdateTemplate updates an existing template
func (s *Store) UpdateTemplate(ctx context.Context, pk, sk string, updates map[string]interface{}) error {
	now := time.Now().UTC().Format(time.RFC3339)

	var updateExpr []string
	exprNames := make(map[string]string)
	exprValues := make(map[string]types.AttributeValue)

	for key, value := range updates {
		placeholder := fmt.Sprintf("#%s", key)
		valuePlaceholder := fmt.Sprintf(":%s", key)
		updateExpr = append(updateExpr, fmt.Sprintf("%s = %s", placeholder, valuePlaceholder))
		exprNames[placeholder] = key

		switch v := value.(type) {
		case string:
			exprValues[valuePlaceholder] = &types.AttributeValueMemberS{Value: v}
		case bool:
			exprValues[valuePlaceholder] = &types.AttributeValueMemberBOOL{Value: v}
		case []string:
			vals := make([]types.AttributeValue, len(v))
			for i, s := range v {
				vals[i] = &types.AttributeValueMemberS{Value: s}
			}
			exprValues[valuePlaceholder] = &types.AttributeValueMemberL{Value: vals}
		}
	}

	// Always update updatedAt
	updateExpr = append(updateExpr, "#updatedAt = :updatedAt")
	exprNames["#updatedAt"] = "updatedAt"
	exprValues[":updatedAt"] = &types.AttributeValueMemberS{Value: now}

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression:          aws.String("SET " + joinStrings(updateExpr, ", ")),
		ExpressionAttributeNames:  exprNames,
		ExpressionAttributeValues: exprValues,
	}

	_, err := s.db.UpdateItem(ctx, input)
	return err
}

// DeleteTemplate deletes a template
func (s *Store) DeleteTemplate(ctx context.Context, templateType, language, version string) error {
	pk := fmt.Sprintf("TEMPLATE#%s#%s", templateType, language)
	sk := fmt.Sprintf("VERSION#%s", version)

	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: pk},
		"SK": &types.AttributeValueMemberS{Value: sk},
	}

	return s.db.DeleteItem(ctx, s.tableName, key)
}

func (s *Store) parseTemplate(item map[string]types.AttributeValue) *notifications.NotificationTemplate {
	template := &notifications.NotificationTemplate{}

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

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
