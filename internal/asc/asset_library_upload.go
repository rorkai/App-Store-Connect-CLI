package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type assetLibraryImageUploadResponse struct {
	Data struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			State            string            `json:"state"`
			SpecID           string            `json:"specId"`
			UploadOperations []UploadOperation `json:"uploadOperations"`
			ImageAsset       *struct {
				Width  int `json:"width"`
				Height int `json:"height"`
			} `json:"imageAsset"`
		} `json:"attributes"`
	} `json:"data"`
}

func (r assetLibraryImageUploadResponse) ready() bool {
	attrs := r.Data.Attributes
	return attrs.State == "PREPARE_FOR_SUBMISSION" && attrs.ImageAsset != nil && attrs.ImageAsset.Width > 0 && attrs.ImageAsset.Height > 0
}

// UploadAssetLibraryImage reserves, uploads, commits, and waits for an image.
// A partial result retains the reserved ID when a later operation fails.
// The caller validates the image and owns the overall upload timeout.
func (c *Client) UploadAssetLibraryImage(ctx context.Context, libraryID, fileName string, file *os.File, fileSize int64, requestContext RequestContextFunc) (AssetLibraryImageUploadResult, error) {
	return c.UploadAssetLibraryImageWithCategory(ctx, libraryID, fileName, file, fileSize, "CREATIVE_ASSETS", "", requestContext)
}

