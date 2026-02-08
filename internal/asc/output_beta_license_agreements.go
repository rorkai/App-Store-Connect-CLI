package asc

func betaLicenseAgreementAppID(resource BetaLicenseAgreementResource) string {
	if resource.Relationships == nil || resource.Relationships.App == nil {
		return ""
	}
	return resource.Relationships.App.Data.ID
}

func betaLicenseAgreementsRows(resp *BetaLicenseAgreementsResponse) ([]string, [][]string) {
	headers := []string{"ID", "App ID", "Agreement Text"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			betaLicenseAgreementAppID(item),
			compactWhitespace(item.Attributes.AgreementText),
		})
	}
	return headers, rows
}

func printBetaLicenseAgreementsTable(resp *BetaLicenseAgreementsResponse) error {
	h, r := betaLicenseAgreementsRows(resp)
	RenderTable(h, r)
	return nil
}

func printBetaLicenseAgreementsMarkdown(resp *BetaLicenseAgreementsResponse) error {
	h, r := betaLicenseAgreementsRows(resp)
	RenderMarkdown(h, r)
	return nil
}
