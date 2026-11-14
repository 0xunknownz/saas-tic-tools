// Package reportarchive implements the tenant-scoped detection report archive:
// uploads, cloud OCR, declarative template rules, structured details, review, versions and search.
package reportarchive

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/0xunknownz/saas.starter/foundation/audit"
	"github.com/0xunknownz/saas.starter/foundation/db"
	"github.com/0xunknownz/saas.starter/foundation/identity"
	"github.com/0xunknownz/saas.starter/foundation/tenant"
)

// Config is the Product plugin's deployment configuration. Credentials are read from the process
// environment by the composition root and never exposed to clients.
type Config struct {
	OCRProvider       string `json:"ocrProvider"`
	OCREndpoint       string `json:"ocrEndpoint"`
	OCRAPIKey         string `json:"ocrAPIKey"`
	OCRModel          string `json:"ocrModel"`
	OCRTimeoutSeconds int    `json:"ocrTimeoutSeconds"`
	OCRMaxConcurrency int    `json:"ocrMaxConcurrency"`
	PollMilliseconds  int    `json:"pollMilliseconds"`
	RetryBaseSeconds  int    `json:"retryBaseSeconds"`
	MaxUploadBytes    int64  `json:"maxUploadBytes"`
	MaxUploadFiles    int    `json:"maxUploadFiles"`
	MaxPDFPages       int    `json:"maxPDFPages"`
}

func (c Config) withDefaults() Config {
	if c.OCRProvider == "" {
		c.OCRProvider = "disabled"
	}
	if c.OCREndpoint == "" {
		c.OCREndpoint = "https://api.ocr.space/parse/image"
	}
	if c.OCRModel == "" {
		c.OCRModel = "ocrspace-engine2"
	}
	if c.OCRTimeoutSeconds <= 0 {
		c.OCRTimeoutSeconds = 90
	}
	if c.OCRMaxConcurrency <= 0 {
		c.OCRMaxConcurrency = 2
	}
	if c.PollMilliseconds <= 0 {
		c.PollMilliseconds = 1000
	}
	if c.RetryBaseSeconds <= 0 {
		c.RetryBaseSeconds = 5
	}
	if c.MaxUploadBytes <= 0 {
		c.MaxUploadBytes = 50_000_000
	}
	if c.MaxUploadFiles <= 0 || c.MaxUploadFiles > 50 {
		c.MaxUploadFiles = 50
	}
	if c.MaxPDFPages <= 0 {
		c.MaxPDFPages = 100
	}
	return c
}

// Deps are the Foundation services the domain needs. HTTP-only services are in HTTPDeps.
type Deps struct {
	Db    db.Transactor
	Audit audit.Writer
}

// HTTPDeps are the session guards and membership lookup. Every report route is mounted below
// Identity.RequireTenant, and every request re-reads the membership so a removed member loses
// access even while an old cookie still exists.
type HTTPDeps struct {
	Identity identity.PlatformGuard
	Tenant   tenant.Membership
}

// Report is one archive record. DetectionResult and ArchiveStatus are deliberately separate: a
// 不合格 detection conclusion is still a normally archived report.
type Report struct {
	ID              string                 `json:"id"`
	File            *FileInfo              `json:"file,omitempty"`
	ReportNumber    string                 `json:"reportNumber"`
	ReportName      string                 `json:"reportName"`
	ProductName     string                 `json:"productName"`
	Institution     string                 `json:"institution"`
	ClientUnit      string                 `json:"clientUnit"`
	InspectedUnit   string                 `json:"inspectedUnit"`
	Manufacturer    string                 `json:"manufacturer"`
	Model           string                 `json:"model"`
	Category        string                 `json:"category"`
	Basis           string                 `json:"basis"`
	ReportDate      string                 `json:"reportDate,omitempty"`
	Conclusion      string                 `json:"conclusion"`
	DetectionResult string                 `json:"detectionResult"`
	ArchiveStatus   string                 `json:"archiveStatus"`
	ReviewStage     string                 `json:"reviewStage,omitempty"`
	ReviewReason    string                 `json:"reviewReason,omitempty"`
	ArchiveReason   string                 `json:"archiveReason,omitempty"`
	ReportVersion   int                    `json:"reportVersion"`
	Revision        int                    `json:"revision"`
	RevisionOf      string                 `json:"revisionOf,omitempty"`
	DuplicateOf     string                 `json:"duplicateOf,omitempty"`
	IsCurrent       bool                   `json:"isCurrent"`
	RuleID          string                 `json:"ruleId,omitempty"`
	RuleVersion     int                    `json:"ruleVersion,omitempty"`
	ParseRunID      string                 `json:"-"`
	FieldSources    map[string]FieldSource `json:"fieldSources,omitempty"`
	RawFields       map[string]any         `json:"rawFields,omitempty"`
	CreatedBy       string                 `json:"createdBy"`
	CreatedAt       time.Time              `json:"createdAt"`
	UpdatedAt       time.Time              `json:"updatedAt"`
	ArchivedAt      *time.Time             `json:"archivedAt,omitempty"`
	VoidedAt        *time.Time             `json:"voidedAt,omitempty"`
	VoidReason      string                 `json:"voidReason,omitempty"`
	Details         []Detail               `json:"details,omitempty"`
	OCR             *OCRRun                `json:"ocr,omitempty"`
	LatestParse     *ParseRun              `json:"latestParse,omitempty"`
	Events          []Event                `json:"events,omitempty"`
	Versions        []ReportSummary        `json:"versions,omitempty"`
}

