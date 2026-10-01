package rlm

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteBackend is a SQLite-based StoreBackend implementation.
type SQLiteBackend struct {
	db *sql.DB
}

// NewSQLiteBackend creates a new SQLite backend and runs schema migrations.
func NewSQLiteBackend(dbPath string) (*SQLiteBackend, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	backend := &SQLiteBackend{db: db}

	if _, err := backend.db.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		_ = backend.db.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	if _, err := backend.db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		_ = backend.db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := backend.migrate(); err != nil {
		_ = backend.db.Close()
		return nil, err
	}

	return backend, nil
}

func (b *SQLiteBackend) migrate() error {
	if _, err := b.db.Exec(`
CREATE TABLE IF NOT EXISTS messages (
	id TEXT PRIMARY KEY,
	role TEXT,
	content TEXT,
	tokens INTEGER,
	timestamp TEXT,
	file_ids TEXT DEFAULT '[]',
	metadata TEXT DEFAULT '{}'
);
`); err != nil {
		return fmt.Errorf("create messages table: %w", err)
	}

	if _, err := b.db.Exec(`
CREATE INDEX IF NOT EXISTS idx_messages_role ON messages(role);
`); err != nil {
		return fmt.Errorf("create messages role index: %w", err)
	}

	if _, err := b.db.Exec(`
CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);
`); err != nil {
		return fmt.Errorf("create messages timestamp index: %w", err)
	}

	if _, err := b.db.Exec(`
CREATE TABLE IF NOT EXISTS summaries (
	id TEXT PRIMARY KEY,
	kind TEXT,
	content TEXT,
	tokens INTEGER,
	level INTEGER,
	created_at TEXT,
	message_ids TEXT DEFAULT '[]',
	child_ids TEXT DEFAULT '[]',
	parent_id TEXT DEFAULT '',
	file_ids TEXT DEFAULT '[]'
);
`); err != nil {
		return fmt.Errorf("create summaries table: %w", err)
	}

	if _, err := b.db.Exec(`
CREATE INDEX IF NOT EXISTS idx_summaries_kind ON summaries(kind);
`); err != nil {
		return fmt.Errorf("create summaries kind index: %w", err)
	}

	if _, err := b.db.Exec(`
CREATE INDEX IF NOT EXISTS idx_summaries_parent_id ON summaries(parent_id);
`); err != nil {
		return fmt.Errorf("create summaries parent_id index: %w", err)
	}

	if _, err := b.db.Exec(`
CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
	id,
	content,
	content='messages',
	content_rowid='rowid'
);
`); err != nil {
		return fmt.Errorf("create messages_fts table: %w", err)
	}

	if _, err := b.db.Exec(`
CREATE TRIGGER IF NOT EXISTS messages_ai
AFTER INSERT ON messages
BEGIN
	INSERT INTO messages_fts(rowid, id, content)
	VALUES (new.rowid, new.id, new.content);
END;
`); err != nil {
		return fmt.Errorf("create messages_ai trigger: %w", err)
	}

	return nil
}

func (b *SQLiteBackend) PersistMessage(msg *StoreMessage) error {
	if msg == nil {
		return fmt.Errorf("message is nil")
	}

	fileIDsJSON, err := marshalJSON(msg.FileIDs, "[]")
	if err != nil {
		return fmt.Errorf("marshal message file_ids: %w", err)
	}

	metadataJSON, err := marshalJSON(msg.Metadata, "{}")
	if err != nil {
		return fmt.Errorf("marshal message metadata: %w", err)
	}

	timestamp := msg.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	_, err = b.db.Exec(`
INSERT OR REPLACE INTO messages (id, role, content, tokens, timestamp, file_ids, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?);
`, msg.ID, string(msg.Role), msg.Content, msg.Tokens, timestamp.Format(time.RFC3339Nano), fileIDsJSON, metadataJSON)
	if err != nil {
		return fmt.Errorf("persist message %s: %w", msg.ID, err)
	}

	return nil
}

func (b *SQLiteBackend) GetMessage(id string) (*StoreMessage, error) {
	row := b.db.QueryRow(`
SELECT id, role, content, tokens, timestamp, file_ids, metadata
FROM messages
WHERE id = ?;
`, id)

	msg, err := scanMessage(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get message %s: %w", id, err)
	}

	return msg, nil
}

