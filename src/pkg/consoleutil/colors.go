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

// Package consoleutil provides utilities for handling colored console output.
// This package helps with formatting terminal output with ANSI color codes
// and provides cross-platform support for colored console output.
package consoleutil

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"runtime"
	"strings"
)

// ANSI color codes for text foreground
const (
	FgBlack   = "\033[30m"
	FgRed     = "\033[31m"
	FgGreen   = "\033[32m"
	FgYellow  = "\033[33m"
	FgBlue    = "\033[34m"
	FgMagenta = "\033[35m"
	FgCyan    = "\033[36m"
	FgWhite   = "\033[37m"
	FgDefault = "\033[39m"

	// Bright foreground colors
	FgBrightBlack   = "\033[90m"
	FgBrightRed     = "\033[91m"
	FgBrightGreen   = "\033[92m"
	FgBrightYellow  = "\033[93m"
	FgBrightBlue    = "\033[94m"
	FgBrightMagenta = "\033[95m"
	FgBrightCyan    = "\033[96m"
	FgBrightWhite   = "\033[97m"
)

// ANSI color codes for text background
const (
	BgBlack   = "\033[40m"
	BgRed     = "\033[41m"
	BgGreen   = "\033[42m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"
	BgWhite   = "\033[47m"
	BgDefault = "\033[49m"

	// Bright background colors
	BgBrightBlack   = "\033[100m"
	BgBrightRed     = "\033[101m"
	BgBrightGreen   = "\033[102m"
	BgBrightYellow  = "\033[103m"
	BgBrightBlue    = "\033[104m"
	BgBrightMagenta = "\033[105m"
	BgBrightCyan    = "\033[106m"
	BgBrightWhite   = "\033[107m"
)

// ANSI format codes
const (
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Italic    = "\033[3m"
	Underline = "\033[4m"
	Blink     = "\033[5m"
	Reverse   = "\033[7m"
	Hidden    = "\033[8m"
	Strikeout = "\033[9m"
	Reset     = "\033[0m"
)

// ColorSupport indicates the level of color support in the terminal
type ColorSupport int

const (
	// ColorNone indicates no color support
	ColorNone ColorSupport = iota
	// ColorBasic indicates support for basic 8 colors
	ColorBasic
	// Color256 indicates support for 256 colors
	Color256
	// ColorTrueColor indicates support for 24-bit true color
	ColorTrueColor
)

var (
	// forceColor can be set to override automatic color detection
	forceColor *bool

	// cached result of color support detection
	cachedColorSupport *ColorSupport

	// ANSI escape sequence pattern for stripping
	ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
)

// SetForceColor allows forcing color output on or off, overriding automatic detection.
// This is useful for unit tests or when the program knows better than the automatic detection.
func SetForceColor(force bool) {
	forceColor = &force
	// Reset cache when force value changes
	cachedColorSupport = nil
}

// isWindowsNonANSI returns true if running on Windows without ANSI support.
// Note: Modern Windows terminals usually support ANSI codes, but this
// is included for potential backward compatibility if needed.
func isWindowsNonANSI() bool {
	// Modern Windows terminals generally support ANSI codes,
	// but this can be modified if specific detection is needed.
	return runtime.GOOS == "windows" && os.Getenv("WT_SESSION") == "" &&
		os.Getenv("TERM") == "" && os.Getenv("TERM_PROGRAM") == ""
}

