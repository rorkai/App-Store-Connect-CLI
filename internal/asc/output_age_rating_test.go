package asc

import (
	"reflect"
	"strings"
	"testing"
)

func TestPrintTableAgeRatingIncludesSocialMediaFields(t *testing.T) {
	gracNumber := "CC-2026-123"
	socialMedia := true
	ageRestricted := false
	resp := &AgeRatingDeclarationResponse{
		Data: Resource[AgeRatingDeclarationAttributes]{
			ID:   "age-441",
			Type: ResourceTypeAgeRatingDeclarations,
			Attributes: AgeRatingDeclarationAttributes{
				SocialMedia:                    &NullableBool{Value: &socialMedia},
				GracRatingClassificationNumber: &NullableString{Value: &gracNumber},
				SocialMediaAgeRestricted:       &NullableBool{Value: &ageRestricted},
			},
		},
	}

	output := captureStdout(t, func() error { return PrintTable(resp) })
	for _, want := range []string{"Social Media", "Social Media Age Restricted", "true", "false", "GRAC Rating Classification Number", gracNumber} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected table output to contain %q, got %q", want, output)
		}
	}
}

func TestAgeRatingAuditResultRows(t *testing.T) {
	result := &AgeRatingAuditResult{
		Apps: []AgeRatingAuditRow{
			{
				AppID:                    "app-1",
				AppInfoID:                "info-1",
				AppInfoState:             "READY_FOR_DISTRIBUTION",
				Name:                     "Ready App",
				SocialMedia:              "true",
				SocialMediaAgeRestricted: "true",
				MessagingAndChat:         "false",
				UserGeneratedContent:     "true",
				AgeAssurance:             "true",
				Ready:                    true,
			},
			{
				AppID:                "app-2",
				Name:                 "Broken App",
				UserGeneratedContent: "-",
				MissingResponses:     []string{"socialMedia", "messagingAndChat"},
				Error:                "request failed",
			},
		},
	}

	headers, rows := ageRatingAuditResultRows(result)
	wantHeaders := []string{"App ID", "App Info ID", "State", "Name", "Social Media", "Age Restricted", "Messaging & Chat", "User Generated Content", "Age Assurance", "Ready", "Missing"}
	if !reflect.DeepEqual(headers, wantHeaders) {
		t.Fatalf("headers = %#v, want %#v", headers, wantHeaders)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %#v, want 2 rows", rows)
	}
	if got := rows[0][1]; got != "info-1" {
		t.Fatalf("app info column = %q, want info-1", got)
	}
	if got := rows[0][2]; got != "READY_FOR_DISTRIBUTION" {
		t.Fatalf("state column = %q, want READY_FOR_DISTRIBUTION", got)
	}
	if got := rows[0][7]; got != "true" {
		t.Fatalf("user-generated content column = %q, want true", got)
	}
	if got := rows[0][8]; got != "true" {
		t.Fatalf("age assurance column = %q, want true", got)
	}
	if got := rows[0][9]; got != "true" {
		t.Fatalf("ready column = %q, want true", got)
	}
	if got := rows[0][10]; got != "" {
		t.Fatalf("ready missing column = %q, want empty", got)
	}
	if got := rows[1][9]; got != "false" {
		t.Fatalf("error ready column = %q, want false", got)
	}
	if got := rows[1][7]; got != "-" {
		t.Fatalf("error user-generated content column = %q, want -", got)
	}
	if got := rows[1][10]; got != "error: request failed" {
		t.Fatalf("error missing column = %q, want error detail", got)
	}
	ensureOutputRegistryPopulated()
	if !isRegistryTypeRegistered(typeForPtr[AgeRatingAuditResult]()) {
		t.Fatal("AgeRatingAuditResult is not registered with the output renderer")
	}
}
