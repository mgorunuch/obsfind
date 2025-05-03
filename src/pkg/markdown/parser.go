package markdown

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Document represents a parsed markdown document
type Document struct {
	Title       string
	Path        string
	Content     string
	Frontmatter map[string]interface{}
	Sections    []Section
	Tags        []string
}

// Section represents a section in a markdown document
type Section struct {
	Title       string
	Level       int
	Content     string
	StartLine   int
	EndLine     int
	StartOffset int
	EndOffset   int
}

// Chunk represents a chunk of text for embedding
type Chunk struct {
	ID          string
	Content     string
	ContentOnly string // Content with markup removed
	Title       string
	Section     string
	SectionPath string
	Tags        []string
	Path        string
	StartLine   int
	EndLine     int
	StartOffset int
	EndOffset   int
}

// ParseOptions contains options for parsing markdown
type ParseOptions struct {
	ExtractTags        bool
	ExtractFrontmatter bool
	IncludeTitle       bool
}

// DefaultParseOptions returns default parsing options
func DefaultParseOptions() ParseOptions {
	return ParseOptions{
		ExtractTags:        true,
		ExtractFrontmatter: true,
		IncludeTitle:       true,
	}
}

// Parser handles markdown parsing
type Parser struct {
	options ParseOptions
}

// NewParser creates a new markdown parser with default options
func NewParser() *Parser {
	return &Parser{
		options: DefaultParseOptions(),
	}
}

// Parse parses a markdown string
func (p *Parser) Parse(content string) (*Document, error) {
	doc := &Document{
		Content:  content,
		Sections: []Section{},
		Tags:     []string{},
	}

	// Extract frontmatter if enabled
	if p.options.ExtractFrontmatter {
		frontmatter, contentWithoutFrontmatter, err := extractFrontmatter([]byte(content))
		if err != nil {
			return nil, fmt.Errorf("failed to extract frontmatter: %w", err)
		}

		if frontmatter != nil {
			doc.Frontmatter = frontmatter
			content = string(contentWithoutFrontmatter)

			// Extract title from frontmatter if available
			if title, ok := frontmatter["title"]; ok {
				if titleStr, ok := title.(string); ok {
					doc.Title = titleStr
				}
			}

			// Extract tags from frontmatter if enabled
			if p.options.ExtractTags {
				if tags, ok := frontmatter["tags"]; ok {
					switch t := tags.(type) {
					case []interface{}:
						for _, tag := range t {
							if tagStr, ok := tag.(string); ok {
								doc.Tags = append(doc.Tags, tagStr)
							}
						}
					case string:
						doc.Tags = append(doc.Tags, t)
					}
				}
			}
		}
	}

	// Parse sections
	doc.Sections = parseSections([]byte(content))

	// Extract title from first heading if not found in frontmatter
	if doc.Title == "" && len(doc.Sections) > 0 && p.options.IncludeTitle {
		doc.Title = doc.Sections[0].Title
	}

	// Extract inline tags if enabled
	if p.options.ExtractTags {
		inlineTags := extractInlineTags([]byte(content))
		for _, tag := range inlineTags {
			if !contains(doc.Tags, tag) {
				doc.Tags = append(doc.Tags, tag)
			}
		}
	}

	return doc, nil
}

// ParseFile parses a markdown file
func (p *Parser) ParseFile(r io.Reader, path string) (*Document, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	doc, err := p.Parse(string(content))
	if err != nil {
		return nil, err
	}

	doc.Path = path
	return doc, nil
}

// ChunkByHeaders chunks a document by headers
func (p *Parser) ChunkByHeaders(doc *Document) []*Chunk {
	chunks := []*Chunk{}

	if len(doc.Sections) == 0 {
		return chunks
	}

	// Process each section
	for i, section := range doc.Sections {
		// Skip empty sections
		if len(strings.TrimSpace(section.Content)) == 0 {
			continue
		}

		// Create chunk
		chunk := &Chunk{
			ID:          fmt.Sprintf("%s:%d", doc.Path, i),
			Content:     section.Content,
			ContentOnly: section.Content, // Should filter out code blocks and other non-textual content
			Title:       doc.Title,
			Section:     section.Title,
			Tags:        doc.Tags,
			Path:        doc.Path,
			StartLine:   section.StartLine,
			EndLine:     section.EndLine,
			StartOffset: section.StartOffset,
			EndOffset:   section.EndOffset,
		}

		chunks = append(chunks, chunk)
	}

	return chunks
}

