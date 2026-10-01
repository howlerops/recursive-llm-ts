package rlm

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// ─── LCM Large File Handling ────────────────────────────────────────────────
// Implements the file handling strategy from the LCM paper (Section 2.2):
// - Files below token threshold: included in context normally
// - Files above threshold: stored externally with Exploration Summary
// - Type-aware dispatching for summary generation

// LCMFileConfig configures large file handling.
type LCMFileConfig struct {
	// TokenThreshold is the token count above which files are stored externally.
	// Default: 25000 (25k tokens, matching the LCM paper).
	TokenThreshold int `json:"token_threshold,omitempty"`
}

// DefaultLCMFileConfig returns default file handling configuration.
func DefaultLCMFileConfig() LCMFileConfig {
	return LCMFileConfig{
		TokenThreshold: 25000,
	}
}

// LCMFileRef is an opaque reference to a file stored outside the active context.
type LCMFileRef struct {
	ID                string `json:"id"`
	Path              string `json:"path"`
	MIMEType          string `json:"mime_type"`
	Tokens            int    `json:"tokens"`
	ExplorationSummary string `json:"exploration_summary"`
}

// LCMFileHandler manages large file references and exploration summaries.
type LCMFileHandler struct {
	config   LCMFileConfig
	files    map[string]*LCMFileRef // File refs by ID
	nextID   int
	observer *Observer

	// LLM config for generating exploration summaries
	model       string
	apiBase     string
	apiKey      string
	timeout     int
	extraParams map[string]interface{}
}

// NewLCMFileHandler creates a new file handler.
func NewLCMFileHandler(config LCMFileConfig, model, apiBase, apiKey string, timeout int, extraParams map[string]interface{}, observer *Observer) *LCMFileHandler {
	if config.TokenThreshold == 0 {
		config.TokenThreshold = 25000
	}
	return &LCMFileHandler{
		config:      config,
		files:       make(map[string]*LCMFileRef),
		observer:    observer,
		model:       model,
		apiBase:     apiBase,
		apiKey:      apiKey,
		timeout:     timeout,
		extraParams: extraParams,
	}
}

// ProcessFile checks if a file should be included in context or stored externally.
// Returns (contextContent, fileRef, error).
// If the file is small enough, contextContent is the file content and fileRef is nil.
// If the file is too large, contextContent is the exploration summary reference and fileRef is set.
func (h *LCMFileHandler) ProcessFile(path string, content string) (string, *LCMFileRef, error) {
	tokens := EstimateTokens(content)

	if tokens <= h.config.TokenThreshold {
		// Small file: include in context normally
		return content, nil, nil
	}

	// Large file: generate exploration summary
	h.observer.Debug("lcm.files", "File %s (%d tokens) exceeds threshold (%d), generating exploration summary",
		path, tokens, h.config.TokenThreshold)

	summary, err := h.generateExplorationSummary(path, content)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate exploration summary for %s: %w", path, err)
	}

	h.nextID++
	ref := &LCMFileRef{
		ID:                 fmt.Sprintf("file_%d", h.nextID),
		Path:               path,
		MIMEType:           detectMIMEType(path),
		Tokens:             tokens,
		ExplorationSummary: summary,
	}
	h.files[ref.ID] = ref

	// Return a compact reference for the active context
	contextRef := fmt.Sprintf("[File %s: %s (%d tokens)]\n%s", ref.ID, path, tokens, summary)
	return contextRef, ref, nil
}

// GetFileRef retrieves a file reference by ID.
func (h *LCMFileHandler) GetFileRef(id string) (*LCMFileRef, bool) {
	ref, ok := h.files[id]
	return ref, ok
}

// ─── Type-Aware Exploration Summary Generation ──────────────────────────────

func (h *LCMFileHandler) generateExplorationSummary(path string, content string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	fileType := classifyFileType(ext)

	switch fileType {
	case fileTypeStructuredData:
		return h.summarizeStructuredData(path, content, ext)
	case fileTypeCode:
		return h.summarizeCode(path, content, ext)
	default:
		return h.summarizeText(path, content)
	}
}

type fileType int

const (
	fileTypeText           fileType = iota
	fileTypeCode
	fileTypeStructuredData
)

func classifyFileType(ext string) fileType {
	switch ext {
	case ".json", ".jsonl", ".csv", ".tsv", ".sql", ".sqlite", ".db",
		".xml", ".yaml", ".yml", ".toml", ".parquet", ".avro":
		return fileTypeStructuredData
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java",
		".c", ".cpp", ".h", ".hpp", ".cs", ".rb", ".php", ".swift",
		".kt", ".scala", ".sh", ".bash", ".zsh", ".lua", ".r", ".R":
		return fileTypeCode
	default:
		return fileTypeText
	}
}

