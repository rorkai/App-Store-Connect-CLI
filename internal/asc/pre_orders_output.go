package asc

func endAppAvailabilityPreOrderRows(resp *EndAppAvailabilityPreOrderResponse) ([]string, [][]string) {
	headers := []string{"ID"}
	rows := [][]string{{resp.Data.ID}}
	return headers, rows
}

func printEndAppAvailabilityPreOrderTable(resp *EndAppAvailabilityPreOrderResponse) error {
	h, r := endAppAvailabilityPreOrderRows(resp)
	RenderTable(h, r)
	return nil
}

func printEndAppAvailabilityPreOrderMarkdown(resp *EndAppAvailabilityPreOrderResponse) error {
	h, r := endAppAvailabilityPreOrderRows(resp)
	RenderMarkdown(h, r)
	return nil
}
