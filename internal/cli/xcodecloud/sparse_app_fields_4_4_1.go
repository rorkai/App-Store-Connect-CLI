package xcodecloud

import "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"

var ciProductAppInAppPurchaseSparseFields441 = []string{"versions"}

var ciProductAppSubscriptionGroupSparseFields441 = []string{"versions"}

func addCiProductAppInclude(values []string, include string) []string {
	if !shared.HasInclude(values, include) {
		values = append(values, include)
	}
	return values
}