// GetColorSupport detects the level of color support in the current terminal.
// The result is cached for performance.
func GetColorSupport() ColorSupport {
	// Return cached result if available
	if cachedColorSupport != nil {
		return *cachedColorSupport
	}

	// Use forced value if set
	if forceColor != nil {
		if *forceColor {
			support := ColorTrueColor
			cachedColorSupport = &support
		} else {
			support := ColorNone
			cachedColorSupport = &support
		}
		return *cachedColorSupport
	}

	// Disable colors for non-TTY output
	if !isTerminal(os.Stdout) {
		support := ColorNone
		cachedColorSupport = &support
		return ColorNone
	}

	// Check if ANSI colors are explicitly disabled
	if os.Getenv("NO_COLOR") != "" || strings.ToLower(os.Getenv("TERM")) == "dumb" {
		support := ColorNone
		cachedColorSupport = &support
		return ColorNone
	}

	// Check for Windows non-ANSI terminal
	if isWindowsNonANSI() {
		support := ColorNone
		cachedColorSupport = &support
		return ColorNone
	}

	// Determine color support level based on environment variables
	colorTerm := os.Getenv("COLORTERM")
	if colorTerm == "truecolor" || colorTerm == "24bit" {
		support := ColorTrueColor
		cachedColorSupport = &support
		return ColorTrueColor
	}

	// Check terminal type
	term := os.Getenv("TERM")
	if strings.Contains(term, "256color") {
		support := Color256
		cachedColorSupport = &support
		return Color256
	}

	if strings.HasPrefix(term, "xterm") || strings.HasPrefix(term, "screen") ||
		strings.HasPrefix(term, "vt100") || strings.Contains(term, "color") {
		support := ColorBasic
		cachedColorSupport = &support
		return ColorBasic
	}

	// Default to basic color support for most terminals
	support := ColorBasic
	cachedColorSupport = &support
	return ColorBasic
}

// isTerminal checks if the given file is a terminal.
// This is a simplified implementation that could be enhanced with platform-specific checks.
func isTerminal(file *os.File) bool {
	// On Windows, this should use syscall to check if handle is a terminal
	// On Unix, this should use isatty
	// For simplicity, we'll just check if it's Stdout, Stderr, or Stdin
	return file == os.Stdout || file == os.Stderr || file == os.Stdin
}

// IsColorSupported returns whether the current environment supports ANSI colors.
func IsColorSupported() bool {
	return GetColorSupport() != ColorNone
}

// Color256Code returns the ANSI escape code for a 256-color foreground color.
func Color256Code(code uint8) string {
	return fmt.Sprintf("\033[38;5;%dm", code)
}

// BgColor256Code returns the ANSI escape code for a 256-color background color.
func BgColor256Code(code uint8) string {
	return fmt.Sprintf("\033[48;5;%dm", code)
}

// RGBCode returns the ANSI escape code for a 24-bit RGB foreground color.
func RGBCode(r, g, b uint8) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
}

// BgRGBCode returns the ANSI escape code for a 24-bit RGB background color.
func BgRGBCode(r, g, b uint8) string {
	return fmt.Sprintf("\033[48;2;%d;%d;%dm", r, g, b)
}

// ColorText applies a foreground color to the provided text and resets color at the end.
// This function handles platform compatibility automatically.
func ColorText(text, color string) string {
	if !IsColorSupported() {
		return text // Return plain text if colors not supported
	}
	return color + text + Reset
}

// ColorTextf formats and colorizes text with a foreground color.
func ColorTextf(color string, format string, args ...interface{}) string {
	return ColorText(fmt.Sprintf(format, args...), color)
}

// ColorBackground applies a background color to the provided text and resets color at the end.
// This function handles platform compatibility automatically.
func ColorBackground(text, bgColor string) string {
	if !IsColorSupported() {
		return text // Return plain text if colors not supported
	}
	return bgColor + text + Reset
}

// ColorizeRGB applies a 24-bit RGB foreground color to the text if supported.
// Falls back to nearest basic color for terminals without true color support.
func ColorizeRGB(text string, r, g, b uint8) string {
	support := GetColorSupport()

	if support == ColorNone {
		return text
	}

	if support == ColorTrueColor {
		return RGBCode(r, g, b) + text + Reset
	}

	// Fall back to 256 colors or basic colors based on support
	if support == Color256 {
		// Convert RGB to approximate 256-color code
		code := rgbTo256(r, g, b)
		return Color256Code(code) + text + Reset
	}

	// Fall back to basic 8 colors
	return ColorText(text, nearestBasicColor(r, g, b))
}

