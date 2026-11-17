package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// DeveloperICloudContainerIdentifierPrefix is the literal prefix Apple
// requires on every iCloud container identifier.
const DeveloperICloudContainerIdentifierPrefix = "iCloud."

const developerICloudContainerResourceType = "cloudContainers"

// DeveloperICloudContainerCreateRequest contains the writable fields sent to
// the Developer Portal iCloud container create endpoint.
type DeveloperICloudContainerCreateRequest struct {
	Identifier string
	Name       string
}

// DeveloperICloudContainerUnverifiedError reports a create whose final state
// cannot be established. iCloud containers cannot be deleted, so callers must
// inspect the list before retrying; the client never retries the POST.
type DeveloperICloudContainerUnverifiedError struct {
	Err error
}

func (e *DeveloperICloudContainerUnverifiedError) Error() string {
	if e == nil || e.Err == nil {
		return "developer portal iCloud container create outcome is unknown"
	}
	return e.Err.Error()
}

func (e *DeveloperICloudContainerUnverifiedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type developerICloudContainerCreatePayload struct {
	Data developerICloudContainerCreateData `json:"data"`
}

type developerICloudContainerCreateData struct {
	Type       string            `json:"type"`
	Attributes map[string]string `json:"attributes"`
}

// ValidateDeveloperICloudContainerIdentifier checks the identifier before any
// session or request is made. The value must carry Apple's literal "iCloud."
// prefix followed by a reverse-DNS string of letters, digits, hyphens, and
// dots. The CLI never adds the prefix on the caller's behalf.
func ValidateDeveloperICloudContainerIdentifier(identifier string) error {
	if identifier == "" {
		return fmt.Errorf("--identifier is required")
	}
	if !strings.HasPrefix(identifier, DeveloperICloudContainerIdentifierPrefix) {
		if len(identifier) >= len(DeveloperICloudContainerIdentifierPrefix) && strings.EqualFold(identifier[:len(DeveloperICloudContainerIdentifierPrefix)], DeveloperICloudContainerIdentifierPrefix) {
			return fmt.Errorf("--identifier must start with %q exactly (got %q); use %q", DeveloperICloudContainerIdentifierPrefix, identifier, DeveloperICloudContainerIdentifierPrefix+identifier[len(DeveloperICloudContainerIdentifierPrefix):])
		}
		return fmt.Errorf("--identifier must start with %q, for example %q", DeveloperICloudContainerIdentifierPrefix, DeveloperICloudContainerIdentifierPrefix+"com.example.app")
	}
	rest := identifier[len(DeveloperICloudContainerIdentifierPrefix):]
	if rest == "" {
		return fmt.Errorf("--identifier must contain a reverse-DNS string after %q, for example %q", DeveloperICloudContainerIdentifierPrefix, DeveloperICloudContainerIdentifierPrefix+"com.example.app")
	}
	for _, r := range rest {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '.':
		default:
			return fmt.Errorf("--identifier may contain only letters, digits, hyphens, and dots after %q (got %q)", DeveloperICloudContainerIdentifierPrefix, identifier)
		}
	}
	if strings.HasPrefix(rest, ".") || strings.HasSuffix(rest, ".") || strings.Contains(rest, "..") {
		return fmt.Errorf("--identifier must not have empty dot-separated parts (got %q)", identifier)
	}
	return nil
}

