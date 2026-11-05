package appleads

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"
)

const platformV1FixtureEndpointCount = 99

type platformV1FixtureContract struct {
	method          string
	path            string
	bodyKind        string
	bodyType        string
	sdkBodyRequired string
	responseType    string
	context         string
	destructive     string
	command         string
}

// TestPlatformV1EndpointContract is the aggregate guard for the independent
// Platform API v1 inventory. The lane tests cover their own implementation
// helpers; this test deliberately reads the fixture and the public aggregate
// so a lane cannot silently drift from the complete 99-operation contract.
func TestPlatformV1EndpointContract(t *testing.T) {
	fixture := readPlatformV1FixtureContracts(t)
	if got := len(fixture); got != platformV1FixtureEndpointCount {
		t.Fatalf("Platform API v1 fixture rows = %d, want %d", got, platformV1FixtureEndpointCount)
	}

	specs := PlatformEndpointSpecs()
	if got := len(specs); got != platformV1FixtureEndpointCount {
		t.Fatalf("PlatformEndpointSpecs() = %d, want %d", got, platformV1FixtureEndpointCount)
	}

	byCommand := make(map[string]EndpointSpec, len(specs))
	names := make(map[string]string, len(specs))
	methodPaths := make(map[string]string, len(specs))
	for _, spec := range specs {
		command := strings.Join(spec.CommandPath, " ")
		if command == "" {
			t.Fatal("PlatformEndpointSpecs() contains an empty command path")
		}
		if previous, ok := names[spec.Name]; ok {
			t.Fatalf("duplicate Platform API v1 endpoint name %q from %q and %q", spec.Name, previous, command)
		}
		names[spec.Name] = command

		methodPath := spec.Method + " " + spec.Path
		if previous, ok := methodPaths[methodPath]; ok {
			t.Fatalf("duplicate Platform API v1 method/path %q from %q and %q", methodPath, previous, command)
		}
		methodPaths[methodPath] = command

		if previous, ok := byCommand[command]; ok {
			t.Fatalf("duplicate Platform API v1 command path %q from %q and %q", command, previous.Name, spec.Name)
		}
		byCommand[command] = spec
	}

	for command, want := range fixture {
		spec, ok := byCommand[command]
		if !ok {
			t.Errorf("fixture command %q is missing from PlatformEndpointSpecs()", command)
			continue
		}
		assertPlatformV1FixtureContract(t, spec, want)
		delete(byCommand, command)
	}
	for command, spec := range byCommand {
		t.Errorf("PlatformEndpointSpecs() command %q is missing from fixture (endpoint %q)", command, spec.Name)
	}
}

func readPlatformV1FixtureContracts(t *testing.T) map[string]platformV1FixtureContract {
	t.Helper()

	file, err := os.Open("testdata/platform_v1_endpoints.tsv")
	if err != nil {
		t.Fatalf("open Platform API v1 fixture: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read Platform API v1 fixture: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("Platform API v1 fixture is empty")
	}
	if got, want := strings.Join(records[0], "\t"), "collection\toperation\ttitle\tmethod\tpath\traw_parameters\tbody_kind\tbody_type\tsdk_body_required\tresponse\tcontext\tdestructive\tcommand\tsource"; got != want {
		t.Fatalf("Platform API v1 fixture header = %q, want %q", got, want)
	}

	contracts := make(map[string]platformV1FixtureContract, len(records)-1)
	for rowNumber, record := range records[1:] {
		line := rowNumber + 2
		if len(record) != 14 {
			t.Fatalf("Platform API v1 fixture row %d has %d columns, want 14: %#v", line, len(record), record)
		}
		command := strings.TrimSpace(record[12])
		if command == "" {
			t.Fatalf("Platform API v1 fixture row %d has an empty command", line)
		}
		if previous, ok := contracts[command]; ok {
			t.Fatalf("duplicate Platform API v1 fixture command %q (paths %q and %q)", command, previous.path, record[4])
		}
		contracts[command] = platformV1FixtureContract{
			method:          strings.TrimSpace(record[3]),
			path:            strings.TrimPrefix(strings.TrimSpace(record[4]), "/"),
			bodyKind:        strings.TrimSpace(record[6]),
			bodyType:        strings.TrimSpace(record[7]),
			sdkBodyRequired: strings.TrimSpace(record[8]),
			responseType:    strings.TrimPrefix(strings.TrimSpace(record[9]), "200 "),
			context:         strings.TrimSpace(record[10]),
			destructive:     strings.TrimSpace(record[11]),
			command:         command,
		}
	}
	return contracts
}

func assertPlatformV1FixtureContract(t *testing.T, spec EndpointSpec, want platformV1FixtureContract) {
	t.Helper()

	if spec.Version != APIVersionPlatformV1 {
		t.Errorf("%q version = %q, want %q", want.command, spec.Version, APIVersionPlatformV1)
	}
	if spec.Method != want.method {
		t.Errorf("%q method = %q, want %q", want.command, spec.Method, want.method)
	}
	if spec.Path != want.path {
		t.Errorf("%q path = %q, want %q", want.command, spec.Path, want.path)
	}

	wantBodyKind, ok := map[string]BodyKind{
		"none":                BodyNone,
		"JSON object":         BodyObject,
		"JSON array":          BodyArray,
		"multipart/form-data": BodyMultipart,
	}[want.bodyKind]
	if !ok {
		t.Errorf("%q fixture body kind %q is unsupported", want.command, want.bodyKind)
	} else if spec.BodyKind != wantBodyKind {
		t.Errorf("%q body kind = %q, want %q", want.command, spec.BodyKind, wantBodyKind)
	}

	wantBodyType := want.bodyType
	if wantBodyType == "none" {
		wantBodyType = ""
	}
	if spec.BodyType != wantBodyType {
		t.Errorf("%q body type = %q, want %q", want.command, spec.BodyType, wantBodyType)
	}
	if spec.ResponseType != want.responseType {
		t.Errorf("%q response type = %q, want %q", want.command, spec.ResponseType, want.responseType)
	}

	wantContext, ok := map[string]ContextKind{
		"none":                ContextNone,
		"ad-account":          ContextAdAccount,
		"optional-ad-account": ContextAdAccountOptional,
	}[want.context]
	if !ok {
		t.Errorf("%q fixture context %q is unsupported", want.command, want.context)
	} else if spec.Context != wantContext {
		t.Errorf("%q context = %v, want %v", want.command, spec.Context, wantContext)
	}
	if spec.RequiresConfirm != (want.destructive == "yes") {
		t.Errorf("%q RequiresConfirm = %t, want %t", want.command, spec.RequiresConfirm, want.destructive == "yes")
	}

	isQuery := spec.Method == "POST" && strings.HasSuffix(spec.Path, "/query")
	// Apple serves geolocation resolution as a read-only POST search.
	if (isQuery || want.command == "geo resolve") && !spec.RetrySafe {
		t.Errorf("%q read-only POST must be retry-safe", want.command)
	}

	// Apple’s SDK marks some request bodies optional, but the CLI keeps
	// mutation payloads required. The only deliberately optional CLI bodies
	// are SDK-optional POST query requests.
	wantOptional := isQuery && want.sdkBodyRequired == "no"
	if spec.BodyKind == BodyNone {
		wantOptional = false
	}
	if spec.BodyOptional != wantOptional {
		t.Errorf("%q BodyOptional = %t, want %t (SDK body required = %q)", want.command, spec.BodyOptional, wantOptional, want.sdkBodyRequired)
	}
}
