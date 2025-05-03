package api

import (
	"obsfind/src/pkg/indexer"
	"time"
)

// SearchRequest represents a search query
type SearchRequest struct {
	Query      string   `json:"query"`
	Limit      int      `json:"limit,omitempty"`
	Offset     int      `json:"offset,omitempty"`
	MinScore   float32  `json:"min_score,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	PathPrefix string   `json:"path_prefix,omitempty"`
}

// SimilarRequest represents a similar document query
type SimilarRequest struct {
	Path       string   `json:"path"`
	Limit      int      `json:"limit,omitempty"`
	Offset     int      `json:"offset,omitempty"`
	MinScore   float32  `json:"min_score,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	PathPrefix string   `json:"path_prefix,omitempty"`
}

// StatusResponse represents the daemon status
type StatusResponse struct {
	Status     string            `json:"status"`
	Uptime     string            `json:"uptime"`
	StartTime  time.Time         `json:"start_time"`
	IndexStats indexer.Stats     `json:"index_stats"`
	Version    string            `json:"version"`
	Config     map[string]string `json:"config"`
}

// IndexFileRequest represents a request to index a specific file
type IndexFileRequest struct {
	FilePath string `json:"file_path"`
	Force    bool   `json:"force,omitempty"`
}

// IndexingStatus represents the current status of the indexing process
type IndexingStatus struct {
	IsIndexing        bool      `json:"is_indexing"`
	Progress          float32   `json:"progress"`
	TotalFiles        int       `json:"total_files"`
	IndexedDocs       int       `json:"indexed_docs"`
	StartTime         time.Time `json:"start_time,omitempty"`
	TotalDocs         int       `json:"total_docs"`
	PercentComplete   float64   `json:"percent_complete"`
	CurrentFile       string    `json:"current_file,omitempty"`
	LastIndexedFile   string    `json:"last_indexed_file,omitempty"`
	IndexingStartTime time.Time `json:"indexing_start_time,omitempty"`
}
