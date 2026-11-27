package catalog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const (
	catalogPlanVersion   = 1
	catalogPlanByteLimit = 16 << 20
)

type savedCatalogPlan struct {
	SchemaVersion  int                     `json:"schemaVersion"`
	AppID          string                  `json:"appId"`
	ManifestSHA256 string                  `json:"manifestSha256"`
	RemoteSHA256   string                  `json:"remoteSha256"`
	Operations     []savedCatalogOperation `json:"operations"`
}

type savedCatalogOperation struct {
	Item            asc.CatalogItem `json:"item"`
	RequiresConfirm bool            `json:"requiresConfirm,omitempty"`
}

type catalogEvidenceKey struct{}

type catalogEvidence map[string]string

func readCatalogFile(path, label string, limit int64) ([]byte, error) {
	file, err := rootfs.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds %d MiB", label, limit>>20)
	}
	return data, nil
}

func catalogDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func readSavedCatalogPlan(path string) (savedCatalogPlan, []byte, error) {
	var plan savedCatalogPlan
	raw, err := readCatalogFile(path, "saved catalog plan", catalogPlanByteLimit)
	if err != nil {
		return plan, nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(&plan); err != nil || plan.SchemaVersion != catalogPlanVersion ||
		len(plan.ManifestSHA256) != sha256.Size*2 || len(plan.RemoteSHA256) != sha256.Size*2 || len(plan.Operations) == 0 {
		return plan, nil, fmt.Errorf("invalid or unsupported saved catalog plan")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return plan, nil, fmt.Errorf("invalid saved catalog plan")
	}
	canonical, err := canonicalCatalogJSON(plan)
	if err != nil || !bytes.Equal(canonical, compact.Bytes()) {
		return plan, nil, fmt.Errorf("saved catalog plan was edited or is invalid; generate it again")
	}
	return plan, canonical, nil
}

func buildSavedCatalogPlan(appID, manifestHash string, remote catalogEvidence, ops []operation) ([]byte, error) {
	remoteBytes, err := json.Marshal(remote)
	if err != nil {
		return nil, err
	}
	result := savedCatalogPlan{
		SchemaVersion:  catalogPlanVersion,
		AppID:          appID,
		ManifestSHA256: manifestHash,
		RemoteSHA256:   catalogDigest(remoteBytes),
		Operations:     make([]savedCatalogOperation, 0, len(ops)),
	}
	for _, op := range ops {
		if op.item.Deferred {
			var versionCommand string
			switch op.item.Kind {
			case "groupLocalization":
				versionCommand = "asc subscriptions groups versions create --group-id GROUP_ID"
			case "subscriptionLocalization":
				versionCommand = "asc subscriptions versions create --subscription-id SUBSCRIPTION_ID"
			case "inAppPurchaseLocalization":
				versionCommand = "asc iap versions create --iap-id IAP_ID"
			}
			if versionCommand != "" {
				return nil, fmt.Errorf("saved plans require resolved targets; create the parent product or group first if needed, then create an editable version with %s; after creation, rerun catalog plan --out FILE with the original manifest", versionCommand)
			}
			return nil, fmt.Errorf("saved plans require resolved targets; create the parent product in a separate catalog plan without pricing, offers, or localizations, apply it, then generate a new plan")
		}
		result.Operations = append(result.Operations, savedCatalogOperation{Item: op.item, RequiresConfirm: op.destructive})
	}
	return canonicalCatalogJSON(result)
}

func writeSavedCatalogPlan(path string, canonical []byte) error {
	var body bytes.Buffer
	if err := json.Indent(&body, canonical, "", "  "); err != nil {
		return err
	}
	body.WriteByte('\n')
	if body.Len() > catalogPlanByteLimit {
		return fmt.Errorf("saved catalog plan exceeds %d MiB", catalogPlanByteLimit>>20)
	}
	return rootfs.CreateNewFileAtomic(path, body.Bytes(), 0o600)
}

func recordCatalogEvidence(ctx context.Context, key string, value any) error {
	evidence, _ := ctx.Value(catalogEvidenceKey{}).(catalogEvidence)
	if evidence == nil {
		return nil
	}
	canonical, err := canonicalCatalogJSON(value)
	if err != nil {
		return err
	}
	digest := catalogDigest(canonical)
	if previous, ok := evidence[key]; ok && previous != digest {
		return fmt.Errorf("catalog changed while reading preflight state")
	}
	evidence[key] = digest
	return nil
}

func canonicalCatalogJSON(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}

func savedCatalogFailure(err error) (status, message string) {
	var response interface{ HTTPStatusCode() int }
	if errors.Is(err, readonly.ErrRefused) || (errors.As(err, &response) && response.HTTPStatusCode() >= 400 &&
		response.HTTPStatusCode() < 500 && response.HTTPStatusCode() != http.StatusRequestTimeout) {
		return "failed", "The write was refused."
	}
	return "unknown", "The write outcome is unknown. Read current state before attempting it again."
}