func (b *SQLiteBackend) GetAllMessages() ([]*StoreMessage, error) {
	rows, err := b.db.Query(`
SELECT id, role, content, tokens, timestamp, file_ids, metadata
FROM messages
ORDER BY timestamp ASC;
`)
	if err != nil {
		return nil, fmt.Errorf("query all messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	messages := make([]*StoreMessage, 0)
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate all messages: %w", err)
	}

	return messages, nil
}

func (b *SQLiteBackend) MessageCount() (int, error) {
	var count int
	if err := b.db.QueryRow(`SELECT COUNT(*) FROM messages;`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count messages: %w", err)
	}
	return count, nil
}

func (b *SQLiteBackend) PersistSummary(node *SummaryNode) error {
	if node == nil {
		return fmt.Errorf("summary node is nil")
	}

	messageIDsJSON, err := marshalJSON(node.MessageIDs, "[]")
	if err != nil {
		return fmt.Errorf("marshal summary message_ids: %w", err)
	}

	childIDsJSON, err := marshalJSON(node.ChildIDs, "[]")
	if err != nil {
		return fmt.Errorf("marshal summary child_ids: %w", err)
	}

	fileIDsJSON, err := marshalJSON(node.FileIDs, "[]")
	if err != nil {
		return fmt.Errorf("marshal summary file_ids: %w", err)
	}

	createdAt := node.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	_, err = b.db.Exec(`
INSERT OR REPLACE INTO summaries (id, kind, content, tokens, level, created_at, message_ids, child_ids, parent_id, file_ids)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
`, node.ID, string(node.Kind), node.Content, node.Tokens, node.Level, createdAt.Format(time.RFC3339Nano), messageIDsJSON, childIDsJSON, node.ParentID, fileIDsJSON)
	if err != nil {
		return fmt.Errorf("persist summary %s: %w", node.ID, err)
	}

	return nil
}

func (b *SQLiteBackend) GetSummary(id string) (*SummaryNode, error) {
	row := b.db.QueryRow(`
SELECT id, kind, content, tokens, level, created_at, message_ids, child_ids, parent_id, file_ids
FROM summaries
WHERE id = ?;
`, id)

	summary, err := scanSummary(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get summary %s: %w", id, err)
	}

	return summary, nil
}

func (b *SQLiteBackend) GetAllSummaries() ([]*SummaryNode, error) {
	rows, err := b.db.Query(`
SELECT id, kind, content, tokens, level, created_at, message_ids, child_ids, parent_id, file_ids
FROM summaries
ORDER BY created_at ASC;
`)
	if err != nil {
		return nil, fmt.Errorf("query all summaries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	summaries := make([]*SummaryNode, 0)
	for rows.Next() {
		summary, err := scanSummary(rows)
		if err != nil {
			return nil, fmt.Errorf("scan summary: %w", err)
		}
		summaries = append(summaries, summary)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate all summaries: %w", err)
	}

	return summaries, nil
}

func (b *SQLiteBackend) UpdateSummaryParent(summaryID, parentID string) error {
	_, err := b.db.Exec(`
UPDATE summaries
SET parent_id = ?
WHERE id = ?;
`, parentID, summaryID)
	if err != nil {
		return fmt.Errorf("update summary parent for %s: %w", summaryID, err)
	}
	return nil
}

func (b *SQLiteBackend) GrepMessages(pattern string, summaryScope *string, maxResults int) ([]*StoreMessage, error) {
	if maxResults <= 0 {
		maxResults = 100
	}

	scopeSet, err := b.scopeMessageSet(summaryScope)
	if err != nil {
		return nil, err
	}

	if isSimpleFTSPattern(pattern) {
		ftsQuery := "\"" + strings.ReplaceAll(pattern, "\"", "\"\"") + "\""
		rows, ftsErr := b.db.Query(`
SELECT m.id, m.role, m.content, m.tokens, m.timestamp, m.file_ids, m.metadata
FROM messages_fts f
JOIN messages m ON m.rowid = f.rowid
WHERE messages_fts MATCH ?
ORDER BY m.timestamp ASC;
`, ftsQuery)
		if ftsErr == nil {
			defer func() { _ = rows.Close() }()

			results := make([]*StoreMessage, 0, maxResults)
			for rows.Next() {
				msg, err := scanMessage(rows)
				if err != nil {
					return nil, fmt.Errorf("scan fts message: %w", err)
				}
				if scopeSet != nil {
					if _, ok := scopeSet[msg.ID]; !ok {
						continue
					}
				}
				results = append(results, msg)
				if len(results) >= maxResults {
					break
				}
			}
			if err := rows.Err(); err != nil {
				return nil, fmt.Errorf("iterate fts messages: %w", err)
			}
			return results, nil
		}
	}

	// Fallback: filter in Go with regex over stored messages.
	messages, err := b.GetAllMessages()
	if err != nil {
		return nil, err
	}

	regexPattern := pattern
	if isSimpleFTSPattern(pattern) {
		regexPattern = regexp.QuoteMeta(pattern)
	}

	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return nil, fmt.Errorf("compile grep regex %q: %w", pattern, err)
	}

	results := make([]*StoreMessage, 0, maxResults)
	for _, msg := range messages {
		if scopeSet != nil {
			if _, ok := scopeSet[msg.ID]; !ok {
				continue
			}
		}
		if !re.MatchString(msg.Content) {
			continue
		}
		results = append(results, msg)
		if len(results) >= maxResults {
			break
		}
	}

	return results, nil
}

func (b *SQLiteBackend) Close() error {
	if b == nil || b.db == nil {
		return nil
	}
	return b.db.Close()
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanMessage(scanner rowScanner) (*StoreMessage, error) {
	var (
		msg         StoreMessage
		role        string
		timestamp   string
		fileIDsJSON string
		metadataJSON string
	)

	if err := scanner.Scan(
		&msg.ID,
		&role,
		&msg.Content,
		&msg.Tokens,
		&timestamp,
		&fileIDsJSON,
		&metadataJSON,
	); err != nil {
		return nil, err
	}

	msg.Role = MessageRole(role)
	parsedTime, err := parseTime(timestamp)
	if err != nil {
		return nil, fmt.Errorf("parse message timestamp: %w", err)
	}
	msg.Timestamp = parsedTime

	if err := unmarshalStringSlice(fileIDsJSON, &msg.FileIDs); err != nil {
		return nil, fmt.Errorf("unmarshal message file_ids: %w", err)
	}
	if err := unmarshalStringMap(metadataJSON, &msg.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal message metadata: %w", err)
	}

	return &msg, nil
}

func scanSummary(scanner rowScanner) (*SummaryNode, error) {
	var (
		node          SummaryNode
		kind          string
		createdAt     string
		messageIDsJSON string
		childIDsJSON   string
		fileIDsJSON    string
	)

	if err := scanner.Scan(
		&node.ID,
		&kind,
		&node.Content,
		&node.Tokens,
		&node.Level,
		&createdAt,
		&messageIDsJSON,
		&childIDsJSON,
		&node.ParentID,
		&fileIDsJSON,
	); err != nil {
		return nil, err
	}

	node.Kind = SummaryKind(kind)
	parsedTime, err := parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse summary created_at: %w", err)
	}
	node.CreatedAt = parsedTime

	if err := unmarshalStringSlice(messageIDsJSON, &node.MessageIDs); err != nil {
		return nil, fmt.Errorf("unmarshal summary message_ids: %w", err)
	}
	if err := unmarshalStringSlice(childIDsJSON, &node.ChildIDs); err != nil {
		return nil, fmt.Errorf("unmarshal summary child_ids: %w", err)
	}
	if err := unmarshalStringSlice(fileIDsJSON, &node.FileIDs); err != nil {
		return nil, fmt.Errorf("unmarshal summary file_ids: %w", err)
	}

	return &node, nil
}

func marshalJSON(v interface{}, defaultJSON string) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	if string(data) == "null" {
		return defaultJSON, nil
	}
	return string(data), nil
}

func unmarshalStringSlice(raw string, out *[]string) error {
	if strings.TrimSpace(raw) == "" {
		*out = []string{}
		return nil
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return err
	}
	if *out == nil {
		*out = []string{}
	}
	return nil
}

func unmarshalStringMap(raw string, out *map[string]string) error {
	if strings.TrimSpace(raw) == "" {
		*out = map[string]string{}
		return nil
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return err
	}
	if *out == nil {
		*out = map[string]string{}
	}
	return nil
}

func parseTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if ts, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return ts, nil
	}
	return time.Parse(time.RFC3339, value)
}

