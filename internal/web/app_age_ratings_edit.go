package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// AgeRatingDeclarationEdit contains fields validated by ParseAgeRatingDeclarationEdit.
type AgeRatingDeclarationEdit struct {
	attributes map[string]json.RawMessage
}

// ParseAgeRatingDeclarationEdit accepts the fields observed in the App Store
// Connect age-rating form's save request.
func ParseAgeRatingDeclarationEdit(input []byte) (AgeRatingDeclarationEdit, error) {
	decoder := json.NewDecoder(bytes.NewReader(input))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return AgeRatingDeclarationEdit{}, fmt.Errorf("--input must contain a JSON object")
	}

	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return AgeRatingDeclarationEdit{}, fmt.Errorf("invalid --input JSON: %w", err)
		}
		field := token.(string) // Object keys are strings.
		if _, ok := fields[field]; ok {
			return AgeRatingDeclarationEdit{}, fmt.Errorf("duplicate --input field %q", field)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return AgeRatingDeclarationEdit{}, fmt.Errorf("invalid --input JSON: %w", err)
		}
		if err := validateAgeRatingEditField(field, value); err != nil {
			return AgeRatingDeclarationEdit{}, err
		}
		fields[field] = value
	}
	if _, err := decoder.Token(); err != nil {
		return AgeRatingDeclarationEdit{}, fmt.Errorf("invalid --input JSON: %w", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return AgeRatingDeclarationEdit{}, fmt.Errorf("--input must contain exactly one JSON object")
	}
	if len(fields) == 0 {
		return AgeRatingDeclarationEdit{}, fmt.Errorf("--input must contain at least one age-rating field")
	}
	return AgeRatingDeclarationEdit{attributes: fields}, nil
}

func validateAgeRatingEditField(field string, value json.RawMessage) error {
	var allowed []string
	switch field {
	case "ageAssurance", "advertising", "gambling", "healthOrWellnessTopics", "lootBox",
		"messagingAndChat", "parentalControls", "socialMedia", "socialMediaAgeRestricted",
		"unrestrictedWebAccess", "userGeneratedContent":
		if string(value) != "true" && string(value) != "false" {
			return fmt.Errorf("--input field %q must be a boolean", field)
		}
		return nil
	case "alcoholTobaccoOrDrugUseOrReferences", "contests", "gamblingSimulated",
		"gunsOrOtherWeapons", "horrorOrFearThemes", "matureOrSuggestiveThemes",
		"medicalOrTreatmentInformation", "profanityOrCrudeHumor",
		"sexualContentGraphicAndNudity", "sexualContentOrNudity", "violenceCartoonOrFantasy",
		"violenceRealistic", "violenceRealisticProlongedGraphicOrSadistic":
		allowed = []string{"NONE", "INFREQUENT_OR_MILD", "FREQUENT_OR_INTENSE", "INFREQUENT", "FREQUENT"}
	case "ageRatingOverrideV2":
		allowed = []string{"NONE", "NINE_PLUS", "THIRTEEN_PLUS", "SIXTEEN_PLUS", "EIGHTEEN_PLUS", "UNRATED"}
	case "kidsAgeBand":
		if string(value) == "null" {
			return nil
		}
		allowed = []string{"FIVE_AND_UNDER", "SIX_TO_EIGHT", "NINE_TO_ELEVEN"}
	case "developerAgeRatingInfoUrl":
		if string(value) == "null" {
			return nil
		}
		var text string
		if err := json.Unmarshal(value, &text); err == nil {
			parsed, err := url.ParseRequestURI(text)
			if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != "" && parsed.User == nil {
				return nil
			}
		}
		return fmt.Errorf("--input field %q must be null or an absolute HTTP(S) URL without credentials", field)
	default:
		return fmt.Errorf("unsupported --input field %q", field)
	}

	var text string
	if err := json.Unmarshal(value, &text); err != nil || !slices.Contains(allowed, text) {
		return fmt.Errorf("--input field %q must be one of %s", field, strings.Join(allowed, ", "))
	}
	return nil
}

// UpdateAgeRatingDeclaration submits the supplied age-rating fields with one Iris PATCH.
func (c *Client) UpdateAgeRatingDeclaration(ctx context.Context, declarationID string, edit AgeRatingDeclarationEdit) (json.RawMessage, error) {
	declarationID = strings.TrimSpace(declarationID)
	if declarationID == "" || declarationID == "." || declarationID == ".." {
		return nil, fmt.Errorf("valid age rating declaration id is required")
	}
	if len(edit.attributes) == 0 {
		return nil, fmt.Errorf("at least one validated age-rating field is required")
	}
	const resourceType = asc.ResourceTypeAgeRatingDeclarations
	type resource struct {
		Type       asc.ResourceType           `json:"type"`
		ID         string                     `json:"id"`
		Attributes map[string]json.RawMessage `json:"attributes"`
	}
	request := struct {
		Data resource `json:"data"`
	}{Data: resource{Type: resourceType, ID: declarationID, Attributes: edit.attributes}}
	// A 307 or 308 can otherwise cause net/http to replay the PATCH.
	client := *c.httpClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return fmt.Errorf("unexpected redirect during age rating update")
	}
	path := "/ageRatingDeclarations/" + url.PathEscape(declarationID)
	body, err := c.doRequestBaseWithHTTPClient(&client, ctx, c.baseURL, http.MethodPatch, path, request, irisRequestHeaders())
	if err != nil || len(bytes.TrimSpace(body)) == 0 {
		return nil, err
	}
	if !json.Valid(body) {
		return nil, fmt.Errorf("age rating update returned invalid JSON, so check the declaration before retrying")
	}
	return body, nil
}