type FieldSource struct {
	Raw  string `json:"raw"`
	Page int    `json:"page"`
	Line int    `json:"line"`
	Rule string `json:"rule,omitempty"`
}

type FileInfo struct {
	ID          string    `json:"id"`
	FileName    string    `json:"fileName"`
	ContentType string    `json:"contentType"`
	Size        int64     `json:"size"`
	Pages       int       `json:"pages"`
	SHA256      string    `json:"sha256"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Detail struct {
	ID          string `json:"id,omitempty"`
	Section     string `json:"section"`
	ItemName    string `json:"itemName"`
	Requirement string `json:"requirement"`
	Unit        string `json:"unit"`
	Method      string `json:"method"`
	ResultRaw   string `json:"resultRaw"`
	Judgement   string `json:"judgement"`
	Position    int    `json:"position"`
	SourcePage  int    `json:"sourcePage"`
	SourceText  string `json:"sourceText"`
}

type OCRPage struct {
	Page int    `json:"page"`
	Text string `json:"text"`
}

type OCRRun struct {
	ID           string          `json:"id"`
	Status       string          `json:"status"`
	Provider     string          `json:"provider"`
	Model        string          `json:"model"`
	Text         string          `json:"text,omitempty"`
	Pages        []OCRPage       `json:"pages,omitempty"`
	Raw          json.RawMessage `json:"raw,omitempty"`
	ErrorCode    string          `json:"errorCode,omitempty"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	StartedAt    time.Time       `json:"startedAt"`
	FinishedAt   *time.Time      `json:"finishedAt,omitempty"`
}

