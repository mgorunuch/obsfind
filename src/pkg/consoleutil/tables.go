// Copyright 2023-2025 ObsFind Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package consoleutil

import (
	"fmt"
	"strings"
)

// StatusRow represents a row in a status table with label, value, and associated status.
type StatusRow struct {
	Label  string
	Value  string
	Status Status
}

// StatusTable represents a collection of status rows to be displayed as a table.
type StatusTable struct {
	Title string
	Rows  []StatusRow
}

// RenderRow formats a status row as a string with appropriate coloring.
func RenderRow(row StatusRow) string {
	return FormatStatusLine(row.Label, row.Value, row.Status)
}

// RenderTable renders a complete status table with title and rows.
func RenderTable(table StatusTable) string {
	var sb strings.Builder

	// Add title if provided
	if table.Title != "" {
		title := fmt.Sprintf("--- %s ---", table.Title)
		sb.WriteString(ColorizeStatusBold(title, StatusUnknown)) // Using StatusUnknown instead of StatusInfo
		sb.WriteString("\n\n")
	}

	// Add rows
	for _, row := range table.Rows {
		sb.WriteString(RenderRow(row))
		sb.WriteString("\n")
	}

	return sb.String()
}

// NewStatusTable creates a new status table with the given title.
func NewStatusTable(title string) *StatusTable {
	return &StatusTable{
		Title: title,
		Rows:  []StatusRow{},
	}
}

// AddRow adds a new row to the status table.
func (t *StatusTable) AddRow(label, value string, status Status) *StatusTable {
	t.Rows = append(t.Rows, StatusRow{
		Label:  label,
		Value:  value,
		Status: status,
	})
	return t
}

// Render renders the status table as a string.
func (t *StatusTable) Render() string {
	return RenderTable(*t)
}

// Example status indicators
const (
	ActiveIndicator   = "●" // Unicode bullet for active status
	InactiveIndicator = "○" // Unicode white bullet for inactive status
	WarningIndicator  = "⚠" // Unicode warning sign
	ErrorIndicator    = "✗" // Unicode cross mark
	SuccessIndicator  = "✓" // Unicode check mark
)

// GetStatusIndicator returns a Unicode indicator for the given status.
func GetStatusIndicator(status Status) string {
	switch status {
	case StatusSuccessValue, StatusActive:
		return ColorizeStatus(SuccessIndicator, status)
	case StatusWarningValue, StatusPending:
		return ColorizeStatus(WarningIndicator, status)
	case StatusErrorValue, StatusInactive:
		return ColorizeStatus(ErrorIndicator, status)
	case StatusInfoValue, StatusUnknown:
		return ColorizeStatus(ActiveIndicator, status)
	default:
		return ""
	}
}

// FormatServiceStatus formats a service status with an indicator and text.
func FormatServiceStatus(serviceName, statusText string, status Status) string {
	indicator := GetStatusIndicator(status)
	return fmt.Sprintf("%s %s: %s", indicator, serviceName, ColorizeStatus(statusText, status))
}
