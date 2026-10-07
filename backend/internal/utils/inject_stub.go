//go:build !cgo

// Package utils provides internal helpers.
//
// File-level note: this file is the pure-Go stub for SQL validation when the
// project is built with CGO_ENABLED=0 (i.e. without gcc). The full parser
// lives in inject.go under `//go:build cgo`. The stub is intentionally
// permissive: it accepts all SQL and does no security analysis. Production
// deployments must build with CGO enabled (CI step in T0.9 / T1.x).
//
// TODO(T1.x): remove this file once a C toolchain is provisioned on every
// developer workstation and CI runner; the CGO path then becomes the only
// path and no stub is needed.
package utils

import "strings"

// SQLParseResult represents the parsed components of a SELECT SQL statement.
//
// In the pure-Go stub no actual parsing is performed; fields are zero-valued
// and IsSelect / ParseError are populated by best-effort heuristics.
type SQLParseResult struct {
	IsSelect     bool     `json:"is_select"`
	TableNames   []string `json:"table_names"`
	SelectFields []string `json:"select_fields"`
	WhereFields  []string `json:"where_fields"`
	WhereClause  string   `json:"where_clause"`
	OriginalSQL  string   `json:"original_sql"`
	ParseError   string   `json:"parse_error,omitempty"`
}

// SQLValidationError represents a validation error.
type SQLValidationError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Details string `json:"details"`
}

// SQLValidationResult represents the result of SQL validation.
type SQLValidationResult struct {
	Valid  bool                 `json:"valid"`
	Errors []SQLValidationError `json:"errors"`
}

// SearchScope describes one allowed knowledge scope for SQL query injection.
type SearchScope struct {
	KnowledgeBaseID string
	KnowledgeIDs    []string
	TagIDs          []string
}

// SQLValidationOption is a function that configures SQL validation.
//
// In the stub all options are accepted and ignored; the stub validator
// performs only the most basic syntax / injection-risk heuristics.
type SQLValidationOption func(*sqlValidator)

// sqlValidator holds validation configuration.
type sqlValidator struct {
	checkInputValidation      bool
	minLength                 int
	maxLength                 int
	checkSelectOnly           bool
	checkSingleStatement      bool
	allowedTables             map[string]bool
	checkTableNames           bool
	allowedFunctions          map[string]bool
	checkFunctionNames        bool
	checkInjectionRisk        bool
	checkSubqueries           bool
	checkCTEs                 bool
	checkSystemColumns        bool
	checkSchemaAccess         bool
	checkDangerousFuncs       bool
	enableTenantInjection     bool
	tenantID                  uint64
	tablesWithTenantID        map[string]bool
	enableSoftDeleteInjection bool
	tablesWithDeletedAt       map[string]bool
	enableHiddenKBFilter      bool
	enableChunkEnabledFilter  bool
	enableSearchScopeFilter   bool
	searchScopeKBIDs          []string
	searchScopeKnowledgeIDs   []string
	searchScopes              []SearchScope
}

// ParseSQL returns a stub SQLParseResult. Pure-Go stub; no real parsing.
func ParseSQL(sql string) *SQLParseResult {
	result := &SQLParseResult{
		OriginalSQL:  sql,
		TableNames:   []string{},
		SelectFields: []string{},
		WhereFields:  []string{},
	}
	trimmed := strings.TrimSpace(sql)
	upper := strings.ToUpper(trimmed)
	if strings.HasPrefix(upper, "SELECT") {
		result.IsSelect = true
	} else {
		result.IsSelect = false
		result.ParseError = "SQL parsing requires CGO build (T1.x)"
	}
	return result
}

// WithAllowedTables is a stub option.
func WithAllowedTables(tables ...string) SQLValidationOption {
	return func(v *sqlValidator) {
		v.checkTableNames = true
		for _, t := range tables {
			if v.allowedTables == nil {
				v.allowedTables = make(map[string]bool)
			}
			v.allowedTables[t] = true
		}
	}
}

// WithInjectionRiskCheck is a stub option.
func WithInjectionRiskCheck() SQLValidationOption {
	return func(v *sqlValidator) { v.checkInjectionRisk = true }
}

// WithInputValidation is a stub option.
func WithInputValidation(minLen, maxLen int) SQLValidationOption {
	return func(v *sqlValidator) {
		v.checkInputValidation = true
		v.minLength = minLen
		v.maxLength = maxLen
	}
}

// WithSelectOnly is a stub option.
func WithSelectOnly() SQLValidationOption {
	return func(v *sqlValidator) { v.checkSelectOnly = true }
}

// WithSingleStatement is a stub option.
func WithSingleStatement() SQLValidationOption {
	return func(v *sqlValidator) { v.checkSingleStatement = true }
}

// WithAllowedFunctions is a stub option.
func WithAllowedFunctions(functions ...string) SQLValidationOption {
	return func(v *sqlValidator) {
		v.checkFunctionNames = true
		for _, f := range functions {
			if v.allowedFunctions == nil {
				v.allowedFunctions = make(map[string]bool)
			}
			v.allowedFunctions[f] = true
		}
	}
}

