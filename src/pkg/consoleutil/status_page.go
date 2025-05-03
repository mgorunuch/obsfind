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

// Status represents the display status for status page elements.
// This differs from StatusType which is used for general status messages.
type Status int

// Status constants for status page elements
const (
	StatusActive   Status = iota // Active/online component
	StatusInactive               // Inactive/offline component
	StatusPending                // Pending/initializing component
	StatusUnknown                // Unknown component status
)

// StatusTypeMapping maps Status values to their equivalent StatusType
// This allows reusing color and formatting from the status package
var StatusTypeMapping = map[Status]StatusType{
	StatusActive:   StatusSuccess,
	StatusInactive: StatusError,
	StatusPending:  StatusWarning,
	StatusUnknown:  StatusInfo,
}

// For compatibility with tables.go, we need these aliases
// We'll define them as variables instead of constants to avoid redeclaration errors
var (
	StatusSuccessValue Status = StatusActive
	StatusErrorValue   Status = StatusInactive
	StatusWarningValue Status = StatusPending
	StatusInfoValue    Status = StatusUnknown
)

// statusPageColors defines specific colors for status page elements
// Falls back to statusColors from status.go when not specified
var statusPageColors = map[Status]string{
	StatusActive:   FgGreen,  // Same as StatusSuccess
	StatusInactive: FgRed,    // Same as StatusError
	StatusPending:  FgYellow, // Same as StatusWarning
	StatusUnknown:  FgBlue,   // Same as StatusInfo
}

// getStatusColor returns the appropriate ANSI color code for the given status
func getStatusColor(status Status) string {
	// First check our status-specific colors
	if color, ok := statusPageColors[status]; ok {
		return color
	}

	// Fall back to StatusType colors via mapping
	if mappedType, ok := StatusTypeMapping[status]; ok {
		return statusColors[mappedType]
	}

	// Default fallback
	return FgDefault
}

// ColorizeStatus applies color formatting to text based on status
func ColorizeStatus(text string, status Status) string {
	return ColorText(text, getStatusColor(status))
}

// ColorizeStatusBold applies bold color formatting to text based on status
func ColorizeStatusBold(text string, status Status) string {
	return Format(text, Bold, getStatusColor(status))
}

// FormatStatusLine formats a status line with label, value, and appropriate coloring
func FormatStatusLine(label, value string, status Status) string {
	var sb strings.Builder

	// Format the label in bold
	sb.WriteString(Format(label, Bold))
	sb.WriteString(": ")

	// Format the value with status-appropriate color
	sb.WriteString(ColorizeStatus(value, status))

	// Add status indicator at the end if specified
	indicator := GetStatusIndicator(status)
	if indicator != "" {
		sb.WriteString(" ")
		sb.WriteString(indicator)
	}

	return sb.String()
}

// FormatStatusBlock creates a formatted block of status information
// with a title and multiple status lines
func FormatStatusBlock(title string, items map[string]StatusRow) string {
	var sb strings.Builder

	// Add title
	if title != "" {
		sb.WriteString(Format(fmt.Sprintf("=== %s ===\n", title), Bold))
	}

	// Sort and add items
	for _, item := range items {
		prefix := "  " // Indent each status line
		sb.WriteString(prefix)
		sb.WriteString(FormatStatusLine(item.Label, item.Value, item.Status))
		sb.WriteString("\n")
	}

	return sb.String()
}

// GetStatusFromState converts a simple boolean state to a Status
func GetStatusFromState(isActive bool) Status {
	if isActive {
		return StatusActive
	}
	return StatusInactive
}

// GetStatusTextFromState returns a descriptive text based on active state
func GetStatusTextFromState(isActive bool) string {
	if isActive {
		return "Active"
	}
	return "Inactive"
}
