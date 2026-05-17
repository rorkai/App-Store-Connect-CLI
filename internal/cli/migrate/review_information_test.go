package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFastlaneReviewInformation_ParsesFiles(t *testing.T) {
	root := t.TempDir()
	reviewDir := filepath.Join(root, "review_information")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("mkdir review_information: %v", err)
	}
	writeReviewFile := func(name, value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(reviewDir, name), []byte(value), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	writeReviewFile("first_name.txt", "Rita")
	writeReviewFile("last_name.txt", "Reviewer")
	writeReviewFile("phone_number.txt", "+1-555-1234")
	writeReviewFile("email_address.txt", "rita@example.com")
	writeReviewFile("demo_user.txt", "demo_user")
	writeReviewFile("demo_password.txt", "demo_pass")
	writeReviewFile("notes.txt", "Notes for review")
	writeReviewFile("demo_required.txt", "true")

	info, err := readFastlaneReviewInformation(root)
	if err != nil {
		t.Fatalf("readFastlaneReviewInformation() error: %v", err)
	}
	if info == nil {
		t.Fatal("expected review information, got nil")
		return
	}
	if info.ContactFirstName == nil || *info.ContactFirstName != "Rita" {
		t.Fatalf("expected contact first name Rita, got %#v", info.ContactFirstName)
	}
	if info.ContactLastName == nil || *info.ContactLastName != "Reviewer" {
		t.Fatalf("expected contact last name Reviewer, got %#v", info.ContactLastName)
	}
	if info.ContactPhone == nil || *info.ContactPhone != "+1-555-1234" {
		t.Fatalf("expected contact phone, got %#v", info.ContactPhone)
	}
	if info.ContactEmail == nil || *info.ContactEmail != "rita@example.com" {
		t.Fatalf("expected contact email, got %#v", info.ContactEmail)
	}
	if info.DemoAccountName == nil || *info.DemoAccountName != "demo_user" {
		t.Fatalf("expected demo account name, got %#v", info.DemoAccountName)
	}
	if info.DemoAccountPassword == nil || *info.DemoAccountPassword != "demo_pass" {
		t.Fatalf("expected demo account password, got %#v", info.DemoAccountPassword)
	}
	if info.Notes == nil || *info.Notes != "Notes for review" {
		t.Fatalf("expected notes, got %#v", info.Notes)
	}
	if info.DemoAccountRequired == nil || !*info.DemoAccountRequired {
		t.Fatalf("expected demo account required true, got %#v", info.DemoAccountRequired)
	}
}

func TestReadFastlaneReviewInformation_InvalidBool(t *testing.T) {
	root := t.TempDir()
	reviewDir := filepath.Join(root, "review_information")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("mkdir review_information: %v", err)
	}
	path := filepath.Join(reviewDir, "demo_required.txt")
	if err := os.WriteFile(path, []byte("maybe"), 0o644); err != nil {
		t.Fatalf("write demo_required: %v", err)
	}

	_, err := readFastlaneReviewInformation(root)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("expected error to mention %q, got %v", path, err)
	}
}

func TestReadFastlaneReviewInformation_MissingDirectory(t *testing.T) {
	root := t.TempDir()

	info, err := readFastlaneReviewInformation(root)
	if err != nil {
		t.Fatalf("readFastlaneReviewInformation() error: %v", err)
	}
	if info != nil {
		t.Fatalf("expected nil review information, got %#v", info)
	}
}
