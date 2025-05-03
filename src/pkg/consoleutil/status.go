// Copyright 2023 ObsFind Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
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

// StatusType represents the type of status message
type StatusType int

// Status type constants
const (
	StatusSuccess StatusType = iota
	StatusWarning
	StatusError
	StatusInfo
	StatusDebug
)

// statusIcons defines symbols that represent different status types
var statusIcons = map[StatusType]string{
	StatusSuccess: "✓",
	StatusWarning: "⚠",
	StatusError:   "✗",
	StatusInfo:    "ℹ",
	StatusDebug:   "⋯",
}

// statusLabels defines text labels for different status types
var statusLabels = map[StatusType]string{
	StatusSuccess: "SUCCESS",
	StatusWarning: "WARNING",
	StatusError:   "ERROR",
	StatusInfo:    "INFO",
	StatusDebug:   "DEBUG",
}

// statusColors defines the color to be used for each status type
var statusColors = map[StatusType]string{
	StatusSuccess: FgGreen,
	StatusWarning: FgYellow,
	StatusError:   FgRed,
	StatusInfo:    FgBlue,
	StatusDebug:   FgMagenta,
}

// FormatStatus formats a status message with appropriate coloring and prefix
// based on the status type. The result is a string ready for printing to the console.
func FormatStatus(message string, statusType StatusType) string {
	icon := statusIcons[statusType]
	label := statusLabels[statusType]
	color := statusColors[statusType]

	// Format the label with color
	formattedLabel := ColorText(fmt.Sprintf("[%s]", label), color)

	// Format the icon with color
	formattedIcon := ColorText(icon, color)

	// Combine icon, label, and message
	return fmt.Sprintf("%s %s %s", formattedIcon, formattedLabel, message)
}

// FormatStatusWithPrefix formats a status message with a custom prefix in addition
// to the standard status formatting.
func FormatStatusWithPrefix(prefix, message string, statusType StatusType) string {
	formattedStatus := FormatStatus(message, statusType)
	formattedPrefix := Format(prefix, Bold)
	return fmt.Sprintf("%s: %s", formattedPrefix, formattedStatus)
}

// FormatSuccess formats a success message
func FormatSuccess(message string) string {
	return FormatStatus(message, StatusSuccess)
}

// FormatWarning formats a warning message
func FormatWarning(message string) string {
	return FormatStatus(message, StatusWarning)
}

// FormatError formats an error message
func FormatError(message string) string {
	return FormatStatus(message, StatusError)
}

// FormatInfo formats an informational message
func FormatInfo(message string) string {
	return FormatStatus(message, StatusInfo)
}

// FormatDebug formats a debug message
func FormatDebug(message string) string {
	return FormatStatus(message, StatusDebug)
}

// FormatErrorWithDetails formats an error message with detailed information
// and optional suggested actions. This is useful for providing users with
// more context about what went wrong and how to fix it.
func FormatErrorWithDetails(message, details string, suggestions []string) string {
	var sb strings.Builder

	// Main error message with formatting
	sb.WriteString(FormatError(message))
	sb.WriteString("\n")

	// Error details with some indent
	if details != "" {
		sb.WriteString("  ")
		sb.WriteString(Format(details, FgRed))
		sb.WriteString("\n")
	}

	// Suggestions with formatting
	if len(suggestions) > 0 {
		sb.WriteString("\n")
		sb.WriteString(Format("Suggestions:", FgYellow))
		sb.WriteString("\n")
		for i, suggestion := range suggestions {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, suggestion))
		}
	}

	return sb.String()
}

// ProgressBar creates a simple progress bar for console output
// percent: percentage complete (0-100)
// width: width of the progress bar in characters
// Returns a formatted string representing the progress
func ProgressBar(percent int, width int) string {
	if percent < 0 {
		percent = 0
	} else if percent > 100 {
		percent = 100
	}

	if width < 10 {
		width = 10
	}

	// Calculate the number of filled characters
	filled := width * percent / 100

	// Create the progress bar
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	// Format with color based on progress
	var coloredBar string
	if percent < 30 {
		coloredBar = ColorText(bar, FgRed)
	} else if percent < 70 {
		coloredBar = ColorText(bar, FgYellow)
	} else {
		coloredBar = ColorText(bar, FgGreen)
	}

	return fmt.Sprintf("[%s] %3d%%", coloredBar, percent)
}

// FormatTable formats a simple two-column table with optional headers
// and separators for displaying information in the console.
func FormatTable(headers []string, rows [][]string, useColor bool) string {
	var sb strings.Builder

	// Find the max width for each column
	colCount := 2 // we only support two columns for simplicity
	if len(headers) > colCount {
		headers = headers[:colCount]
	}

	colWidths := make([]int, colCount)

	// Check headers
	if len(headers) == colCount {
		for i, header := range headers {
			if len(header) > colWidths[i] {
				colWidths[i] = len(header)
			}
		}
	}

	// Check rows
	for _, row := range rows {
		if len(row) > colCount {
			row = row[:colCount]
		}

		for i, cell := range row {
			if i < colCount && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Print headers
	if len(headers) == colCount {
		for i, header := range headers {
			if i > 0 {
				sb.WriteString(" | ")
			}

			padding := strings.Repeat(" ", colWidths[i]-len(header))
			if useColor {
				sb.WriteString(Format(header+padding, Bold))
			} else {
				sb.WriteString(header + padding)
			}
		}
		sb.WriteString("\n")

		// Print separator
		for i, width := range colWidths {
			if i > 0 {
				sb.WriteString("-+-")
			}
			sb.WriteString(strings.Repeat("-", width))
		}
		sb.WriteString("\n")
	}

	// Print rows
	for _, row := range rows {
		for i := 0; i < colCount; i++ {
			if i > 0 {
				sb.WriteString(" | ")
			}

			if i < len(row) {
				cell := row[i]
				padding := strings.Repeat(" ", colWidths[i]-len(cell))
				sb.WriteString(cell + padding)
			} else {
				sb.WriteString(strings.Repeat(" ", colWidths[i]))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
