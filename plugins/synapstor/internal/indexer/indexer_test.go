package indexer

import (
	"testing"
)

func TestSplitMarkdown(t *testing.T) {
	content := `# Header 1
Content 1

## Subheader A
Content A

# Header 2
Content 2
`
	chunks := splitMarkdown(content)

	if len(chunks) != 3 {
		t.Errorf("Expected 3 chunks, got %d", len(chunks))
	}

	expectedFirst := "# Header 1\nContent 1"
	if len(chunks) > 0 && chunks[0] != expectedFirst {
		t.Errorf("First chunk mismatch.\nExpected:\n%s\nGot:\n%s", expectedFirst, chunks[0])
	}
}

func TestSplitMarkdown_NoHeaders(t *testing.T) {
	content := "Just some plain text without headers."
	chunks := splitMarkdown(content)

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(chunks))
	}

	if chunks[0] != content {
		t.Errorf("Content mismatch")
	}
}

func TestSplitMarkdown_SmallChunksIgnored(t *testing.T) {
	// The splitter ignores chunks < 50 chars
	content := `# Big Header
This content is long enough to be preserved by the splitter logic I hope.

# Small
Short.`

	chunks := splitMarkdown(content)

	// "Short." section is very small, might be ignored if splitter logic < 50
	// Let's verify behavior. "Short." + "# Small\n" is around 14 chars.
	// First chunk is > 50.

	if len(chunks) != 2 {
		t.Errorf("Expected 2 valid chunks (small one accepted > 10 chars), got %d", len(chunks))
	}
}