// ChunkBySlidingWindow chunks a document using a sliding window approach
func (p *Parser) ChunkBySlidingWindow(doc *Document, windowSize, overlap int) []*Chunk {
	chunks := []*Chunk{}

	// Get full text content
	text := doc.Content

	// Split into paragraphs
	paragraphs := strings.Split(text, "\n\n")

	// Apply sliding window chunking
	var currentChunk strings.Builder
	var currentSize int
	chunkIndex := 0

	for i, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}

		paragraphSize := len(paragraph)

		// If adding this paragraph would exceed max size, create a new chunk
		if currentSize+paragraphSize > windowSize && currentSize >= windowSize-overlap {
			// Create chunk
			chunk := &Chunk{
				ID:          fmt.Sprintf("%s:chunk_%d", doc.Path, chunkIndex),
				Content:     currentChunk.String(),
				ContentOnly: currentChunk.String(), // Should filter out code blocks and other non-textual content
				Title:       doc.Title,
				Tags:        doc.Tags,
				Path:        doc.Path,
			}

			chunks = append(chunks, chunk)
			chunkIndex++

			// Reset for next chunk with overlap
			currentChunk.Reset()
			currentSize = 0
		}

		// Add paragraph to current chunk
		if currentSize > 0 {
			currentChunk.WriteString("\n\n")
			currentSize += 2
		}
		currentChunk.WriteString(paragraph)
		currentSize += paragraphSize

		// If this is the last paragraph, add the remaining content as a chunk
		if i == len(paragraphs)-1 && currentSize > 0 {
			chunk := &Chunk{
				ID:          fmt.Sprintf("%s:chunk_%d", doc.Path, chunkIndex),
				Content:     currentChunk.String(),
				ContentOnly: currentChunk.String(), // Should filter out code blocks and other non-textual content
				Title:       doc.Title,
				Tags:        doc.Tags,
				Path:        doc.Path,
			}

			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// ChunkHybrid uses a hybrid approach combining headers and sliding windows
func (p *Parser) ChunkHybrid(doc *Document, windowSize, overlap int) []*Chunk {
	// First try header-based chunking
	headerChunks := p.ChunkByHeaders(doc)

	// Check if we need to further chunk any large sections
	var finalChunks []*Chunk

	for _, chunk := range headerChunks {
		// If chunk is smaller than max size, keep as is
		if len(chunk.Content) <= windowSize {
			finalChunks = append(finalChunks, chunk)
			continue
		}

		// Create a temporary document from this chunk
		tempDoc := &Document{
			Path:    doc.Path,
			Title:   chunk.Title,
			Content: chunk.Content,
			Tags:    chunk.Tags,
		}

		// Apply sliding window chunking to this large chunk
		subChunks := p.ChunkBySlidingWindow(tempDoc, windowSize, overlap)

		// Update IDs and section info for sub-chunks
		for i, subChunk := range subChunks {
			subChunk.ID = fmt.Sprintf("%s:%d", chunk.ID, i)
			subChunk.Section = chunk.Section
			subChunk.SectionPath = chunk.SectionPath
			finalChunks = append(finalChunks, subChunk)
		}
	}

	return finalChunks
}

// extractFrontmatter extracts YAML frontmatter from markdown content
func extractFrontmatter(content []byte) (map[string]interface{}, []byte, error) {
	frontmatterRegex := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n(.*)$`)
	matches := frontmatterRegex.FindSubmatch(content)

	if len(matches) != 3 {
		// No frontmatter found
		return nil, content, nil
	}

	frontmatterYAML := matches[1]
	remainingContent := matches[2]

	// Placeholder for frontmatter parsing
	// In a real implementation, we would use a YAML library to parse the frontmatter
	// For simplicity, we'll just create a dummy map
	frontmatter := make(map[string]interface{})

	// Parse simple key-value pairs
	scanner := bufio.NewScanner(bytes.NewReader(frontmatterYAML))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// Handle tags specially
			if key == "tags" {
				if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
					// Array format
					tagsStr := value[1 : len(value)-1]
					tags := []interface{}{}
					for _, tag := range strings.Split(tagsStr, ",") {
						tags = append(tags, strings.Trim(strings.TrimSpace(tag), "\"'"))
					}
					frontmatter[key] = tags
				} else {
					// Single tag
					frontmatter[key] = value
				}
			} else {
				frontmatter[key] = value
			}
		}
	}

	return frontmatter, remainingContent, nil
}

// parseSections parses markdown content into sections
func parseSections(content []byte) []Section {
	var sections []Section

	// Define regex for headers
	headerRegex := regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)

	// Find all headers
	matches := headerRegex.FindAllSubmatchIndex(content, -1)

	// Process headers and their content
	for i, match := range matches {
		headerStart := match[0]
		headerEnd := match[1]
		levelStart := match[2]
		levelEnd := match[3]
		titleStart := match[4]
		titleEnd := match[5]

		level := levelEnd - levelStart
		title := string(content[titleStart:titleEnd])

		// Determine content boundaries
		contentStart := headerEnd
		contentEnd := len(content)
		if i < len(matches)-1 {
			contentEnd = matches[i+1][0]
		}

		// Calculate line numbers
		startLine := bytes.Count(content[:headerStart], []byte{'\n'}) + 1
		endLine := bytes.Count(content[:contentEnd], []byte{'\n'}) + 1

		// Create section
		section := Section{
			Title:       title,
			Level:       level,
			Content:     string(content[contentStart:contentEnd]),
			StartLine:   startLine,
			EndLine:     endLine,
			StartOffset: headerStart,
			EndOffset:   contentEnd,
		}

		sections = append(sections, section)
	}

	// If no sections found, create a default section with the entire content
	if len(sections) == 0 {
		sections = append(sections, Section{
			Title:       "",
			Level:       0,
			Content:     string(content),
			StartLine:   1,
			EndLine:     bytes.Count(content, []byte{'\n'}) + 1,
			StartOffset: 0,
			EndOffset:   len(content),
		})
	}

	return sections
}

// extractInlineTags extracts tags in the format #tag from markdown
func extractInlineTags(content []byte) []string {
	tags := []string{}
	tagRegex := regexp.MustCompile(`(?:^|\s)#([a-zA-Z][a-zA-Z0-9_-]*)`)
	matches := tagRegex.FindAllSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 1 {
			tag := string(match[1])
			if !contains(tags, tag) {
				tags = append(tags, tag)
			}
		}
	}

	return tags
}