// Format applies multiple formatting options to text.
// Example: Format("Important", Bold, FgRed)
func Format(text string, formats ...string) string {
	if !IsColorSupported() {
		return text // Return plain text if colors not supported
	}

	formatting := ""
	for _, format := range formats {
		formatting += format
	}

	return formatting + text + Reset
}

// Formatf applies formatting to formatted text.
// Example: Formatf(Bold+FgRed, "Error: %s", err)
func Formatf(format string, textFormat string, args ...interface{}) string {
	return Format(fmt.Sprintf(textFormat, args...), format)
}

// FormatBuilder is a helper for constructing formatted text in multiple steps.
type FormatBuilder struct {
	text      string
	formatted bool
}

// NewFormatBuilder creates a new format builder for the given text.
func NewFormatBuilder(text string) *FormatBuilder {
	return &FormatBuilder{
		text:      text,
		formatted: false,
	}
}

// WithColor adds a foreground color to the text.
func (fb *FormatBuilder) WithColor(color string) *FormatBuilder {
	if IsColorSupported() {
		fb.text = color + fb.text
		fb.formatted = true
	}
	return fb
}

// WithBackground adds a background color to the text.
func (fb *FormatBuilder) WithBackground(bgColor string) *FormatBuilder {
	if IsColorSupported() {
		fb.text = bgColor + fb.text
		fb.formatted = true
	}
	return fb
}

// WithFormat adds a format like Bold or Underline to the text.
func (fb *FormatBuilder) WithFormat(format string) *FormatBuilder {
	if IsColorSupported() {
		fb.text = format + fb.text
		fb.formatted = true
	}
	return fb
}

// WithRGB adds a 24-bit RGB foreground color to the text if supported.
func (fb *FormatBuilder) WithRGB(r, g, b uint8) *FormatBuilder {
	support := GetColorSupport()

	if support == ColorNone {
		return fb
	}

	if support == ColorTrueColor {
		fb.text = RGBCode(r, g, b) + fb.text
		fb.formatted = true
		return fb
	}

	// Fall back to 256 colors or basic colors based on support
	if support == Color256 {
		// Convert RGB to approximate 256-color code
		code := rgbTo256(r, g, b)
		fb.text = Color256Code(code) + fb.text
		fb.formatted = true
		return fb
	}

	// Fall back to basic 8 colors
	return fb.WithColor(nearestBasicColor(r, g, b))
}

// Build finalizes the formatting and returns the formatted string.
func (fb *FormatBuilder) Build() string {
	if fb.formatted && IsColorSupported() {
		return fb.text + Reset
	}
	return fb.text
}

// StripANSI removes ANSI escape sequences from a string
func StripANSI(str string) string {
	return ansiPattern.ReplaceAllString(str, "")
}

