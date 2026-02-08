package dynamodb

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/dreschagin/monitoring-dashboard/internal/application/port"
)

const (
	defaultListLimit  = 24
	maxListLimit      = 100
	maxBatchWriteSize = 25
	maxBatchRetries   = 5

	screenshotMetadataGSI1 = "GSI1"

	attrPK           = "PK"
	attrSK           = "SK"
	attrGSI1PK       = "GSI1PK"
	attrGSI1SK       = "GSI1SK"
	attrDashboardID  = "dashboard_id"
	attrArtifactType = "artifact_type"
	attrS3Key        = "s3_key"
	attrURL          = "url"
	attrContentType  = "content_type"
	attrSizeBytes    = "size_bytes"
	attrCapturedAt   = "captured_at"
	attrCreatedAt    = "created_at"
	attrExpiresAt    = "expires_at"
)

var dashboardIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

type Config struct {
	TableName       string
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	StrongReads     bool
}

type ScreenshotMetadataRepository struct {
	client      *dynamodb.Client
	tableName   string
	strongReads bool
}

type cursorMode string

const (
	cursorModeDashboard cursorMode = "dashboard"
	cursorModeType      cursorMode = "type"
)

type cursorPayload struct {
	Mode         cursorMode             `json:"mode"`
	DashboardID  string                 `json:"dashboard_id"`
	ArtifactType string                 `json:"artifact_type,omitempty"`
	FromMS       int64                  `json:"from_ms,omitempty"`
	ToMS         int64                  `json:"to_ms,omitempty"`
	Key          map[string]cursorValue `json:"key"`
}

type cursorValue struct {
	S string `json:"s,omitempty"`
	N string `json:"n,omitempty"`
}

func NewScreenshotMetadataRepository(ctx context.Context, cfg Config) (*ScreenshotMetadataRepository, error) {
	if strings.TrimSpace(cfg.TableName) == "" {
		return nil, fmt.Errorf("dynamodb table name is required")
	}

	if strings.TrimSpace(cfg.Region) == "" {
		cfg.Region = "us-east-1"
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	accessKeyID := strings.TrimSpace(cfg.AccessKeyID)
	secretAccessKey := strings.TrimSpace(cfg.SecretAccessKey)
	if accessKeyID != "" || secretAccessKey != "" {
		if accessKeyID == "" || secretAccessKey == "" {
			return nil, fmt.Errorf("both dynamodb access key id and secret access key are required for static credentials")
		}
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretAccessKey,
			"",
		)))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create aws config for dynamodb: %w", err)
	}

	client := dynamodb.NewFromConfig(awsCfg, func(options *dynamodb.Options) {
		if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
			options.BaseEndpoint = &endpoint
		}
	})

	return &ScreenshotMetadataRepository{
		client:      client,
		tableName:   strings.TrimSpace(cfg.TableName),
		strongReads: cfg.StrongReads,
	}, nil
}

func (r *ScreenshotMetadataRepository) PutBatch(ctx context.Context, records []port.ScreenshotMetadata) error {
	if len(records) == 0 {
		return nil
	}

	for start := 0; start < len(records); start += maxBatchWriteSize {
		end := start + maxBatchWriteSize
		if end > len(records) {
			end = len(records)
		}

		requests := make([]types.WriteRequest, 0, end-start)
		for _, record := range records[start:end] {
			item, err := r.toItem(record)
			if err != nil {
				return err
			}
			requests = append(requests, types.WriteRequest{
				PutRequest: &types.PutRequest{Item: item},
			})
		}

		if err := r.writeBatchWithRetry(ctx, requests); err != nil {
			return err
		}
	}

	return nil
}