type ParseRun struct {
	ID          string          `json:"id"`
	Status      string          `json:"status"`
	RuleID      string          `json:"ruleId,omitempty"`
	RuleVersion int             `json:"ruleVersion,omitempty"`
	Fields      json.RawMessage `json:"fields,omitempty"`
	Details     json.RawMessage `json:"details,omitempty"`
	Issues      []Issue         `json:"issues,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	AppliedAt   *time.Time      `json:"appliedAt,omitempty"`
}

type Event struct {
	ID        string          `json:"id"`
	Action    string          `json:"action"`
	Actor     string          `json:"actor"`
	Reason    string          `json:"reason,omitempty"`
	Before    json.RawMessage `json:"before,omitempty"`
	After     json.RawMessage `json:"after,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}

type ReportSummary struct {
	ID            string    `json:"id"`
	ReportNumber  string    `json:"reportNumber"`
	ReportName    string    `json:"reportName"`
	ArchiveStatus string    `json:"archiveStatus"`
	ReportVersion int       `json:"reportVersion"`
	IsCurrent     bool      `json:"isCurrent"`
	RevisionOf    string    `json:"revisionOf,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

type UploadBatch struct {
	ID        string       `json:"id"`
	CreatedBy string       `json:"createdBy"`
	Total     int          `json:"total"`
	Accepted  int          `json:"accepted"`
	Failed    int          `json:"failed"`
	Duplicate int          `json:"duplicate"`
	CreatedAt time.Time    `json:"createdAt"`
	Items     []UploadItem `json:"items"`
}

type UploadItem struct {
	ID            string    `json:"id"`
	FileName      string    `json:"fileName"`
	Status        string    `json:"status"`
	ReportID      string    `json:"reportId,omitempty"`
	FileID        string    `json:"fileId,omitempty"`
	SHA256        string    `json:"sha256,omitempty"`
	Size          int64     `json:"size,omitempty"`
	Pages         int       `json:"pages,omitempty"`
	ErrorCode     string    `json:"errorCode,omitempty"`
	ErrorMessage  string    `json:"errorMessage,omitempty"`
	ArchiveStatus string    `json:"archiveStatus,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Rule is one immutable JSON configuration version. Built-in templates have no tenant id.
type Rule struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenantId,omitempty"`
	TemplateKey string     `json:"templateKey"`
	Name        string     `json:"name"`
	Version     int        `json:"version"`
	Status      string     `json:"status"`
	Builtin     bool       `json:"builtin"`
	Config      RuleConfig `json:"config"`
	CreatedBy   string     `json:"createdBy"`
	CreatedAt   time.Time  `json:"createdAt"`
	ActivatedAt *time.Time `json:"activatedAt,omitempty"`
	RetiredAt   *time.Time `json:"retiredAt,omitempty"`
}

type RuleConfig struct {
	TemplateKey   string               `json:"templateKey"`
	Name          string               `json:"name"`
	Institution   string               `json:"institution"`
	LayoutMarkers []string             `json:"layoutMarkers"`
	Fields        map[string]FieldRule `json:"fields"`
	Details       DetailRule           `json:"details"`
	Validation    ValidationRule       `json:"validation"`
}

type FieldRule struct {
	Labels   []string `json:"labels"`
	Pattern  string   `json:"pattern"`
	Required bool     `json:"required"`
	Format   string   `json:"format,omitempty"`
	Exclude  []string `json:"exclude,omitempty"`
}

type DetailRule struct {
	StartMarkers        []string `json:"startMarkers"`
	EndMarkers          []string `json:"endMarkers"`
	SkipPatterns        []string `json:"skipPatterns"`
	SectionPattern      string   `json:"sectionPattern,omitempty"`
	RowPattern          string   `json:"rowPattern"`
	ContinuationPattern string   `json:"continuationPattern,omitempty"`
	MinRows             int      `json:"minRows"`
	ExpectedRows        int      `json:"expectedRows,omitempty"`
	RequireFields       []string `json:"requireFields"`
}

type ValidationRule struct {
	RequiredFields   []string `json:"requiredFields"`
	RequireDetails   bool     `json:"requireDetails"`
	AllowEmptyValues []string `json:"allowEmptyValues,omitempty"`
}

type Issue struct {
	Code    string `json:"code"`
	Stage   string `json:"stage"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
	Page    int    `json:"page,omitempty"`
	Line    int    `json:"line,omitempty"`
}

type ValidationPreview struct {
	Issues []Issue `json:"issues"`
}

type ParseResult struct {
	Fields       map[string]string      `json:"fields"`
	FieldSources map[string]FieldSource `json:"fieldSources"`
	RawFields    map[string]any         `json:"rawFields"`
	Details      []Detail               `json:"details"`
	Issues       []Issue                `json:"issues"`
	RuleID       string                 `json:"ruleId"`
	RuleVersion  int                    `json:"ruleVersion"`
	TemplateKey  string                 `json:"templateKey"`
}

type ListFilter struct {
	Query       string
	Institution string
	DateFrom    string
	DateTo      string
	Status      string
	Item        string
	Judgement   string
	Scope       string
	Page        int
	PageSize    int
}

type ListResult struct {
	Items    []Report `json:"items"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
}

// APIError is a stable client-facing error with a status and code.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

func fail(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

var (
	ErrNotFound        = errors.New("report archive: not found")
	ErrConflict        = errors.New("report archive: conflict")
	ErrInvalidInput    = errors.New("report archive: invalid input")
	ErrForbidden       = fail(http.StatusForbidden, "PERMISSION_DENIED", "only tenant administrators may perform this action")
	ErrNoTenant        = fail(http.StatusConflict, "TENANT_CONTEXT_REQUIRED", "select a tenant first")
	ErrNotMember       = fail(http.StatusForbidden, "TENANT_NOT_A_MEMBER", "you are not a member of this tenant")
	ErrUnauthenticated = fail(http.StatusUnauthorized, "UNAUTHENTICATED", "sign in required")
)

func trim(s string) string { return strings.TrimSpace(s) }

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func validUUID(s string) bool { return uuidPattern.MatchString(strings.ToLower(strings.TrimSpace(s))) }

func normalizeDate(s string) (string, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, " ", ""))
	s = strings.ReplaceAll(s, "年", "-")
	s = strings.ReplaceAll(s, "月", "-")
	s = strings.ReplaceAll(s, "日", "")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ".", "-")
	parts := strings.Split(s, "-")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid date %q", s)
	}
	for i, p := range parts {
		parts[i] = strings.TrimLeft(p, "0")
		if parts[i] == "" {
			parts[i] = "0"
		}
	}
	year, month, day := parts[0], parts[1], parts[2]
	if len(year) != 4 {
		return "", fmt.Errorf("invalid year in %q", s)
	}
	if len(month) != 2 {
		month = "0" + month
	}
	if len(day) != 2 {
		day = "0" + day
	}
	out := year + "-" + month + "-" + day
	if _, err := time.Parse("2006-01-02", out); err != nil {
		return "", fmt.Errorf("invalid date %q", s)
	}
	return out, nil
}

func compilePattern(pattern string) (*regexp.Regexp, error) {
	if len(pattern) > 2000 {
		return nil, fmt.Errorf("pattern is too long")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("pattern %q: %w", pattern, err)
	}
	return re, nil
}