func isSimpleFTSPattern(pattern string) bool {
	if strings.TrimSpace(pattern) == "" {
		return false
	}
	// If regex metacharacters appear, skip FTS and use Go regex fallback.
	meta := regexp.MustCompile(`[\\.\^\$\|\?\*\+\(\)\{\}\[\]]`)
	return !meta.MatchString(pattern)
}

func (b *SQLiteBackend) scopeMessageSet(summaryScope *string) (map[string]struct{}, error) {
	if summaryScope == nil || strings.TrimSpace(*summaryScope) == "" {
		return nil, nil
	}

	visited := map[string]struct{}{}
	messageSet := map[string]struct{}{}

	if err := b.collectScopedMessages(*summaryScope, visited, messageSet); err != nil {
		return nil, err
	}

	return messageSet, nil
}

func (b *SQLiteBackend) collectScopedMessages(summaryID string, visited map[string]struct{}, messageSet map[string]struct{}) error {
	if _, ok := visited[summaryID]; ok {
		return nil
	}
	visited[summaryID] = struct{}{}

	summary, err := b.GetSummary(summaryID)
	if err != nil {
		return fmt.Errorf("collect scoped messages for %s: %w", summaryID, err)
	}
	if summary == nil {
		return nil
	}

	for _, id := range summary.MessageIDs {
		messageSet[id] = struct{}{}
	}
	for _, childID := range summary.ChildIDs {
		if err := b.collectScopedMessages(childID, visited, messageSet); err != nil {
			return err
		}
	}

	return nil
}