// contains checks if a string is in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ChunkOptions defines options for chunking
type ChunkOptions struct {
	Strategy            string
	MaxChunkSize        int
	MinChunkSize        int
	ChunkOverlap        int
	IncludeSectionTitle bool
	IncludeDocTitle     bool
}

// DefaultChunkOptions returns default chunking options
type ChunkStrategy struct {
	Name      string
	Options   ChunkOptions
	ChunkFunc func(*Document, ChunkOptions) ([]Chunk, error)
}

// NewHeaderChunkStrategy creates a header-based chunking strategy
func NewHeaderChunkStrategy() ChunkStrategy {
	return ChunkStrategy{
		Name: "header",
		Options: ChunkOptions{
			MaxChunkSize:        1000,
			MinChunkSize:        100,
			IncludeSectionTitle: true,
			IncludeDocTitle:     true,
		},
		ChunkFunc: headerBasedChunking,
	}
}

// NewSlidingWindowChunkStrategy creates a sliding window chunking strategy
func NewSlidingWindowChunkStrategy() ChunkStrategy {
	return ChunkStrategy{
		Name: "sliding_window",
		Options: ChunkOptions{
			MaxChunkSize:    500,
			MinChunkSize:    100,
			ChunkOverlap:    100,
			IncludeDocTitle: true,
		},
		ChunkFunc: slidingWindowChunking,
	}
}

// NewHybridChunkStrategy creates a hybrid chunking strategy
func NewHybridChunkStrategy() ChunkStrategy {
	return ChunkStrategy{
		Name: "hybrid",
		Options: ChunkOptions{
			Strategy:            "hybrid",
			MaxChunkSize:        1000,
			MinChunkSize:        100,
			ChunkOverlap:        100,
			IncludeSectionTitle: true,
			IncludeDocTitle:     true,
		},
		ChunkFunc: hybridChunking,
	}
}

// Chunk splits a document into chunks using the specified strategy
func (s *ChunkStrategy) Chunk(doc *Document) ([]Chunk, error) {
	return s.ChunkFunc(doc, s.Options)
}

