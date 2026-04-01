package main

import "sync"

const studioSubprocessConcurrency = 6

func boundedStudioConcurrency(total int) int {
	if total <= 0 {
		return 1
	}
	if total < studioSubprocessConcurrency {
		return total
	}
	return studioSubprocessConcurrency
}

func runWithConcurrency(limit, total int, work func(int)) {
	if total <= 0 {
		return
	}
	if limit <= 0 {
		limit = 1
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, limit)

	for i := 0; i < total; i++ {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			work(idx)
		}(i)
	}

	wg.Wait()
}