func (r *ScreenshotMetadataRepository) ListByDashboard(
	ctx context.Context,
	query port.ScreenshotListQuery,
) (port.ScreenshotListPage, error) {
	dashboardID := strings.TrimSpace(query.DashboardID)
	if !dashboardIDPattern.MatchString(dashboardID) {
		return port.ScreenshotListPage{}, fmt.Errorf("invalid dashboard_id")
	}

	limit := query.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	artifactType := strings.TrimSpace(query.ArtifactType)
	fromMS, toMS, hasRange, err := normalizeTimeRange(query.From, query.To)
	if err != nil {
		return port.ScreenshotListPage{}, err
	}

	mode := cursorModeDashboard
	if artifactType != "" {
		mode = cursorModeType
	}

	input := &dynamodb.QueryInput{
		TableName:                 &r.tableName,
		Limit:                     int32Pointer(int32(limit)),
		ScanIndexForward:          boolPointer(false),
		ConsistentRead:            boolPointer(r.strongReads),
		ExpressionAttributeNames:  map[string]string{},
		ExpressionAttributeValues: map[string]types.AttributeValue{},
	}

	if mode == cursorModeDashboard {
		pk := buildPK(dashboardID)
		input.ExpressionAttributeNames["#pk"] = attrPK
		input.ExpressionAttributeValues[":pk"] = &types.AttributeValueMemberS{Value: pk}
		keyCondition := "#pk = :pk"
		if hasRange {
			input.ExpressionAttributeNames["#sk"] = attrSK
			input.ExpressionAttributeValues[":from"] = &types.AttributeValueMemberS{Value: buildSortLowerBound(fromMS)}
			input.ExpressionAttributeValues[":to"] = &types.AttributeValueMemberS{Value: buildSortUpperBound(toMS)}
			keyCondition += " AND #sk BETWEEN :from AND :to"
		}
		input.KeyConditionExpression = &keyCondition
	} else {
		gsiPK := buildGSI1PK(dashboardID, artifactType)
		input.IndexName = stringPointer(screenshotMetadataGSI1)
		input.ConsistentRead = nil
		input.ExpressionAttributeNames["#gsi1pk"] = attrGSI1PK
		input.ExpressionAttributeValues[":pk"] = &types.AttributeValueMemberS{Value: gsiPK}
		keyCondition := "#gsi1pk = :pk"
		if hasRange {
			input.ExpressionAttributeNames["#gsi1sk"] = attrGSI1SK
			input.ExpressionAttributeValues[":from"] = &types.AttributeValueMemberS{Value: buildGSISortLowerBound(fromMS)}
			input.ExpressionAttributeValues[":to"] = &types.AttributeValueMemberS{Value: buildGSISortUpperBound(toMS)}
			keyCondition += " AND #gsi1sk BETWEEN :from AND :to"
		}
		input.KeyConditionExpression = &keyCondition
	}

	if strings.TrimSpace(query.Cursor) != "" {
		exclusiveStartKey, err := decodeCursor(query.Cursor, mode, dashboardID, artifactType, fromMS, toMS)
		if err != nil {
			return port.ScreenshotListPage{}, err
		}
		input.ExclusiveStartKey = exclusiveStartKey
	}

	output, err := r.client.Query(ctx, input)
	if err != nil {
		return port.ScreenshotListPage{}, fmt.Errorf("dynamodb query failed: %w", err)
	}

	items := make([]port.ScreenshotMetadata, 0, len(output.Items))
	for _, raw := range output.Items {
		item, err := fromItem(raw)
		if err != nil {
			return port.ScreenshotListPage{}, err
		}
		items = append(items, item)
	}

	nextCursor := ""
	if len(output.LastEvaluatedKey) > 0 {
		nextCursor, err = encodeCursor(output.LastEvaluatedKey, mode, dashboardID, artifactType, fromMS, toMS)
		if err != nil {
			return port.ScreenshotListPage{}, err
		}
	}

	return port.ScreenshotListPage{
		Items:      items,
		NextCursor: nextCursor,
	}, nil
}

func (r *ScreenshotMetadataRepository) writeBatchWithRetry(ctx context.Context, requests []types.WriteRequest) error {
	if len(requests) == 0 {
		return nil
	}

	pending := map[string][]types.WriteRequest{
		r.tableName: requests,
	}

	for attempt := 0; attempt < maxBatchRetries; attempt++ {
		output, err := r.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: pending,
		})
		if err != nil {
			return fmt.Errorf("dynamodb batch write failed: %w", err)
		}

		if len(output.UnprocessedItems) == 0 {
			return nil
		}

		pending = output.UnprocessedItems
		time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
	}

	return fmt.Errorf("dynamodb batch write has unprocessed items after retries")
}

