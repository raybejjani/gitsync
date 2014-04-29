package changebus

import (
	"gitsync"
	"testing"
)

// TestBlockingListener tests if we skip blocking listener channels
func TestBlockingListener(t *testing.T) {
	errors := make(chan error, 8)
	cb := New(10, errors)
	change := gitsync.GitChange{}

	// add a listener that will block, this should trigger an error
	listener := make(ChangeListener)
	cb.RegisterListener(listener)
	if err := cb.Publish(change); err != nil {
		t.Fatalf("Saw error when we shouldn't have: %s", err.Error())
	} else if err := <-errors; err == nil {
		t.Fatalf("Did not see expected error")
	}
}

// TestNilErrors tests if we skip blocking listener channels
func TestNilErrors(t *testing.T) {
	cb := New(10, nil)
	change := gitsync.GitChange{}

	// don't crash if there are no listeners
	if err := cb.Publish(change); err != nil {
		t.Fatalf("Saw error when we shouldn't have: %s", err.Error())
	}

	// add a listener that will block, this should trigger an error that cannot be
	// reported because errors is nil
	listener := make(ChangeListener)
	cb.RegisterListener(listener)
	if err := cb.Publish(change); err != nil {
		t.Fatalf("Saw error when we shouldn't have: %s", err.Error())
	}
}
