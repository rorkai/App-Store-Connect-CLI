package approvals

import "testing"

func TestQueueApproveFlow(t *testing.T) {
	queue := NewQueue()
	queue.Enqueue(Action{ID: "one", Title: "Publish update"})

	pending := queue.Pending()
	if len(pending) != 1 {
		t.Fatalf("Pending() len = %d, want 1", len(pending))
	}

	got, err := queue.Approve("one")
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if got.Status != StatusApproved {
		t.Fatalf("Status = %q, want approved", got.Status)
	}
	if len(queue.Pending()) != 0 {
		t.Fatalf("Pending() len after approval = %d, want 0", len(queue.Pending()))
	}
}
