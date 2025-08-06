package llm

import (
	"fmt"
	"strings"
)

// TextSplitter handles splitting text into chunks while preserving formatting
type TextSplitter struct {
	MaxLength     int
	PreserveLines bool
}

// NewTextSplitter creates a new text splitter with default settings
func NewTextSplitter(maxLength int) *TextSplitter {
	return &TextSplitter{
		MaxLength:     maxLength,
		PreserveLines: true,
	}
}

// SplitText splits text into chunks while preserving line breaks and formatting
func (ts *TextSplitter) SplitText(content string) []string {
	if len(content) <= ts.MaxLength {
		return []string{content}
	}

	if !ts.PreserveLines {
		// Simple character-based splitting
		return ts.splitByCharacters(content)
	}

	// Split by lines first to avoid breaking in the middle of sentences
	lines := strings.Split(content, "\n")
	var chunks []string
	var currentChunk strings.Builder

	for _, line := range lines {
		// If adding this line would exceed the limit, start a new chunk
		if currentChunk.Len()+len(line)+1 > ts.MaxLength {
			if currentChunk.Len() > 0 {
				chunks = append(chunks, currentChunk.String())
				currentChunk.Reset()
			}

			// Handle very long lines that exceed maxLength
			if len(line) > ts.MaxLength {
				longLineChunks := ts.splitLongLine(line)
				chunks = append(chunks, longLineChunks[:len(longLineChunks)-1]...)
				// Start new chunk with the last piece
				currentChunk.WriteString(longLineChunks[len(longLineChunks)-1])
				continue
			}
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n")
		}
		currentChunk.WriteString(line)
	}

	// Add the final chunk
	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}

// splitByCharacters splits text by character count without preserving lines
func (ts *TextSplitter) splitByCharacters(content string) []string {
	var chunks []string
	for len(content) > ts.MaxLength {
		chunks = append(chunks, content[:ts.MaxLength])
		content = content[ts.MaxLength:]
	}
	if len(content) > 0 {
		chunks = append(chunks, content)
	}
	return chunks
}

// splitLongLine splits a single line that exceeds maxLength
func (ts *TextSplitter) splitLongLine(line string) []string {
	var chunks []string
	for len(line) > ts.MaxLength {
		chunks = append(chunks, line[:ts.MaxLength])
		line = line[ts.MaxLength:]
	}
	if len(line) > 0 {
		chunks = append(chunks, line)
	}
	return chunks
}

// SplitTextWithParts splits text and adds part indicators to each chunk
func (ts *TextSplitter) SplitTextWithParts(content string) []string {
	chunks := ts.SplitText(content)

	if len(chunks) <= 1 {
		return chunks
	}

	// Add part indicators to multi-part messages
	for i := range chunks {
		if i > 0 {
			chunks[i] = fmt.Sprintf("(Part %d/%d)\n%s", i+1, len(chunks), chunks[i])
		}
	}

	return chunks
}

// TruncateText truncates text to maxLength and adds ellipsis if needed
func TruncateText(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}

	if maxLength <= 0 {
		return ""
	}

	if maxLength <= 3 {
		return content[:maxLength]
	}

	return content[:maxLength-3] + "..."
}

// TruncateTextWithNotice truncates text and adds a custom notice
func TruncateTextWithNotice(content string, maxLength int, notice string) string {
	if len(content) <= maxLength {
		return content
	}

	noticeLen := len(notice)
	if maxLength <= noticeLen {
		return notice[:maxLength]
	}

	return content[:maxLength-noticeLen] + notice
}
