package consts

// API endpoint constants
const (
	// API version prefix
	APIPrefix = "/api/v1"

	// Health endpoints
	APIHealthEndpoint = APIPrefix + "/health"

	// Status endpoints
	APIStatusEndpoint = APIPrefix + "/status"

	// Search endpoints
	APISearchPrefix  = APIPrefix + "/search"
	APISearchQuery   = APISearchPrefix + "/query"
	APISearchSimilar = APISearchPrefix + "/similar"

	// Index endpoints
	APIIndexPrefix = APIPrefix + "/index"
	APIIndexFile   = APIIndexPrefix + "/file"
	APIIndexAll    = APIIndexPrefix + "/all"
	APIIndexStatus = APIIndexPrefix + "/status"
)

// Query parameter keys
const (
	// Common query parameters
	QueryParamQuery      = "q"
	QueryParamLimit      = "limit"
	QueryParamOffset     = "offset"
	QueryParamMinScore   = "min_score"
	QueryParamTag        = "tag"
	QueryParamPathPrefix = "path_prefix"
	QueryParamFilter     = "filter"
)

// Filter prefixes
const (
	FilterPrefixPath = "path:"
	FilterPrefixTags = "tags:"
)

// Default values
const (
	DefaultSearchLimit = 10
)
