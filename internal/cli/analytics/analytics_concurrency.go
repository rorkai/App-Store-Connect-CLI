package analytics

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const analyticsInstanceFetchConcurrency = 4

var (
	analyticsInstanceFetchMaxInFlight atomic.Int32
	fetchAnalyticsReportInstancesFn   = fetchAnalyticsReportInstances
)

func collectAnalyticsReports(ctx context.Context, client *asc.Client, reports []asc.Resource[asc.AnalyticsReportAttributes], instanceOpts []asc.AnalyticsReportInstancesOption, includeSegments bool, processingDateFilter string) ([]asc.AnalyticsReportGetReport, int, error) {
	results := make([]asc.AnalyticsReportGetReport, len(reports))
	include := make([]bool, len(reports))
	counts := make([]int, len(reports))
	sem := make(chan struct{}, analyticsInstanceFetchConcurrency)
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var firstErr error
	var firstErrOnce sync.Once
	recordError := func(err error) {
		if err == nil {
			return
		}
		firstErrOnce.Do(func() {
			firstErr = err
			cancel()
		})
	}
	var wg sync.WaitGroup
	var inFlight atomic.Int32
	for index, report := range reports {
		wg.Add(1)
		go func(index int, report asc.Resource[asc.AnalyticsReportAttributes]) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-workCtx.Done():
				return
			}
			current := inFlight.Add(1)
			for {
				seen := analyticsInstanceFetchMaxInFlight.Load()
				if current <= seen || analyticsInstanceFetchMaxInFlight.CompareAndSwap(seen, current) {
					break
				}
			}
			defer func() {
				inFlight.Add(-1)
				<-sem
			}()
			reportResult, count, err := analyticsReportResult(workCtx, client, report, instanceOpts, includeSegments)
			if err != nil {
				recordError(err)
				return
			}
			results[index] = reportResult
			counts[index] = count
			include[index] = processingDateFilter == "" || count != 0
		}(index, report)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, 0, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	collected := make([]asc.AnalyticsReportGetReport, 0, len(reports))
	total := 0
	for index := range reports {
		total += counts[index]
		if include[index] {
			collected = append(collected, results[index])
		}
	}
	return collected, total, nil
}

func analyticsReportResult(ctx context.Context, client *asc.Client, report asc.Resource[asc.AnalyticsReportAttributes], instanceOpts []asc.AnalyticsReportInstancesOption, includeSegments bool) (asc.AnalyticsReportGetReport, int, error) {
	instances, err := fetchAnalyticsReportInstancesFn(ctx, client, report.ID, instanceOpts...)
	if err != nil {
		return asc.AnalyticsReportGetReport{}, 0, fmt.Errorf("analytics view: failed to fetch instances: %w", err)
	}
	reportResult := asc.AnalyticsReportGetReport{
		ID:          report.ID,
		ReportType:  report.Attributes.ReportType,
		Name:        report.Attributes.Name,
		Category:    report.Attributes.Category,
		Granularity: report.Attributes.Granularity,
	}
	for _, instance := range instances {
		instanceResult := asc.AnalyticsReportGetInstance{
			ID:             instance.ID,
			ReportDate:     instance.Attributes.ReportDate,
			ProcessingDate: instance.Attributes.ProcessingDate,
			Granularity:    instance.Attributes.Granularity,
			Version:        instance.Attributes.Version,
		}
		if includeSegments {
			segments, err := fetchAnalyticsReportSegments(ctx, client, instance.ID)
			if err != nil {
				return asc.AnalyticsReportGetReport{}, 0, fmt.Errorf("analytics view: failed to fetch segments: %w", err)
			}
			for _, segment := range segments {
				instanceResult.Segments = append(instanceResult.Segments, asc.AnalyticsReportGetSegment{
					ID:                segment.ID,
					DownloadURL:       segment.Attributes.URL,
					Checksum:          segment.Attributes.Checksum,
					SizeInBytes:       segment.Attributes.SizeInBytes,
					URLExpirationDate: segment.Attributes.URLExpirationDate,
				})
			}
		}
		reportResult.Instances = append(reportResult.Instances, instanceResult)
	}
	return reportResult, len(instances), nil
}
