package shared

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const targetSubscriptionPlanTypeFlagName = "target-subscription-plan-type"

// NormalizeSubscriptionPlanType validates a plan type, naming source in the error.
func NormalizeSubscriptionPlanType(value, source string) (asc.SubscriptionPlanType, error) {
	switch planType := asc.SubscriptionPlanType(strings.ToUpper(strings.TrimSpace(value))); planType {
	case asc.SubscriptionPlanTypeMonthly, asc.SubscriptionPlanTypeUpfront:
		return planType, nil
	default:
		return "", fmt.Errorf("%s must be one of: MONTHLY, UPFRONT", source)
	}
}

// BindTargetSubscriptionPlanTypeFlag registers the optional flag shared by the
// subscription offer create commands. The returned function validates the value
// and yields an empty plan type only when the flag was not provided.
func BindTargetSubscriptionPlanTypeFlag(fs *flag.FlagSet) func() (asc.SubscriptionPlanType, error) {
	return bindTargetSubscriptionPlanTypeFlagTo(os.Stderr, fs)
}

// BindTargetSubscriptionPlanTypeFlag is BindTargetSubscriptionPlanTypeFlag
// with the validation's "Error:" line written to d's stderr.
func (d *Deps) BindTargetSubscriptionPlanTypeFlag(fs *flag.FlagSet) func() (asc.SubscriptionPlanType, error) {
	return bindTargetSubscriptionPlanTypeFlagTo(d.IO.ErrOut, fs)
}

func bindTargetSubscriptionPlanTypeFlagTo(stderr io.Writer, fs *flag.FlagSet) func() (asc.SubscriptionPlanType, error) {
	value := fs.String(targetSubscriptionPlanTypeFlagName, "", "Target subscription plan type: MONTHLY, UPFRONT")
	return func() (asc.SubscriptionPlanType, error) {
		provided := false
		fs.Visit(func(parsed *flag.Flag) {
			provided = provided || parsed.Name == targetSubscriptionPlanTypeFlagName
		})
		if !provided {
			return "", nil
		}
		if strings.TrimSpace(*value) == "" {
			return "", usageErrorfTo(stderr, "invalid value for --%s: cannot be empty", targetSubscriptionPlanTypeFlagName)
		}
		planType, err := NormalizeSubscriptionPlanType(*value, "--"+targetSubscriptionPlanTypeFlagName)
		if err != nil {
			return "", usageErrorTo(stderr, err.Error())
		}
		return planType, nil
	}
}
