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
)

// ExampleStatusPage demonstrates how to use the status page coloring functionality
func ExampleStatusPage() {
	// Example 1: Creating and rendering a status table
	table := NewStatusTable("System Status")
	table.AddRow("Database", "Connected", StatusActive)
	table.AddRow("API Server", "Running", StatusActive)
	table.AddRow("File Watcher", "Offline", StatusInactive)
	table.AddRow("Indexer", "Processing", StatusPending)

	fmt.Println(table.Render())

	// Example 2: Using individual status formatting functions
	fmt.Println(FormatStatusLine("Daemon", "Active (PID: 1234)", StatusActive))
	fmt.Println(FormatStatusLine("Index Status", "Out of date", StatusPending)) // Using StatusPending instead of StatusWarning

	// Example 3: Formatting service status
	fmt.Println(FormatServiceStatus("Qdrant", "Connected", StatusActive))
	fmt.Println(FormatServiceStatus("Ollama", "Error: Connection refused", StatusInactive)) // Using StatusInactive instead of StatusError

	// Example 4: Using status indicators
	fmt.Printf("Daemon: %s Running\n", GetStatusIndicator(StatusActive))
	fmt.Printf("Indexer: %s Stopped\n", GetStatusIndicator(StatusInactive))

	// Example 5: Using a status block
	items := map[string]StatusRow{
		"db": {
			Label:  "Database",
			Value:  "Connected",
			Status: StatusActive,
		},
		"api": {
			Label:  "API Server",
			Value:  "Running on :8080",
			Status: StatusActive,
		},
		"watcher": {
			Label:  "File Watcher",
			Value:  "Not running",
			Status: StatusInactive,
		},
	}

	fmt.Println("\n" + FormatStatusBlock("Component Status", items))
}

// ExampleColorizeStatus demonstrates how to use the ColorizeStatus function
func ExampleColorizeStatus() {
	// Colorize text based on status
	fmt.Println(ColorizeStatus("Active", StatusActive))
	fmt.Println(ColorizeStatus("Warning", StatusPending)) // Using StatusPending instead of StatusWarning
	fmt.Println(ColorizeStatus("Error", StatusInactive))  // Using StatusInactive instead of StatusError
	fmt.Println(ColorizeStatus("Info", StatusUnknown))    // Using StatusUnknown instead of StatusInfo

	// Colorize with bold formatting
	fmt.Println(ColorizeStatusBold("ACTIVE", StatusActive))
	fmt.Println(ColorizeStatusBold("WARNING", StatusPending)) // Using StatusPending
	fmt.Println(ColorizeStatusBold("ERROR", StatusInactive))  // Using StatusInactive
	fmt.Println(ColorizeStatusBold("INFO", StatusUnknown))    // Using StatusUnknown
}