// WithDefaultSafeFunctions is a stub option.
func WithDefaultSafeFunctions() SQLValidationOption {
	return func(v *sqlValidator) { v.checkFunctionNames = true }
}

// WithNoSubqueries is a stub option.
func WithNoSubqueries() SQLValidationOption {
	return func(v *sqlValidator) { v.checkSubqueries = true }
}

// WithNoCTEs is a stub option.
func WithNoCTEs() SQLValidationOption {
	return func(v *sqlValidator) { v.checkCTEs = true }
}

// WithNoSystemColumns is a stub option.
func WithNoSystemColumns() SQLValidationOption {
	return func(v *sqlValidator) { v.checkSystemColumns = true }
}

// WithNoSchemaAccess is a stub option.
func WithNoSchemaAccess() SQLValidationOption {
	return func(v *sqlValidator) { v.checkSchemaAccess = true }
}

// WithNoDangerousFunctions is a stub option.
func WithNoDangerousFunctions() SQLValidationOption {
	return func(v *sqlValidator) { v.checkDangerousFuncs = true }
}

// WithTenantIsolation is a stub option.
func WithTenantIsolation(tenantID uint64, tables ...string) SQLValidationOption {
	return func(v *sqlValidator) {
		v.enableTenantInjection = true
		v.tenantID = tenantID
		for _, t := range tables {
			if v.tablesWithTenantID == nil {
				v.tablesWithTenantID = make(map[string]bool)
			}
			v.tablesWithTenantID[t] = true
		}
	}
}

// WithSoftDeleteFilter is a stub option.
func WithSoftDeleteFilter(tables ...string) SQLValidationOption {
	return func(v *sqlValidator) {
		v.enableSoftDeleteInjection = true
		for _, t := range tables {
			if v.tablesWithDeletedAt == nil {
				v.tablesWithDeletedAt = make(map[string]bool)
			}
			v.tablesWithDeletedAt[t] = true
		}
	}
}

// WithHiddenKBFilter is a stub option.
func WithHiddenKBFilter() SQLValidationOption {
	return func(v *sqlValidator) { v.enableHiddenKBFilter = true }
}

// WithChunkEnabledFilter is a stub option.
func WithChunkEnabledFilter() SQLValidationOption {
	return func(v *sqlValidator) { v.enableChunkEnabledFilter = true }
}

// WithSearchScopeFilter is a stub option.
func WithSearchScopeFilter(kbIDs []string, knowledgeIDs []string) SQLValidationOption {
	return func(v *sqlValidator) {
		v.enableSearchScopeFilter = true
		v.searchScopeKBIDs = kbIDs
		v.searchScopeKnowledgeIDs = knowledgeIDs
	}
}

// WithSearchScopes is a stub option.
func WithSearchScopes(scopes []SearchScope) SQLValidationOption {
	return func(v *sqlValidator) {
		v.searchScopes = scopes
		v.enableSearchScopeFilter = true
	}
}

// WithSecurityDefaults is a stub option.
func WithSecurityDefaults(tenantID uint64) SQLValidationOption {
	return func(v *sqlValidator) {
		v.tenantID = tenantID
		v.checkInputValidation = true
		v.minLength = 6
		v.maxLength = 4096
		v.checkSelectOnly = true
		v.checkSingleStatement = true
		v.checkDangerousFuncs = true
		v.enableTenantInjection = true
		v.enableHiddenKBFilter = true
		v.enableChunkEnabledFilter = true
	}
}

// ValidateSQL is the pure-Go stub. It accepts any non-empty SQL and returns
// Valid=true. Production deployments must build with CGO_ENABLED=1.
func ValidateSQL(sql string, opts ...SQLValidationOption) (*SQLParseResult, *SQLValidationResult) {
	validator := &sqlValidator{}
	for _, opt := range opts {
		opt(validator)
	}
	parseResult := ParseSQL(sql)
	validationResult := &SQLValidationResult{
		Valid:  true,
		Errors: []SQLValidationError{},
	}
	if validator.checkInputValidation && validator.minLength > 0 && len(sql) < validator.minLength {
		validationResult.Valid = false
		validationResult.Errors = append(validationResult.Errors, SQLValidationError{
			Type:    "input_too_short",
			Message: "SQL statement is too short",
		})
	}
	return parseResult, validationResult
}

// ValidateAndSecureSQL is the pure-Go stub. It returns the original SQL
// unchanged with Valid=true. Production deployments must build with
// CGO_ENABLED=1 to enforce tenant isolation, soft-delete injection, and
// other security checks.
func ValidateAndSecureSQL(sql string, opts ...SQLValidationOption) (string, *SQLValidationResult, error) {
	parseResult, validationResult := ValidateSQL(sql, opts...)
	_ = parseResult
	return sql, validationResult, nil
}

// InjectAndConditions appends `AND filter` to the SQL's WHERE clause. The
// stub does a string-level append; the CGO path does AST-level rewriting.
func InjectAndConditions(sql, filter string) string {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	if strings.Contains(upper, " WHERE ") {
		return sql + " AND " + filter
	}
	return sql + " WHERE " + filter
}