// summarizeStructuredData extracts schema and shape for JSON, CSV, SQL, etc.
func (h *LCMFileHandler) summarizeStructuredData(path string, content string, ext string) (string, error) {
	// For structured data, try to extract schema deterministically first
	switch ext {
	case ".json":
		return h.summarizeJSON(content), nil
	case ".jsonl":
		return h.summarizeJSONL(content), nil
	case ".csv", ".tsv":
		return h.summarizeCSV(content, ext), nil
	default:
		// Fall back to LLM summary for other structured formats
		return h.llmSummarize(path, content, "structured data")
	}
}

func (h *LCMFileHandler) summarizeJSON(content string) string {
	var parsed interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return fmt.Sprintf("JSON file (parse error: %s)", err)
	}

	var sb strings.Builder
	sb.WriteString("JSON file analysis:\n")
	describeJSONShape(&sb, parsed, "", 0, 3)
	return sb.String()
}

func describeJSONShape(sb *strings.Builder, v interface{}, prefix string, depth int, maxDepth int) {
	if depth > maxDepth {
		sb.WriteString(prefix + "...\n")
		return
	}

	switch val := v.(type) {
	case map[string]interface{}:
		fmt.Fprintf(sb, "%sObject with %d keys: ", prefix, len(val))
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		if len(keys) > 10 {
			fmt.Fprintf(sb, "%s... (%d more)\n", strings.Join(keys[:10], ", "), len(keys)-10)
		} else {
			sb.WriteString(strings.Join(keys, ", ") + "\n")
		}
		for k, child := range val {
			if depth < maxDepth-1 {
				describeJSONShape(sb, child, prefix+"  "+k+": ", depth+1, maxDepth)
			}
		}
	case []interface{}:
		fmt.Fprintf(sb, "%sArray with %d items", prefix, len(val))
		if len(val) > 0 {
			fmt.Fprintf(sb, " (first item type: %T)\n", val[0])
			describeJSONShape(sb, val[0], prefix+"  [0]: ", depth+1, maxDepth)
		} else {
			sb.WriteString(" (empty)\n")
		}
	default:
		fmt.Fprintf(sb, "%s%T\n", prefix, v)
	}
}

func (h *LCMFileHandler) summarizeJSONL(content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var sb strings.Builder
	fmt.Fprintf(&sb, "JSONL file: %d lines\n", len(lines))

	// Analyze first line for schema
	if len(lines) > 0 {
		var first interface{}
		if err := json.Unmarshal([]byte(lines[0]), &first); err == nil {
			sb.WriteString("Schema (from first line):\n")
			describeJSONShape(&sb, first, "  ", 0, 2)
		}
	}

	return sb.String()
}

func (h *LCMFileHandler) summarizeCSV(content string, ext string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var sb strings.Builder

	delimiter := ","
	if ext == ".tsv" {
		delimiter = "\t"
	}

	fmt.Fprintf(&sb, "CSV file: %d rows\n", len(lines))
	if len(lines) > 0 {
		headers := strings.Split(lines[0], delimiter)
		fmt.Fprintf(&sb, "Columns (%d): %s\n", len(headers), strings.Join(headers, ", "))
	}
	if len(lines) > 1 {
		fmt.Fprintf(&sb, "Sample row: %s\n", lines[1])
	}

	return sb.String()
}

// summarizeCode extracts structural analysis for code files.
func (h *LCMFileHandler) summarizeCode(path string, content string, ext string) (string, error) {
	// Deterministic structural analysis
	summary := extractCodeStructure(content, ext)
	if summary != "" {
		return summary, nil
	}
	// Fall back to LLM
	return h.llmSummarize(path, content, "source code")
}

// extractCodeStructure does basic structural analysis without LLM.
func extractCodeStructure(content string, ext string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	fmt.Fprintf(&sb, "Code file: %d lines\n", len(lines))

	// Extract function/class/struct definitions
	var defs []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isDefinitionLine(trimmed, ext) {
			defs = append(defs, trimmed)
		}
	}

	if len(defs) > 0 {
		fmt.Fprintf(&sb, "Definitions (%d):\n", len(defs))
		for _, d := range defs {
			if len(d) > 120 {
				d = d[:120] + "..."
			}
			sb.WriteString("  " + d + "\n")
		}
	}

	// Extract imports
	var imports []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isImportLine(trimmed, ext) {
			imports = append(imports, trimmed)
		}
	}
	if len(imports) > 0 {
		fmt.Fprintf(&sb, "Imports (%d):\n", len(imports))
		max := 20
		if len(imports) < max {
			max = len(imports)
		}
		for _, imp := range imports[:max] {
			sb.WriteString("  " + imp + "\n")
		}
		if len(imports) > max {
			fmt.Fprintf(&sb, "  ... and %d more\n", len(imports)-max)
		}
	}

	return sb.String()
}

