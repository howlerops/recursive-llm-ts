package rlm

import (
	"strings"
	"testing"
	"time"
)

func testStoreMessage(id string, role MessageRole, content string, tokens int, ts time.Time, fileIDs []string) *StoreMessage {
	return &StoreMessage{
		ID:        id,
		Role:      role,
		Content:   content,
		Tokens:    tokens,
		Timestamp: ts,
		FileIDs:   fileIDs,
		Metadata: map[string]string{
			"source": "test",
		},
	}
}

func testSummaryNode(id string, kind SummaryKind, content string, tokens int, level int, createdAt time.Time) *SummaryNode {
	return &SummaryNode{
		ID:         id,
		Kind:       kind,
		Content:    content,
		Tokens:     tokens,
		Level:      level,
		CreatedAt:  createdAt,
		MessageIDs: []string{"msg_1", "msg_2"},
		ChildIDs:   []string{"sum_child_1"},
		ParentID:   "",
		FileIDs:    []string{"file_a.go", "file_b.go"},
	}
}

func TestMemoryBackend_PersistAndGetMessage(t *testing.T) {
	backend := NewMemoryBackend()

	ts := time.Now().UTC()
	msg := testStoreMessage("msg_1", RoleUser, "hello memory backend", 5, ts, []string{"a.txt"})
	if err := backend.PersistMessage(msg); err != nil {
		t.Fatalf("PersistMessage() error = %v", err)
	}

	got, err := backend.GetMessage("msg_1")
	if err != nil {
		t.Fatalf("GetMessage() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetMessage() = nil, want message")
	}
	if got.ID != msg.ID || got.Role != msg.Role || got.Content != msg.Content || got.Tokens != msg.Tokens {
		t.Fatalf("GetMessage() mismatch: got %+v, want %+v", got, msg)
	}
	if !got.Timestamp.Equal(msg.Timestamp) {
		t.Fatalf("GetMessage().Timestamp = %v, want %v", got.Timestamp, msg.Timestamp)
	}
	if len(got.FileIDs) != 1 || got.FileIDs[0] != "a.txt" {
		t.Fatalf("GetMessage().FileIDs = %v, want [a.txt]", got.FileIDs)
	}
}