// PrettyJSON returns a colorized JSON string for terminal output.
// This is useful for displaying JSON data with syntax highlighting.
func PrettyJSON(jsonStr string) string {
	if !IsColorSupported() {
		return jsonStr
	}

	// Simple JSON syntax highlighting
	result := strings.Builder{}
	inString := false
	inNumber := false

	for i := 0; i < len(jsonStr); i++ {
		c := jsonStr[i]

		switch {
		case c == '"':
			if i == 0 || jsonStr[i-1] != '\\' {
				inString = !inString
			}

			if inString {
				result.WriteString(FgGreen)
			}
			result.WriteByte(c)
			if !inString {
				result.WriteString(Reset)
			}

		case c == '{' || c == '}' || c == '[' || c == ']':
			if inString {
				result.WriteByte(c)
			} else {
				result.WriteString(FgCyan)
				result.WriteByte(c)
				result.WriteString(Reset)
			}

		case c == ':':
			result.WriteByte(c)
			if !inString {
				result.WriteString(" ")
			}

		case c == ',':
			result.WriteByte(c)
			if !inString {
				result.WriteString(" ")
			}

		case c >= '0' && c <= '9' || c == '-' || c == '.':
			if inString {
				result.WriteByte(c)
			} else {
				if !inNumber {
					result.WriteString(FgYellow)
					inNumber = true
				}
				result.WriteByte(c)
			}

		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if inString {
				result.WriteByte(c)
			} else if i > 0 && jsonStr[i-1] != ':' && jsonStr[i-1] != ',' {
				// Skip extra whitespace but keep formatting
				continue
			}

		default:
			if inNumber && !(c >= '0' && c <= '9') && c != 'e' && c != 'E' && c != '+' {
				inNumber = false
				result.WriteString(Reset)
			}

			if !inString && (c == 't' || c == 'f' || c == 'n') {
				// true, false, null
				// Check if we have a full keyword
				if i+3 < len(jsonStr) && jsonStr[i:i+4] == "true" {
					result.WriteString(FgMagenta)
					result.WriteString("true")
					result.WriteString(Reset)
					i += 3
					continue
				} else if i+4 < len(jsonStr) && jsonStr[i:i+5] == "false" {
					result.WriteString(FgMagenta)
					result.WriteString("false")
					result.WriteString(Reset)
					i += 4
					continue
				} else if i+3 < len(jsonStr) && jsonStr[i:i+4] == "null" {
					result.WriteString(FgMagenta)
					result.WriteString("null")
					result.WriteString(Reset)
					i += 3
					continue
				}
			}

			result.WriteByte(c)
		}
	}

	if inNumber || inString {
		result.WriteString(Reset)
	}

	return result.String()
}

// Helper function to convert RGB to the nearest basic ANSI color
func nearestBasicColor(r, g, b uint8) string {
	// Simplified algorithm to find the nearest basic color
	// This can be improved for better color matching

	if r == g && g == b {
		// Grayscale
		if r < 64 {
			return FgBlack
		} else if r < 192 {
			return FgWhite
		} else {
			return FgBrightWhite
		}
	}

	// Find dominant color
	max := r
	if g > max {
		max = g
	}
	if b > max {
		max = b
	}

	bright := max >= 192

	switch {
	case r == max && r > g+b:
		return map[bool]string{false: FgRed, true: FgBrightRed}[bright]
	case g == max && g > r+b:
		return map[bool]string{false: FgGreen, true: FgBrightGreen}[bright]
	case b == max && b > r+g:
		return map[bool]string{false: FgBlue, true: FgBrightBlue}[bright]
	case r == max && g > b:
		return map[bool]string{false: FgYellow, true: FgBrightYellow}[bright]
	case g == max && b > r:
		return map[bool]string{false: FgCyan, true: FgBrightCyan}[bright]
	case b == max && r > g:
		return map[bool]string{false: FgMagenta, true: FgBrightMagenta}[bright]
	default:
		return map[bool]string{false: FgWhite, true: FgBrightWhite}[bright]
	}
}

// Helper function to convert RGB to the nearest 256-color code
func rgbTo256(r, g, b uint8) uint8 {
	// For simplicity, we'll use a basic approximation
	// This could be enhanced with a proper color quantization algorithm

	// Check if it's grayscale
	if r == g && g == b {
		if r < 8 {
			return 16 // black
		}
		if r > 248 {
			return 231 // white
		}
		// Use grayscale palette (24 steps, from 232 to 255)
		return uint8(((r - 8) / 10) + 232)
	}

	// Use 6x6x6 color cube (216 colors, from 16 to 231)
	rr := uint8(math.Min(5, math.Floor(float64(r)/256.0*6.0)))
	gg := uint8(math.Min(5, math.Floor(float64(g)/256.0*6.0)))
	bb := uint8(math.Min(5, math.Floor(float64(b)/256.0*6.0)))

	return 16 + rr*36 + gg*6 + bb
}
