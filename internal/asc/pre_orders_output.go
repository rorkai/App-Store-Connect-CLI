package asc

func endAppAvailabilityPreOrderRows(resp *EndAppAvailabilityPreOrderResponse) ([]string, [][]string) {
	headers := []string{"ID"}
	rows := [][]string{{resp.Data.ID}}
	return headers, rows
}