func TestMemoryBackend_GetAllMessages(t *testing.T) {
	backend := NewMemoryBackend()
	base := time.Now().UTC()

	m1 := testStoreMessage("msg_1", RoleUser, "first", 1, base, []string{"1.txt"})
	m2 := testStoreMessage("msg_2", RoleAssistant, "second", 2, base.Add(time.Second), []string{"2.txt"})
	m3 := testStoreMessage("msg_3", RoleTool, "third", 3, base.Add(2*time.Second), []string{"3.txt"})

	for _, m := range []*StoreMessage{m1, m2, m3} {
		if err := backend.PersistMessage(m); err != nil {
			t.Fatalf("PersistMessage(%s) error = %v", m.ID, err)
		}
	}

	all, err := backend.GetAllMessages()
	if err != nil {
		t.Fatalf("GetAllMessages() error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("GetAllMessages() len = %d, want 3", len(all))
	}
	if all[0].ID != "msg_1" || all[1].ID != "msg_2" || all[2].ID != "msg_3" {
		t.Fatalf("GetAllMessages() order = [%s %s %s], want [msg_1 msg_2 msg_3]", all[0].ID, all[1].ID, all[2].ID)
	}
}

func TestMemoryBackend_PersistAndGetSummary(t *testing.T) {
	backend := NewMemoryBackend()

	node := testSummaryNode("sum_1", SummaryLeaf, "summary content", 7, 1, time.Now().UTC())
	if err := backend.PersistSummary(node); err != nil {
		t.Fatalf("PersistSummary() error = %v", err)
	}

	got, err := backend.GetSummary("sum_1")
	if err != nil {
		t.Fatalf("GetSummary() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetSummary() = nil, want summary")
	}
	if got.ID != node.ID || got.Kind != node.Kind || got.Content != node.Content || got.Tokens != node.Tokens || got.Level != node.Level {
		t.Fatalf("GetSummary() mismatch: got %+v, want %+v", got, node)
	}
	if got.ParentID != "" {
		t.Fatalf("GetSummary().ParentID = %q, want empty", got.ParentID)
	}
}

func TestMemoryBackend_MessageCount(t *testing.T) {
	backend := NewMemoryBackend()

	for i := 1; i <= 4; i++ {
		msg := testStoreMessage(
			"msg_"+string(rune('0'+i)),
			RoleUser,
			"count test",
			i,
			time.Now().UTC().Add(time.Duration(i)*time.Second),
			[]string{"count.txt"},
		)
		if err := backend.PersistMessage(msg); err != nil {
			t.Fatalf("PersistMessage(%s) error = %v", msg.ID, err)
		}
	}

	count, err := backend.MessageCount()
	if err != nil {
		t.Fatalf("MessageCount() error = %v", err)
	}
	if count != 4 {
		t.Fatalf("MessageCount() = %d, want 4", count)
	}
}

func TestMemoryBackend_UpdateSummaryParent(t *testing.T) {
	backend := NewMemoryBackend()

	node := testSummaryNode("sum_1", SummaryLeaf, "summary", 5, 1, time.Now().UTC())
	if err := backend.PersistSummary(node); err != nil {
		t.Fatalf("PersistSummary() error = %v", err)
	}

	if err := backend.UpdateSummaryParent("sum_1", "sum_parent"); err != nil {
		t.Fatalf("UpdateSummaryParent() error = %v", err)
	}

	got, err := backend.GetSummary("sum_1")
	if err != nil {
		t.Fatalf("GetSummary() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetSummary() = nil, want summary")
	}
	if got.ParentID != "sum_parent" {
		t.Fatalf("GetSummary().ParentID = %q, want %q", got.ParentID, "sum_parent")
	}
}

func TestSQLiteBackend_CreateAndClose(t *testing.T) {
	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteBackend(:memory:) error = %v", err)
	}
	if backend == nil {
		t.Fatal("NewSQLiteBackend(:memory:) = nil, want backend")
	}

	if err := backend.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestSQLiteBackend_PersistAndGetMessage(t *testing.T) {
	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteBackend(:memory:) error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	ts := time.Now().UTC()
	msg := testStoreMessage("msg_1", RoleAssistant, "sqlite hello", 11, ts, []string{"sqlite.txt"})
	if err := backend.PersistMessage(msg); err != nil {
		t.Fatalf("PersistMessage() error = %v", err)
	}

	got, err := backend.GetMessage("msg_1")
	if err != nil {
		t.Fatalf("GetMessage() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetMessage() = nil, want message")
	}
	if got.ID != msg.ID || got.Role != msg.Role || got.Content != msg.Content || got.Tokens != msg.Tokens {
		t.Fatalf("GetMessage() mismatch: got %+v, want %+v", got, msg)
	}
	if !got.Timestamp.Equal(msg.Timestamp) {
		t.Fatalf("GetMessage().Timestamp = %v, want %v", got.Timestamp, msg.Timestamp)
	}
	if len(got.FileIDs) != 1 || got.FileIDs[0] != "sqlite.txt" {
		t.Fatalf("GetMessage().FileIDs = %v, want [sqlite.txt]", got.FileIDs)
	}
}

func TestSQLiteBackend_GetAllMessages(t *testing.T) {
	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteBackend(:memory:) error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	base := time.Now().UTC()
	m1 := testStoreMessage("msg_1", RoleUser, "first sqlite", 1, base, []string{"1.txt"})
	m2 := testStoreMessage("msg_2", RoleAssistant, "second sqlite", 2, base.Add(time.Second), []string{"2.txt"})
	m3 := testStoreMessage("msg_3", RoleTool, "third sqlite", 3, base.Add(2*time.Second), []string{"3.txt"})

	for _, m := range []*StoreMessage{m1, m2, m3} {
		if err := backend.PersistMessage(m); err != nil {
			t.Fatalf("PersistMessage(%s) error = %v", m.ID, err)
		}
	}

	all, err := backend.GetAllMessages()
	if err != nil {
		t.Fatalf("GetAllMessages() error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("GetAllMessages() len = %d, want 3", len(all))
	}
	if all[0].ID != "msg_1" || all[1].ID != "msg_2" || all[2].ID != "msg_3" {
		t.Fatalf("GetAllMessages() order = [%s %s %s], want [msg_1 msg_2 msg_3]", all[0].ID, all[1].ID, all[2].ID)
	}
}

func TestSQLiteBackend_MessageCount(t *testing.T) {
	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteBackend(:memory:) error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	for i := 1; i <= 5; i++ {
		msg := testStoreMessage(
			"msg_"+string(rune('0'+i)),
			RoleUser,
			"sqlite count",
			i,
			time.Now().UTC().Add(time.Duration(i)*time.Second),
			[]string{"count_sqlite.txt"},
		)
		if err := backend.PersistMessage(msg); err != nil {
			t.Fatalf("PersistMessage(%s) error = %v", msg.ID, err)
		}
	}

	count, err := backend.MessageCount()
	if err != nil {
		t.Fatalf("MessageCount() error = %v", err)
	}
	if count != 5 {
		t.Fatalf("MessageCount() = %d, want 5", count)
	}
}

func TestSQLiteBackend_PersistAndGetSummary(t *testing.T) {
	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteBackend(:memory:) error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	node := testSummaryNode("sum_1", SummaryCondensed, "sqlite summary", 13, 2, time.Now().UTC())
	node.ParentID = "sum_parent"
	if err := backend.PersistSummary(node); err != nil {
		t.Fatalf("PersistSummary() error = %v", err)
	}

	got, err := backend.GetSummary("sum_1")
	if err != nil {
		t.Fatalf("GetSummary() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetSummary() = nil, want summary")
	}
	if got.ID != node.ID || got.Kind != node.Kind || got.Content != node.Content || got.Tokens != node.Tokens || got.Level != node.Level {
		t.Fatalf("GetSummary() mismatch: got %+v, want %+v", got, node)
	}
	if got.ParentID != "sum_parent" {
		t.Fatalf("GetSummary().ParentID = %q, want %q", got.ParentID, "sum_parent")
	}
}

func TestSQLiteBackend_UpdateSummaryParent(t *testing.T) {
	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteBackend(:memory:) error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	node := testSummaryNode("sum_1", SummaryLeaf, "sqlite summary", 5, 1, time.Now().UTC())
	if err := backend.PersistSummary(node); err != nil {
		t.Fatalf("PersistSummary() error = %v", err)
	}

	if err := backend.UpdateSummaryParent("sum_1", "sum_parent_2"); err != nil {
		t.Fatalf("UpdateSummaryParent() error = %v", err)
	}

	got, err := backend.GetSummary("sum_1")
	if err != nil {
		t.Fatalf("GetSummary() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetSummary() = nil, want summary")
	}
	if got.ParentID != "sum_parent_2" {
		t.Fatalf("GetSummary().ParentID = %q, want %q", got.ParentID, "sum_parent_2")
	}
}

func TestSQLiteBackend_GrepMessages_Simple(t *testing.T) {
	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteBackend(:memory:) error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	base := time.Now().UTC()
	msgs := []*StoreMessage{
		testStoreMessage("msg_1", RoleUser, "alpha beta gamma", 3, base, []string{"a.txt"}),
		testStoreMessage("msg_2", RoleAssistant, "delta epsilon", 2, base.Add(time.Second), []string{"b.txt"}),
		testStoreMessage("msg_3", RoleTool, "gamma zeta", 2, base.Add(2*time.Second), []string{"c.txt"}),
	}
	for _, m := range msgs {
		if err := backend.PersistMessage(m); err != nil {
			t.Fatalf("PersistMessage(%s) error = %v", m.ID, err)
		}
	}

	results, err := backend.GrepMessages("gamma", nil, 10)
	if err != nil {
		t.Fatalf("GrepMessages() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("GrepMessages() len = %d, want 2", len(results))
	}
	for _, r := range results {
		if !strings.Contains(r.Content, "gamma") {
			t.Fatalf("result content %q does not contain %q", r.Content, "gamma")
		}
	}
}

func TestSQLiteBackend_GrepMessages_Regex(t *testing.T) {
	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteBackend(:memory:) error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	base := time.Now().UTC()
	msgs := []*StoreMessage{
		testStoreMessage("msg_1", RoleUser, "hello world", 2, base, []string{"a.txt"}),
		testStoreMessage("msg_2", RoleAssistant, "hallo world", 2, base.Add(time.Second), []string{"b.txt"}),
		testStoreMessage("msg_3", RoleTool, "goodbye world", 2, base.Add(2*time.Second), []string{"c.txt"}),
	}
	for _, m := range msgs {
		if err := backend.PersistMessage(m); err != nil {
			t.Fatalf("PersistMessage(%s) error = %v", m.ID, err)
		}
	}

	results, err := backend.GrepMessages("h.llo\\s+world", nil, 10)
	if err != nil {
		t.Fatalf("GrepMessages() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("GrepMessages() len = %d, want 2", len(results))
	}
}

func TestSQLiteBackend_GetMessage_NotFound(t *testing.T) {
	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteBackend(:memory:) error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	got, err := backend.GetMessage("does_not_exist")
	if err != nil {
		t.Fatalf("GetMessage() error = %v", err)
	}
	if got != nil {
		t.Fatalf("GetMessage() = %+v, want nil", got)
	}
}

func TestSQLiteBackend_GetSummary_NotFound(t *testing.T) {
	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteBackend(:memory:) error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	got, err := backend.GetSummary("does_not_exist")
	if err != nil {
		t.Fatalf("GetSummary() error = %v", err)
	}
	if got != nil {
		t.Fatalf("GetSummary() = %+v, want nil", got)
	}
}

func TestStoreBackend_InterfaceCompliance(t *testing.T) {
	var _ StoreBackend = NewMemoryBackend()

	sqliteBackend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteBackend(:memory:) error = %v", err)
	}
	defer func() { _ = sqliteBackend.Close() }()

	var _ StoreBackend = sqliteBackend
}