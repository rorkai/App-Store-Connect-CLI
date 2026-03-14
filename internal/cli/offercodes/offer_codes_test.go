package offercodes

import "testing"

func TestOfferCodesCommandConstructors(t *testing.T) {
	constructors := []func() any{
		func() any { return OfferCodeCustomCodesCommand() },
		func() any { return OfferCodePricesCommand() },
		func() any { return OfferCodesGenerateCommand() },
		func() any { return OfferCodesValuesCommand() },
	}
	for _, ctor := range constructors {
		if got := ctor(); got == nil {
			t.Fatal("expected constructor to return command")
		}
	}
}
