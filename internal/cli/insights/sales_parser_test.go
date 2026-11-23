package insights

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const salesParserHeader = "SKU\tApple Identifier\tParent Identifier\tProduct Type Identifier\tSubscription\tUnits\tDeveloper Proceeds\tCustomer Price\n"

func TestSalesParserPreservesScopeAndOrder(t *testing.T) {
	report := salesParserHeader + "iap\t456\tAPP\tIAY\tRenewal\t10000000000000000\t1\t2\n" +
		"iap\t456\tAPP\tIAY\tNew\t-10000000000000000\t1\t2\n" +
		"other\t789\tOTHER\t1\t\t500\t500\t500\n" +
		"APP\t123\t\t1\t\t1\t1\t2\n"
	for _, sku := range []string{"APP", "", "  "} {
		t.Run(fmt.Sprintf("sku=%q", sku), func(t *testing.T) {
			metrics, err := ParseSalesReportMetrics(bytes.NewReader(gzipText(t, report)), SalesScope{AppID: "123", AppSKU: sku})
			if err != nil {
				t.Fatal(err)
			}
			if metrics.RowCount != 3 || metrics.UnitsTotal != 1 || metrics.MonetizedUnitsTotal != 0 || metrics.DownloadUnitsTotal != 1 || metrics.DeveloperProceedsByCurrency[""] != 1 || metrics.SubscriptionRows != 2 || metrics.RenewalRows != 1 {
				t.Fatalf("unexpected ordered app/IAP metrics: %+v", metrics)
			}
		})
	}
}

func TestSalesParserFullStreamErrorsPrecedeHeaderValidation(t *testing.T) {
	for _, sku := range []string{"APP", ""} {
		for _, headers := range []string{salesParserHeader, "SKU\tUnits\n", ""} {
			t.Run(fmt.Sprintf("sku=%q/header=%q", sku, headers), func(t *testing.T) {
				report := headers
				if headers != "" {
					report += "APP\t123\t\t1\t\t2\t3\t4\n"
				}
				compressed := gzipText(t, report)
				compressed[len(compressed)-8] ^= 1 // Corrupt CRC after otherwise valid rows.
				metrics, err := ParseSalesReportMetrics(bytes.NewReader(compressed), SalesScope{AppID: "123", AppSKU: sku})
				if err == nil || !strings.Contains(err.Error(), "parse report rows:") {
					t.Fatalf("expected row-read error before header validation, got %v", err)
				}
				if !reflect.DeepEqual(metrics, SalesMetrics{}) {
					t.Fatalf("partial metrics escaped a corrupt stream: %+v", metrics)
				}
			})
		}
	}
}

func TestSalesParserEmptyAndHeaderOnly(t *testing.T) {
	for _, sku := range []string{"APP", ""} {
		for _, tc := range []struct{ report, wantErr string }{
			{"", "report is empty"},
			{"SKU\tUnits\n", "report is missing Apple Identifier and Parent Identifier columns"},
			{salesParserHeader, ""},
		} {
			t.Run(fmt.Sprintf("sku=%q/error=%q", sku, tc.wantErr), func(t *testing.T) {
				metrics, err := ParseSalesReportMetrics(bytes.NewReader(gzipText(t, tc.report)), SalesScope{AppID: "123", AppSKU: sku})
				if tc.wantErr != "" {
					if err == nil || err.Error() != tc.wantErr {
						t.Fatalf("error=%v, want %q", err, tc.wantErr)
					}
					if !reflect.DeepEqual(metrics, SalesMetrics{}) {
						t.Fatalf("unexpected metrics: %+v", metrics)
					}
				} else if err != nil || metrics.RowCount != 0 || !metrics.UnitsColumnPresent || !metrics.DownloadUnitsAvailable {
					t.Fatalf("metrics=%+v err=%v", metrics, err)
				}
			})
		}
	}
}

func BenchmarkParseSalesReportMetrics(b *testing.B) {
	for _, rows := range []int{10000, 100000} {
		var report strings.Builder
		report.WriteString(salesParserHeader)
		for i := range rows {
			parent := "OTHER"
			if i%100 == 0 {
				parent = "APP"
			}
			fmt.Fprintf(&report, "iap\t456\t%s\tIAY\tRenewal\t3\t1.25\t2.50\n", parent)
		}
		report.WriteString("APP\t123\t\t1\t\t2\t0\t0\n") // Late SKU discovery.
		var compressed bytes.Buffer
		writer := gzip.NewWriter(&compressed)
		if _, err := writer.Write([]byte(report.String())); err != nil {
			b.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			b.Fatal(err)
		}
		for _, sku := range []string{"APP", ""} {
			label := "knownSKU"
			if sku == "" {
				label = "unknownSKU"
			}
			b.Run(fmt.Sprintf("rows=%d/%s", rows, label), func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(report.Len()))
				for range b.N {
					metrics, err := ParseSalesReportMetrics(bytes.NewReader(compressed.Bytes()), SalesScope{AppID: "123", AppSKU: sku})
					if err != nil || metrics.RowCount != rows/100+1 || metrics.UnitsTotal != float64(rows/100*3+2) {
						b.Fatalf("metrics=%+v err=%v", metrics, err)
					}
				}
			})
		}
	}
}