func (r *ScreenshotMetadataRepository) toItem(record port.ScreenshotMetadata) (map[string]types.AttributeValue, error) {
	dashboardID := strings.TrimSpace(record.DashboardID)
	artifactType := strings.TrimSpace(record.ArtifactType)
	s3Key := strings.TrimSpace(record.S3Key)
	if !dashboardIDPattern.MatchString(dashboardID) {
		return nil, fmt.Errorf("invalid dashboard_id")
	}
	if artifactType == "" {
		return nil, fmt.Errorf("artifact_type is required")
	}
	if s3Key == "" {
		return nil, fmt.Errorf("s3_key is required")
	}

	capturedAt := record.CapturedAt.UTC()
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}

	lastModified := record.LastModified.UTC()
	if lastModified.IsZero() {
		lastModified = capturedAt
	}

	capturedAtMS := capturedAt.UnixMilli()
	lastModifiedMS := lastModified.UnixMilli()

	item := map[string]types.AttributeValue{
		attrPK:           &types.AttributeValueMemberS{Value: buildPK(dashboardID)},
		attrSK:           &types.AttributeValueMemberS{Value: buildSK(capturedAtMS, artifactType, s3Key)},
		attrGSI1PK:       &types.AttributeValueMemberS{Value: buildGSI1PK(dashboardID, artifactType)},
		attrGSI1SK:       &types.AttributeValueMemberS{Value: buildGSI1SK(capturedAtMS, s3Key)},
		attrDashboardID:  &types.AttributeValueMemberS{Value: dashboardID},
		attrArtifactType: &types.AttributeValueMemberS{Value: artifactType},
		attrS3Key:        &types.AttributeValueMemberS{Value: s3Key},
		attrCapturedAt:   &types.AttributeValueMemberN{Value: strconv.FormatInt(capturedAtMS, 10)},
		attrCreatedAt:    &types.AttributeValueMemberN{Value: strconv.FormatInt(lastModifiedMS, 10)},
	}

	if url := strings.TrimSpace(record.URL); url != "" {
		item[attrURL] = &types.AttributeValueMemberS{Value: url}
	}
	if contentType := strings.TrimSpace(record.ContentType); contentType != "" {
		item[attrContentType] = &types.AttributeValueMemberS{Value: contentType}
	}
	if record.SizeBytes > 0 {
		item[attrSizeBytes] = &types.AttributeValueMemberN{Value: strconv.FormatInt(record.SizeBytes, 10)}
	}
	if !record.ExpiresAt.IsZero() {
		item[attrExpiresAt] = &types.AttributeValueMemberN{Value: strconv.FormatInt(record.ExpiresAt.UTC().Unix(), 10)}
	}

	return item, nil
}

func fromItem(item map[string]types.AttributeValue) (port.ScreenshotMetadata, error) {
	dashboardID, err := attrString(item, attrDashboardID)
	if err != nil {
		return port.ScreenshotMetadata{}, err
	}
	artifactType, err := attrString(item, attrArtifactType)
	if err != nil {
		return port.ScreenshotMetadata{}, err
	}
	s3Key, err := attrString(item, attrS3Key)
	if err != nil {
		return port.ScreenshotMetadata{}, err
	}

	capturedAtMS, err := attrInt64(item, attrCapturedAt)
	if err != nil {
		return port.ScreenshotMetadata{}, err
	}
	createdAtMS, err := attrInt64(item, attrCreatedAt)
	if err != nil {
		return port.ScreenshotMetadata{}, err
	}

	record := port.ScreenshotMetadata{
		DashboardID:  dashboardID,
		ArtifactType: artifactType,
		S3Key:        s3Key,
		URL:          optionalString(item, attrURL),
		ContentType:  optionalString(item, attrContentType),
		SizeBytes:    optionalInt64(item, attrSizeBytes),
		CapturedAt:   time.UnixMilli(capturedAtMS).UTC(),
		LastModified: time.UnixMilli(createdAtMS).UTC(),
	}

	expiresAtSeconds := optionalInt64(item, attrExpiresAt)
	if expiresAtSeconds > 0 {
		record.ExpiresAt = time.Unix(expiresAtSeconds, 0).UTC()
	}

	return record, nil
}

func normalizeTimeRange(from, to time.Time) (int64, int64, bool, error) {
	from = from.UTC()
	to = to.UTC()
	if from.IsZero() && to.IsZero() {
		return 0, math.MaxInt64, false, nil
	}

	fromMS := int64(0)
	toMS := int64(math.MaxInt64)
	if !from.IsZero() {
		fromMS = from.UnixMilli()
	}
	if !to.IsZero() {
		toMS = to.UnixMilli()
	}

	if fromMS > toMS {
		return 0, 0, false, fmt.Errorf("from must be less than or equal to to")
	}

	return fromMS, toMS, true, nil
}

func buildPK(dashboardID string) string {
	return "DASHBOARD#" + dashboardID
}