// UploadAssetLibraryImageWithCategory uploads an independently managed image.
// An empty referenceName leaves Apple's default reference name in place.
func (c *Client) UploadAssetLibraryImageWithCategory(ctx context.Context, libraryID, fileName string, file *os.File, fileSize int64, category, referenceName string, requestContext RequestContextFunc) (result AssetLibraryImageUploadResult, uploadErr error) {
	result = AssetLibraryImageUploadResult{LibraryID: libraryID, FileName: fileName, FileSize: fileSize, ReferenceName: referenceName, Category: category}
	if category != "CREATIVE_ASSETS" && category != "APP_SCREENSHOTS_AND_PREVIEWS" {
		return result, fmt.Errorf("unsupported asset category %q", category)
	}
	if file == nil {
		return result, fmt.Errorf("image file is required")
	}
	if strings.TrimSpace(libraryID) == "" {
		return result, fmt.Errorf("library ID is required")
	}
	payload := struct {
		Data struct {
			Type       string `json:"type"`
			Attributes struct {
				FileSize      int64  `json:"fileSize"`
				FileName      string `json:"fileName"`
				Category      string `json:"category"`
				ReferenceName string `json:"referenceName,omitempty"`
			} `json:"attributes"`
			Relationships struct {
				AssetLibrary Relationship `json:"assetLibrary"`
			} `json:"relationships"`
		} `json:"data"`
	}{}
	payload.Data.Type = "appAssetLibraryImages"
	payload.Data.Attributes.FileSize, payload.Data.Attributes.FileName, payload.Data.Attributes.Category, payload.Data.Attributes.ReferenceName = fileSize, fileName, category, referenceName
	payload.Data.Relationships.AssetLibrary.Data = ResourceData{Type: "appAssetLibraries", ID: libraryID}
	body, err := json.Marshal(payload)
	if err != nil {
		return result, err
	}
	stageCtx, stage := startImageUploadStage(ctx, "reserve")
	defer func() {
		status := "completed"
		if uploadErr != nil {
			status = "failed"
		}
		result.Diagnostics = append(result.Diagnostics, stage.emit(status, uploadErr))
	}()
	nextStage := func(name string) {
		result.Diagnostics = append(result.Diagnostics, stage.emit("completed", nil))
		stageCtx, stage = startImageUploadStage(ctx, name)
	}
	request := func(method, path string, body []byte) ([]byte, error) {
		requestCtx, cancel := requestContextFor(stageCtx, requestContext)
		defer cancel()
		return c.RawRequest(requestCtx, method, path, body)
	}
	raw, err := request("POST", "/v1/appAssetLibraryImages", body)
	if err != nil {
		return result, fmt.Errorf("reserve image: %w", err)
	}
	var reserved assetLibraryImageUploadResponse
	decodeErr := json.Unmarshal(raw, &reserved)
	result.ImageID = reserved.Data.ID
	result.State = reserved.Data.Attributes.State
	if decodeErr != nil {
		return result, fmt.Errorf("%w: %w", errImageUploadInvalidResponse, decodeErr)
	}
	if result.ImageID == "" || reserved.Data.Type != "appAssetLibraryImages" {
		return result, fmt.Errorf("%w: missing image resource identity", errImageUploadInvalidResponse)
	}
	nextStage("upload")
	if err := UploadAssetFromFile(stageCtx, file, fileSize, reserved.Data.Attributes.UploadOperations); err != nil {
		return result, fmt.Errorf("upload image %s: %w", result.ImageID, err)
	}
	path := "/v1/appAssetLibraryImages/" + url.PathEscape(result.ImageID)
	commit := struct {
		Data struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				Uploaded bool `json:"uploaded"`
			} `json:"attributes"`
		} `json:"data"`
	}{}
	commit.Data.Type, commit.Data.ID, commit.Data.Attributes.Uploaded = "appAssetLibraryImages", result.ImageID, true
	body, err = json.Marshal(commit)
	if err != nil {
		return result, err
	}
	nextStage("commit")
	raw, err = request("PATCH", path, body)
	var commitErr error
	if err != nil {
		commitErr = fmt.Errorf("commit image %s: %w", result.ImageID, err)
		if !isResponseBodyReadError(err) || ctx.Err() != nil {
			return result, commitErr
		}
		// Apple returned success but the response body was lost. Confirm
		// the reserved image's state before treating the upload as committed.
		raw, err = request("GET", path, nil)
		if err != nil {
			return result, commitErr
		}
	}
	var committed assetLibraryImageUploadResponse
	decodeErr = json.Unmarshal(raw, &committed)
	if commitErr != nil {
		if decodeErr != nil || committed.Data.ID != result.ImageID || committed.Data.Type != "appAssetLibraryImages" {
			return result, commitErr
		}
		switch committed.Data.Attributes.State {
		case "UPLOAD_COMPLETE", "COMPLETE", "PREPARE_FOR_SUBMISSION":
			result.State = committed.Data.Attributes.State
		default:
			return result, commitErr
		}
	}
	result.Uploaded = true
	nextStage("processing")
	if decodeErr == nil &&
		committed.Data.ID == result.ImageID && committed.Data.Type == "appAssetLibraryImages" &&
		committed.ready() && strings.TrimSpace(committed.Data.Attributes.SpecID) != "" {
		attrs := committed.Data.Attributes
		result.State, result.SpecID = attrs.State, attrs.SpecID
		result.Width, result.Height = attrs.ImageAsset.Width, attrs.ImageAsset.Height
		result.Ready = true
		return result, nil
	}
	_, err = PollMediaUntil(ctx, func(context.Context) (struct{}, bool, error) {
		raw, err := request("GET", path, nil)
		if err != nil {
			return struct{}{}, false, err
		}
		var current assetLibraryImageUploadResponse
		if err := json.Unmarshal(raw, &current); err != nil {
			return struct{}{}, false, fmt.Errorf("%w: %w", errImageUploadInvalidResponse, err)
		}
		if current.Data.ID != result.ImageID || current.Data.Type != "appAssetLibraryImages" {
			return struct{}{}, false, fmt.Errorf("%w: processing identity mismatch", errImageUploadInvalidResponse)
		}
		attrs := current.Data.Attributes
		result.State, result.SpecID = attrs.State, attrs.SpecID
		if attrs.State == "FAILED" {
			return struct{}{}, false, errImageProcessingFailed
		}
		if attrs.ImageAsset != nil {
			result.Width, result.Height = attrs.ImageAsset.Width, attrs.ImageAsset.Height
		}
		result.Ready = current.ready()
		if !result.Ready {
			stage.emit("started", nil)
		}
		return struct{}{}, result.Ready, nil
	}, PollOptions{Tolerate: IsTransientWaitError, OnToleratedFailure: func(err error, failures, max int) {
		stage.retry(err)
		if stage.observe == nil {
			logToleratedPollFailure(err, failures, max)
		}
	}})
	if err != nil {
		return result, fmt.Errorf("wait for image %s processing (state %q): %w", result.ImageID, result.State, err)
	}
	return result, nil
}
