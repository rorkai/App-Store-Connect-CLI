package asc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestLegacy441ClientMethodsHaveDeprecationDocs(t *testing.T) {
	want := map[string]string{
		"GetInAppPurchaseLocalizations":                                   "Use GetInAppPurchaseVersionLocalizations with an in-app purchase version ID.",
		"CreateInAppPurchaseLocalization":                                 "Use CreateInAppPurchaseLocalizationV2 with an in-app purchase version ID.",
		"UpdateInAppPurchaseLocalization":                                 "Use UpdateInAppPurchaseLocalizationV2.",
		"DeleteInAppPurchaseLocalization":                                 "Use DeleteInAppPurchaseLocalizationV2.",
		"GetInAppPurchaseLocalization":                                    "Use GetInAppPurchaseLocalizationV2.",
		"GetInAppPurchaseInAppPurchaseLocalizationsRelationships":         "Use GetInAppPurchaseVersionLocalizationsRelationships with an in-app purchase version ID.",
		"GetInAppPurchaseImages":                                          "Use GetInAppPurchaseVersionImages with an in-app purchase version ID.",
		"GetInAppPurchaseImage":                                           "Use GetInAppPurchaseImageV2.",
		"CreateInAppPurchaseImage":                                        "Use CreateInAppPurchaseImageV2 with an in-app purchase version ID.",
		"UpdateInAppPurchaseImage":                                        "Create a replacement with CreateInAppPurchaseImageV2; UpdateInAppPurchaseImageV2 only commits upload state.",
		"DeleteInAppPurchaseImage":                                        "Use DeleteInAppPurchaseImageV2.",
		"GetInAppPurchaseImagesRelationships":                             "Use GetInAppPurchaseVersionImagesRelationships with an in-app purchase version ID.",
		"CreateInAppPurchaseSubmission":                                   "Create an in-app purchase version and add it with CreateReviewSubmissionItem.",
		"GetSubscriptionLocalizations":                                    "Use GetSubscriptionVersionLocalizations with a subscription version ID.",
		"GetSubscriptionLocalization":                                     "Use GetSubscriptionLocalizationV2.",
		"CreateSubscriptionLocalization":                                  "Use CreateSubscriptionLocalizationV2 with a subscription version ID.",
		"UpdateSubscriptionLocalization":                                  "Use UpdateSubscriptionLocalizationV2.",
		"DeleteSubscriptionLocalization":                                  "Use DeleteSubscriptionLocalizationV2.",
		"GetSubscriptionSubscriptionLocalizationsRelationships":           "Use GetSubscriptionVersionLocalizationsRelationships with a subscription version ID.",
		"GetSubscriptionImages":                                           "Use GetSubscriptionVersionImages with a subscription version ID.",
		"GetSubscriptionImage":                                            "Use GetSubscriptionImageV2.",
		"CreateSubscriptionImage":                                         "Use CreateSubscriptionImageV2 with a subscription version ID.",
		"UpdateSubscriptionImage":                                         "Use CreateSubscriptionImageV2 for a new upload and UpdateSubscriptionImageV2 to commit its upload state.",
		"DeleteSubscriptionImage":                                         "Use DeleteSubscriptionImageV2.",
		"GetSubscriptionImagesRelationships":                              "Use GetSubscriptionVersionImagesRelationships with a subscription version ID.",
		"CreateSubscriptionSubmission":                                    "Create a subscription version and add it with CreateReviewSubmissionItem.",
		"CreateSubscriptionGroupSubmission":                               "Create a subscription group version and add it with CreateReviewSubmissionItem.",
		"GetSubscriptionGroupLocalizations":                               "Use GetSubscriptionGroupVersionLocalizations with a subscription group version ID.",
		"GetSubscriptionGroupLocalization":                                "Use GetSubscriptionGroupLocalizationV2.",
		"CreateSubscriptionGroupLocalization":                             "Use CreateSubscriptionGroupLocalizationV2 with a subscription group version ID.",
		"UpdateSubscriptionGroupLocalization":                             "Use UpdateSubscriptionGroupLocalizationV2.",
		"DeleteSubscriptionGroupLocalization":                             "Use DeleteSubscriptionGroupLocalizationV2.",
		"GetSubscriptionGroupSubscriptionGroupLocalizationsRelationships": "Use GetSubscriptionGroupVersionLocalizationsRelationships with a subscription group version ID.",
	}
	verified := make(map[string]bool, len(want))
	for method, guidance := range want {
		if strings.TrimSpace(guidance) == "" {
			t.Fatalf("expected replacement guidance for %s must not be empty", method)
		}
	}

	files := []string{
		"client_iap.go",
		"client_iap_subresources.go",
		"client_subscription_resources.go",
		"client_subscriptions.go",
	}
	fset := token.NewFileSet()
	for _, path := range files {
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || !isExportedClientMethod(function) {
				continue
			}

			guidance, deprecated := deprecatedGuidance(function)
			expected, tracked := want[function.Name.Name]
			if deprecated && !tracked {
				t.Errorf("unexpected deprecated Client method %s; add it to the App Store Connect API 4.4.1 inventory", function.Name.Name)
				continue
			}
			if !tracked {
				continue
			}
			if !deprecated {
				t.Errorf("%s must name its App Store Connect API 4.4.1 replacement in a Deprecated doc comment", function.Name.Name)
				continue
			}
			if guidance != expected {
				t.Errorf("%s deprecation guidance = %q, want %q", function.Name.Name, guidance, expected)
				continue
			}
			verified[function.Name.Name] = true
		}
	}

	for method := range want {
		if !verified[method] {
			t.Errorf("did not verify deprecation docs for %s", method)
		}
	}
}

func isExportedClientMethod(function *ast.FuncDecl) bool {
	if function == nil || function.Recv == nil || !function.Name.IsExported() || len(function.Recv.List) != 1 {
		return false
	}

	receiver := function.Recv.List[0].Type
	if pointer, ok := receiver.(*ast.StarExpr); ok {
		receiver = pointer.X
	}
	identifier, ok := receiver.(*ast.Ident)
	return ok && identifier.Name == "Client"
}

func deprecatedGuidance(function *ast.FuncDecl) (string, bool) {
	if function == nil || function.Doc == nil {
		return "", false
	}

	const marker = "Deprecated:"
	doc := function.Doc.Text()
	index := strings.Index(doc, marker)
	if index < 0 {
		return "", false
	}
	return strings.TrimSpace(doc[index+len(marker):]), true
}