func isDefinitionLine(line string, ext string) bool {
	switch ext {
	case ".go":
		return strings.HasPrefix(line, "func ") || strings.HasPrefix(line, "type ") ||
			strings.HasPrefix(line, "var ") || strings.HasPrefix(line, "const ")
	case ".py":
		return strings.HasPrefix(line, "def ") || strings.HasPrefix(line, "class ") ||
			strings.HasPrefix(line, "async def ")
	case ".ts", ".tsx", ".js", ".jsx":
		return strings.HasPrefix(line, "function ") || strings.HasPrefix(line, "class ") ||
			strings.HasPrefix(line, "export ") || strings.HasPrefix(line, "interface ") ||
			strings.HasPrefix(line, "type ") || strings.HasPrefix(line, "const ")
	case ".rs":
		return strings.HasPrefix(line, "fn ") || strings.HasPrefix(line, "pub fn ") ||
			strings.HasPrefix(line, "struct ") || strings.HasPrefix(line, "enum ") ||
			strings.HasPrefix(line, "impl ") || strings.HasPrefix(line, "trait ")
	case ".java", ".kt", ".scala":
		return strings.HasPrefix(line, "public ") || strings.HasPrefix(line, "private ") ||
			strings.HasPrefix(line, "class ") || strings.HasPrefix(line, "interface ")
	default:
		return strings.HasPrefix(line, "function ") || strings.HasPrefix(line, "class ") ||
			strings.HasPrefix(line, "def ")
	}
}

func isImportLine(line string, ext string) bool {
	switch ext {
	case ".go":
		return strings.HasPrefix(line, "import ") || (strings.HasPrefix(line, "\"") && strings.HasSuffix(line, "\""))
	case ".py":
		return strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "from ")
	case ".ts", ".tsx", ".js", ".jsx":
		return strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "require(")
	case ".rs":
		return strings.HasPrefix(line, "use ")
	case ".java":
		return strings.HasPrefix(line, "import ")
	default:
		return strings.HasPrefix(line, "import ")
	}
}

// summarizeText generates an LLM summary for unstructured text.
func (h *LCMFileHandler) summarizeText(path string, content string) (string, error) {
	return h.llmSummarize(path, content, "text document")
}

func (h *LCMFileHandler) llmSummarize(path string, content string, fileType string) (string, error) {
	// Truncate content for the summary prompt (we don't need the whole file)
	maxChars := 10000
	truncated := content
	if len(content) > maxChars {
		truncated = content[:maxChars/2] + "\n...\n" + content[len(content)-maxChars/2:]
	}

	prompt := fmt.Sprintf(`Generate a concise exploration summary for this %s file (%s).
Include: structure, key entities, purpose, and notable patterns.
Keep it under 200 words.

Content:
%s`, fileType, path, truncated)

	request := ChatRequest{
		Model: h.model,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
		APIBase:     h.apiBase,
		APIKey:      h.apiKey,
		Timeout:     h.timeout,
		ExtraParams: h.extraParams,
	}

	result, err := CallChatCompletion(request)
	if err != nil {
		// Fall back to deterministic summary on LLM failure
		lines := strings.Split(content, "\n")
		return fmt.Sprintf("%s file: %d lines, %d tokens", fileType, len(lines), EstimateTokens(content)), nil
	}

	return result.Content, nil
}

// detectMIMEType returns a MIME type based on file extension.
func detectMIMEType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	mimeTypes := map[string]string{
		".json":  "application/json",
		".jsonl": "application/x-jsonlines",
		".csv":   "text/csv",
		".tsv":   "text/tab-separated-values",
		".sql":   "application/sql",
		".xml":   "application/xml",
		".yaml":  "application/yaml",
		".yml":   "application/yaml",
		".go":    "text/x-go",
		".ts":    "text/typescript",
		".js":    "text/javascript",
		".py":    "text/x-python",
		".rs":    "text/x-rust",
		".java":  "text/x-java",
		".md":    "text/markdown",
		".txt":   "text/plain",
		".html":  "text/html",
		".css":   "text/css",
	}
	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}