// CreateDeveloperICloudContainer registers one iCloud container for the
// selected Developer Portal team. iCloud containers cannot be deleted.
//
// The request uses the modern Developer Portal JSON:API collection that the
// list command reads: POST /services-account/v1/cloudContainers with
// data.type=cloudContainers and identifier, name, and teamId attributes, the
// same shape as the captured websitepushIds and Services ID creates. The
// selected team travels only in data.attributes; a root-level teamId is
// rejected by Apple on create for the sibling resources.
//
// Both visible and hidden collections are read first so an existing
// identifier fails before the permanent write. After the POST, the visible
// and hidden collections are read back and the new container must appear
// exactly once with the requested identifier.
func (c *Client) CreateDeveloperICloudContainer(ctx context.Context, request DeveloperICloudContainerCreateRequest) (*asc.WebICloudContainerCreateResult, error) {
	request.Identifier = strings.TrimSpace(request.Identifier)
	request.Name = strings.TrimSpace(request.Name)
	if err := ValidateDeveloperICloudContainerIdentifier(request.Identifier); err != nil {
		return nil, err
	}
	if request.Name == "" {
		return nil, fmt.Errorf("--name is required")
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("developer portal team is not selected; %s", developerPortalAuthHint())
	}

	existing, err := c.findDeveloperICloudContainers(ctx, request.Identifier, true)
	if err != nil {
		return nil, fmt.Errorf("cannot safely preflight iCloud container creation: %w", err)
	}
	if len(existing) > 0 {
		return nil, fmt.Errorf("iCloud container %q already exists in the selected Developer Portal team (id %s)", request.Identifier, existing[0].ID)
	}

	payload := developerICloudContainerCreatePayload{
		Data: developerICloudContainerCreateData{
			Type: developerICloudContainerResourceType,
			Attributes: map[string]string{
				"identifier": request.Identifier,
				"name":       request.Name,
				"teamId":     teamID,
			},
		},
	}
	body, err := c.doDeveloperPortalRequest(ctx, http.MethodPost, "/cloudContainers", payload, developerPortalHeaders(""), true)
	if err != nil {
		return nil, developerICloudContainerWriteError(err)
	}

	createdID, parseErr := developerICloudContainerIDFromCreateResponse(body, request.Identifier)
	created, verifyErr := c.findDeveloperICloudContainers(ctx, request.Identifier, false)
	if verifyErr != nil {
		return nil, &DeveloperICloudContainerUnverifiedError{Err: fmt.Errorf("developer portal accepted the iCloud container create but the read-back failed; run asc web icloud-containers list before retrying: %w", verifyErr)}
	}
	if len(created) != 1 {
		detail := fmt.Sprintf("the read-back found %d containers with identifier %q", len(created), request.Identifier)
		if parseErr != nil {
			detail += fmt.Sprintf(" and the create response was unusable (%v)", parseErr)
		}
		return nil, &DeveloperICloudContainerUnverifiedError{Err: fmt.Errorf("developer portal accepted the iCloud container create but %s; run asc web icloud-containers list before retrying", detail)}
	}
	container := created[0]
	if parseErr != nil {
		return nil, &DeveloperICloudContainerUnverifiedError{Err: fmt.Errorf("developer portal accepted the iCloud container create, and the read-back found %q with id %s, but the create response disagrees (%w); run asc web icloud-containers list before retrying", request.Identifier, container.ID, parseErr)}
	}
	if createdID != "" && container.ID != createdID {
		return nil, &DeveloperICloudContainerUnverifiedError{Err: fmt.Errorf("developer portal created iCloud container %q with id %s but the read-back returned id %s; run asc web icloud-containers list before retrying", request.Identifier, createdID, container.ID)}
	}

	result := &asc.WebICloudContainerCreateResult{
		Operation:   "create",
		ContainerID: container.ID,
		Identifier:  container.Attributes.Identifier,
		Name:        container.Attributes.Name,
		Prefix:      container.Attributes.Prefix,
		Hidden:      container.Attributes.Hidden,
		Changed:     true,
		Verified:    true,
		Permanent:   true,
		Status:      "created",
	}
	if result.Name != request.Name {
		result.RequestedName = request.Name
	}
	return result, nil
}

// findDeveloperICloudContainers returns every visible or hidden container
// whose identifier matches exactly. With requireComplete, it fails closed
// when either bounded collection reports more rows than it returned, because
// an absent match in an incomplete collection proves nothing. The post-create
// read-back skips that check: a found match is still proof, and a missing one
// is reported as unverified.
func (c *Client) findDeveloperICloudContainers(ctx context.Context, identifier string, requireComplete bool) ([]DeveloperICloudContainer, error) {
	var matches []DeveloperICloudContainer
	for _, hidden := range []bool{false, true} {
		result, err := c.listDeveloperICloudContainersAfterSession(ctx, hidden)
		if err != nil {
			return nil, err
		}
		if requireComplete {
			if err := requireCompleteDeveloperICloudContainerCollection(result, hidden); err != nil {
				return nil, err
			}
		}
		for _, container := range result.Data {
			if container.Attributes.Identifier == identifier {
				matches = append(matches, container)
			}
		}
	}
	return matches, nil
}

func requireCompleteDeveloperICloudContainerCollection(result *DeveloperICloudContainersListResult, hidden bool) error {
	collection := "visible"
	if hidden {
		collection = "hidden"
	}
	if links := result.GetLinks(); links != nil && links.Next != "" {
		return fmt.Errorf("the %s iCloud container collection has another page, which this command cannot read", collection)
	}
	if total, ok := asc.ParsePagingTotalOK(result.GetMeta()); ok && total > len(result.Data) {
		return fmt.Errorf("the %s iCloud container collection reports %d containers but returned %d", collection, total, len(result.Data))
	}
	return nil
}

// developerICloudContainerIDFromCreateResponse extracts the created resource
// ID. An empty body or missing data yields "" so the read-back decides. A
// resource of the wrong type or identifier is an error.
func developerICloudContainerIDFromCreateResponse(body []byte, identifier string) (string, error) {
	if len(strings.TrimSpace(string(body))) == 0 {
		return "", nil
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", fmt.Errorf("failed to parse create response: %w", err)
	}
	if missingJSONValue(envelope.Data) {
		return "", nil
	}
	var resource DeveloperICloudContainer
	if err := json.Unmarshal(envelope.Data, &resource); err != nil {
		return "", fmt.Errorf("failed to parse created iCloud container: %w", err)
	}
	if err := validateDeveloperICloudContainerResource(resource); err != nil {
		return "", err
	}
	if got := resource.Attributes.Identifier; got != "" && got != identifier {
		return "", fmt.Errorf("created iCloud container identifier is %q, want %q", got, identifier)
	}
	return resource.ID, nil
}

func developerICloudContainerWriteError(err error) error {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.Status == http.StatusRequestTimeout || apiErr.Status >= http.StatusInternalServerError {
			return &DeveloperICloudContainerUnverifiedError{Err: fmt.Errorf("developer portal iCloud container create outcome is unknown after status %d; run asc web icloud-containers list before retrying: %w", apiErr.Status, err)}
		}
		return err
	}
	if isAmbiguousDeveloperPortalWriteFailure(err) {
		return &DeveloperICloudContainerUnverifiedError{Err: fmt.Errorf("developer portal iCloud container create outcome is unknown; run asc web icloud-containers list before retrying: %w", err)}
	}
	return err
}