// headerBasedChunking splits a document into chunks based on headers
func headerBasedChunking(doc *Document, options ChunkOptions) ([]Chunk, error) {
	chunks := []Chunk{}
	sectionPath := []string{}

	if len(doc.Sections) == 0 {
		return chunks, nil
	}

	// Add document title to section path if specified
	if options.IncludeDocTitle && doc.Title != "" {
		sectionPath = append(sectionPath, doc.Title)
	}

	for i, section := range doc.Sections {
		// Update section path based on header level
		level := section.Level

		// Skip if level is 0 (no header)
		if level > 0 {
			// Truncate section path if going up in the hierarchy
			if len(sectionPath) >= level {
				sectionPath = sectionPath[:level]
			}

			// Add current section title to path
			if len(sectionPath) < level {
				sectionPath = append(sectionPath, section.Title)
			} else {
				sectionPath[level-1] = section.Title
			}
		}

		// Skip empty sections
		if len(strings.TrimSpace(section.Content)) == 0 {
			continue
		}

		// Create chunk ID
		chunkID := fmt.Sprintf("%s:%d", doc.Path, i)

		// Determine section title path
		var sectionTitle string
		if len(sectionPath) > 0 {
			sectionTitle = strings.Join(sectionPath, " > ")
		}

		// Create chunk
		chunk := Chunk{
			ID:          chunkID,
			Content:     section.Content,
			Title:       doc.Title,
			Section:     section.Title,
			SectionPath: sectionTitle,
			Tags:        doc.Tags,
			Path:        doc.Path,
			StartLine:   section.StartLine,
			EndLine:     section.EndLine,
			StartOffset: section.StartOffset,
			EndOffset:   section.EndOffset,
		}

		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// slidingWindowChunking splits a document into chunks using sliding window
func slidingWindowChunking(doc *Document, options ChunkOptions) ([]Chunk, error) {
	chunks := []Chunk{}

	// Get full text content
	text := doc.Content

	// Split into paragraphs
	paragraphs := strings.Split(text, "\n\n")

	// Apply sliding window chunking
	var currentChunk strings.Builder
	var currentSize int
	chunkIndex := 0

	for i, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}

		paragraphSize := len(paragraph)

		// If adding this paragraph would exceed max size, create a new chunk
		if currentSize+paragraphSize > options.MaxChunkSize && currentSize >= options.MinChunkSize {
			// Create chunk
			chunk := Chunk{
				ID:      fmt.Sprintf("%s:chunk_%d", doc.Path, chunkIndex),
				Content: currentChunk.String(),
				Title:   doc.Title,
				Tags:    doc.Tags,
				Path:    doc.Path,
				// Line numbers and offsets would require more precise tracking
			}

			chunks = append(chunks, chunk)
			chunkIndex++

			// Reset for next chunk, potentially with overlap
			// For simplicity, we're not implementing the overlap here
			currentChunk.Reset()
			currentSize = 0
		}

		// Add paragraph to current chunk
		if currentSize > 0 {
			currentChunk.WriteString("\n\n")
			currentSize += 2
		}
		currentChunk.WriteString(paragraph)
		currentSize += paragraphSize

		// If this is the last paragraph, add the remaining content as a chunk
		if i == len(paragraphs)-1 && currentSize > 0 {
			chunk := Chunk{
				ID:      fmt.Sprintf("%s:chunk_%d", doc.Path, chunkIndex),
				Content: currentChunk.String(),
				Title:   doc.Title,
				Tags:    doc.Tags,
				Path:    doc.Path,
				// Line numbers and offsets would require more precise tracking
			}

			chunks = append(chunks, chunk)
		}
	}

	return chunks, nil
}

// hybridChunking uses header-based chunking but falls back to sliding window
// for large sections
func hybridChunking(doc *Document, options ChunkOptions) ([]Chunk, error) {
	// First try header-based chunking
	headerChunks, err := headerBasedChunking(doc, options)
	if err != nil {
		return nil, err
	}

	// Check if we need to further chunk any large sections
	var finalChunks []Chunk

	for _, chunk := range headerChunks {
		// If chunk is smaller than max size, keep as is
		if len(chunk.Content) <= options.MaxChunkSize {
			finalChunks = append(finalChunks, chunk)
			continue
		}

		// Create a temporary document from this chunk
		tempDoc := &Document{
			Path:    doc.Path,
			Title:   chunk.Title,
			Content: chunk.Content,
			Tags:    chunk.Tags,
		}

		// Apply sliding window chunking to this large chunk
		subChunks, err := slidingWindowChunking(tempDoc, options)
		if err != nil {
			return nil, err
		}

		// Update IDs and section info for sub-chunks
		for i, subChunk := range subChunks {
			subChunk.ID = fmt.Sprintf("%s:%d", chunk.ID, i)
			subChunk.Section = chunk.Section
			subChunk.SectionPath = chunk.SectionPath
			finalChunks = append(finalChunks, subChunk)
		}
	}

	return finalChunks, nil
}