func buildSK(capturedAtMS int64, artifactType, s3Key string) string {
	return fmt.Sprintf("TS#%013d#TYPE#%s#KEY#%s", capturedAtMS, artifactType, objectHash(s3Key))
}

func buildGSI1PK(dashboardID, artifactType string) string {
	return fmt.Sprintf("DASHBOARD#%s#TYPE#%s", dashboardID, artifactType)
}

func buildGSI1SK(capturedAtMS int64, s3Key string) string {
	return fmt.Sprintf("TS#%013d#KEY#%s", capturedAtMS, objectHash(s3Key))
}

func buildSortLowerBound(tsMS int64) string {
	return fmt.Sprintf("TS#%013d#", tsMS)
}

func buildSortUpperBound(tsMS int64) string {
	return fmt.Sprintf("TS#%013d#~", tsMS)
}

func buildGSISortLowerBound(tsMS int64) string {
	return fmt.Sprintf("TS#%013d#", tsMS)
}

func buildGSISortUpperBound(tsMS int64) string {
	return fmt.Sprintf("TS#%013d#~", tsMS)
}

func objectHash(key string) string {
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:8])
}

func encodeCursor(
	key map[string]types.AttributeValue,
	mode cursorMode,
	dashboardID, artifactType string,
	fromMS, toMS int64,
) (string, error) {
	values := make(map[string]cursorValue, len(key))
	for attributeName, raw := range key {
		switch value := raw.(type) {
		case *types.AttributeValueMemberS:
			values[attributeName] = cursorValue{S: value.Value}
		case *types.AttributeValueMemberN:
			values[attributeName] = cursorValue{N: value.Value}
		default:
			return "", fmt.Errorf("unsupported cursor attribute type for %s", attributeName)
		}
	}

	payload := cursorPayload{
		Mode:         mode,
		DashboardID:  dashboardID,
		ArtifactType: artifactType,
		FromMS:       fromMS,
		ToMS:         toMS,
		Key:          values,
	}

	serialized, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cursor: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(serialized), nil
}

func decodeCursor(
	cursor string,
	mode cursorMode,
	dashboardID, artifactType string,
	fromMS, toMS int64,
) (map[string]types.AttributeValue, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}

	var payload cursorPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}

	if payload.Mode != mode ||
		payload.DashboardID != dashboardID ||
		payload.ArtifactType != artifactType ||
		payload.FromMS != fromMS ||
		payload.ToMS != toMS {
		return nil, fmt.Errorf("cursor does not match query filters")
	}

	key := make(map[string]types.AttributeValue, len(payload.Key))
	for attributeName, value := range payload.Key {
		if value.S != "" {
			key[attributeName] = &types.AttributeValueMemberS{Value: value.S}
			continue
		}
		if value.N != "" {
			key[attributeName] = &types.AttributeValueMemberN{Value: value.N}
			continue
		}
		return nil, fmt.Errorf("invalid cursor")
	}

	return key, nil
}

func attrString(item map[string]types.AttributeValue, name string) (string, error) {
	raw, ok := item[name]
	if !ok {
		return "", fmt.Errorf("missing attribute %s", name)
	}
	value, ok := raw.(*types.AttributeValueMemberS)
	if !ok || strings.TrimSpace(value.Value) == "" {
		return "", fmt.Errorf("invalid attribute %s", name)
	}
	return value.Value, nil
}

func optionalString(item map[string]types.AttributeValue, name string) string {
	raw, ok := item[name]
	if !ok {
		return ""
	}
	value, ok := raw.(*types.AttributeValueMemberS)
	if !ok {
		return ""
	}
	return value.Value
}

func attrInt64(item map[string]types.AttributeValue, name string) (int64, error) {
	raw, ok := item[name]
	if !ok {
		return 0, fmt.Errorf("missing attribute %s", name)
	}
	value, ok := raw.(*types.AttributeValueMemberN)
	if !ok {
		return 0, fmt.Errorf("invalid attribute %s", name)
	}
	parsed, err := strconv.ParseInt(value.Value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid attribute %s: %w", name, err)
	}
	return parsed, nil
}

func optionalInt64(item map[string]types.AttributeValue, name string) int64 {
	raw, ok := item[name]
	if !ok {
		return 0
	}
	value, ok := raw.(*types.AttributeValueMemberN)
	if !ok {
		return 0
	}
	parsed, err := strconv.ParseInt(value.Value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func boolPointer(v bool) *bool {
	return &v
}

func int32Pointer(v int32) *int32 {
	return &v
}

func stringPointer(v string) *string {
	return &v
}
