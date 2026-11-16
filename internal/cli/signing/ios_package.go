package signing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/iosbuild"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"howett.net/plist"
)

// IOSPackageOptions selects explicit local signing material, never remote provisioning.
type IOSPackageOptions struct {
	AppPath, IPAPath, IdentityPath, PasswordPath, ProfilePath, EntitlementsPath string
	AdHoc                                                                       bool
	LogWriter                                                                   io.Writer
}

// PackageIOSApp signs one fresh device app with rcodesign and packages an IPA.
// It does not claim complete Apple signature verification or Apple acceptance.
var removeIOSPackageStage = os.RemoveAll

func PackageIOSApp(ctx context.Context, d *shared.Deps, opts IOSPackageOptions) (result *asc.IOSArtifactResult, resultErr error) {
	ctx, stop := iosbuild.ContextWithSignals(ctx)
	defer stop()
	started := d.Now()
	result = &asc.IOSArtifactResult{Operation: "package", Backend: "rcodesign", Platform: "device", SigningType: "adHoc", AppleAcceptance: "notVerified"}
	defer func() { result.DurationMs = d.Now().Sub(started).Milliseconds() }()
	if !opts.AdHoc && runtime.GOOS == "windows" {
		return result, fmt.Errorf("signing with an identity requires Linux or macOS")
	}
	destination, err := filepath.Abs(opts.IPAPath)
	if err != nil {
		return result, err
	}
	outputRoot, err := rootfs.New(filepath.Dir(destination))
	if err != nil {
		return result, err
	}
	defer outputRoot.Close()
	if err := outputRoot.CheckCreateNewFile(filepath.Base(destination)); err != nil {
		return result, fmt.Errorf("IPA destination must not exist: %w", err)
	}
	stage, err := os.MkdirTemp("", "asc-ios-package-")
	if err != nil {
		return result, err
	}
	defer func() {
		if err := removeIOSPackageStage(stage); err != nil {
			result.Success = false
			resultErr = errors.Join(resultErr, fmt.Errorf("private signing stage cleanup failed at %s; inspect IPA destination before retry: %w", stage, err))
		}
	}()
	app := filepath.Join(stage, filepath.Base(opts.AppPath))
	if err := iosbuild.CopyBundle(ctx, opts.AppPath, app); err != nil {
		return result, err
	}
	info, err := iosbuild.ReadAppInfo(app)
	if err != nil {
		return result, err
	}
	if err := iosbuild.ValidateDeviceApp(app, info); err != nil {
		return result, err
	}
	bundleID := info["CFBundleIdentifier"].(string)
	stageRoot, err := rootfs.New(stage)
	if err != nil {
		return result, err
	}
	defer stageRoot.Close()
	requested := map[string]any{}
	if opts.EntitlementsPath != "" {
		data, err := readBoundedSigningRunFile(opts.EntitlementsPath, infoplist.MaxBytes, false)
		if err != nil {
			return result, fmt.Errorf("read requested entitlements failed")
		}
		if err := infoplist.ValidateStructure(data); err != nil {
			return result, fmt.Errorf("invalid requested entitlements: %w", err)
		}
		if _, err := plist.Unmarshal(data, &requested); err != nil {
			return result, fmt.Errorf("decode requested entitlements: %w", err)
		}
		if requested == nil {
			requested = map[string]any{}
		}
	}
	if err := validateSigningResignExistingEntitlements(requested, bundleID); err != nil {
		return result, err
	}
	args := []string{"--config-file", os.DevNull, "sign", "--timestamp-url", "none"}
	var signerInputs [][]byte
	if !opts.AdHoc {
		profileData, err := readBoundedSigningRunFile(opts.ProfilePath, signingResignProfileMaxBytes, false)
		if err != nil {
			return result, fmt.Errorf("read provisioning profile failed")
		}
		defer clear(profileData)
		profile, err := inspectSigningResignProfile(profileData, d.Now())
		if err != nil {
			return result, err
		}
		if err := validateSigningResignProfileForTarget(profile, bundleID); err != nil {
			return result, err
		}
		identityData, err := readBoundedSigningRunFile(opts.IdentityPath, signingRunInputLimit, true)
		if err != nil {
			return result, fmt.Errorf("read signing identity failed")
		}
		defer clear(identityData)
		var password []byte
		if opts.PasswordPath != "" {
			password, err = readBoundedSigningRunFile(opts.PasswordPath, signingRunPasswordLimit, true)
			if err != nil {
				return result, fmt.Errorf("read signing password failed")
			}
			defer clear(password)
		}
		password = bytes.TrimSuffix(bytes.TrimSuffix(password, []byte("\n")), []byte("\r"))
		identity, err := inspectSigningRunIdentity(identityData, password, d.Now())
		if err != nil {
			return result, fmt.Errorf("inspect signing identity failed")
		}
		if err := validateSigningResignProfileIdentity(profile, identity); err != nil {
			return result, err
		}
		requested, err = buildSigningResignEntitlementsForProfile(requested, profile)
		if err != nil {
			return result, err
		}
		// rcodesign reads the validated identity and password through pipes, so
		// neither they nor a decrypted key are written to disk. It uses only the
		// first line of the password input.
		if bytes.ContainsAny(password, "\r\n") {
			return result, fmt.Errorf("signing password must be a single line")
		}
		passwordLine := append(append([]byte(nil), password...), '\n')
		defer clear(passwordLine)
		signerInputs = [][]byte{identityData, passwordLine}
		args = append(args, "--p12-file", "/dev/fd/3", "--p12-password-file", "/dev/fd/4")
		appRoot, err := rootfs.New(app)
		if err != nil {
			return result, err
		}
		err = appRoot.WriteFile("embedded.mobileprovision", profileData, 0o644)
		appRoot.Close()
		if err != nil {
			return result, fmt.Errorf("stage provisioning profile failed")
		}
		args = append(args, "--team-name", profile.TeamID)
		result.SigningType = profile.Class
		result.ProfileValidated = true
	} else {
		// Ad hoc output must not carry a previous distribution profile.
		appRoot, err := rootfs.New(app)
		if err != nil {
			return result, err
		}
		rooted, err := appRoot.OpenRoot()
		appRoot.Close()
		if err != nil {
			return result, err
		}
		_, err = rooted.Lstat("embedded.mobileprovision")
		rooted.Close()
		if err == nil {
			return result, fmt.Errorf("ad hoc input must not contain a provisioning profile")
		}
		if !errors.Is(err, os.ErrNotExist) {
			return result, err
		}
	}
	entitlements, err := marshalSigningResignEntitlements(requested)
	if len(requested) == 0 {
		// rcodesign otherwise imports optional claims from a previous signature.
		// The shared macOS helper intentionally returns no bytes for this case.
		entitlements, err = plist.MarshalIndent(requested, plist.XMLFormat, "\t")
	}
	if err != nil {
		return result, fmt.Errorf("encode signing entitlements: %w", err)
	}
	if err := stageRoot.WriteFile("entitlements.plist", entitlements, 0o600); err != nil {
		return result, err
	}
	args = append(args, "--entitlements-xml-file", filepath.Join(stage, "entitlements.plist"))
	args = append(args, app)
	// The private staging tree has no rcodesign configuration or inherited ASC auth.
	if err := iosbuild.RunSigner(ctx, stage, opts.LogWriter, signerInputs, "rcodesign", args...); err != nil {
		return result, err
	}
	archive := filepath.Join(stage, "output.ipa")
	if err := iosbuild.WriteIPA(ctx, app, archive); err != nil {
		return result, err
	}
	f, err := stageRoot.OpenFile("output.ipa")
	if err != nil {
		return result, err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return result, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if _, err := outputRoot.CreateNewFrom(filepath.Base(destination), f, 0o644); err != nil {
		return result, fmt.Errorf("publish IPA (inspect destination before retry): %w", err)
	}
	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("IPA created at %s; inspect before retry: %w", destination, err)
	}
	result.IPAPath = destination
	result.BundleID = bundleID
	result.SHA256 = hex.EncodeToString(hash.Sum(nil))
	result.Success = true
	return result, nil
}
