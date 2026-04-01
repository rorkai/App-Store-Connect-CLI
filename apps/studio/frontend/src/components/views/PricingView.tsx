import { PricingOverviewState } from "../../types";
import { fmt } from "../../utils";

type PricingViewProps = {
  pricingOverview: PricingOverviewState;
};

export function PricingView({ pricingOverview }: PricingViewProps) {
  return (
    <div className="app-detail-view">
      <div className="app-detail-section">
        <h3 className="section-label">Pricing and Availability</h3>
        {pricingOverview.loading ? (
          <p className="empty-hint">Loading…</p>
        ) : pricingOverview.error ? (
          <p className="empty-hint">{pricingOverview.error}</p>
        ) : (
          <>
            <table className="data-table" style={{ marginBottom: 20 }}>
              <tbody>
                <tr>
                  <td className="vcard-label">Current Price</td>
                  <td style={{ fontWeight: 600 }}>
                    {pricingOverview.currentPrice === "0.0" || pricingOverview.currentPrice === "0.00"
                      ? "Free"
                      : pricingOverview.currentPrice
                        ? `${pricingOverview.baseCurrency} $${pricingOverview.currentPrice}`
                        : "\u2014"}
                  </td>
                </tr>
                {pricingOverview.currentPrice && pricingOverview.currentPrice !== "0.0" && pricingOverview.currentPrice !== "0.00" && (
                  <tr>
                    <td className="vcard-label">Proceeds</td>
                    <td>{pricingOverview.baseCurrency} ${pricingOverview.currentProceeds}</td>
                  </tr>
                )}
                <tr>
                  <td className="vcard-label">Available in New Territories</td>
                  <td>{pricingOverview.availableInNewTerritories ? "Yes" : "No"}</td>
                </tr>
                <tr>
                  <td className="vcard-label">Territories</td>
                  <td>{pricingOverview.territories.filter((t) => t.available).length} available / {pricingOverview.territories.length} total</td>
                </tr>
              </tbody>
            </table>

            {pricingOverview.territories.length > 0 && (
              <>
                <h3 className="section-label">Territory Availability</h3>
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>Territory</th>
                      <th>Price</th>
                      <th>Available</th>
                      <th>Release Date</th>
                    </tr>
                  </thead>
                  <tbody>
                    {pricingOverview.territories.map((t) => (
                      <tr key={t.territory}>
                        <td>{t.territory}</td>
                        <td>{pricingOverview.currentPrice === "0.0" || pricingOverview.currentPrice === "0.00" ? "Free" : `$${pricingOverview.currentPrice}`}</td>
                        <td>{t.available ? "Yes" : "No"}</td>
                        <td>{t.releaseDate || "\u2014"}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </>
            )}

            {pricingOverview.subscriptionPricing.length > 0 && (
              <>
                <h3 className="section-label">Subscription Prices</h3>
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>Group</th>
                      <th>Name</th>
                      <th>Period</th>
                      <th>Price</th>
                      <th>Proceeds</th>
                      <th>Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {pricingOverview.subscriptionPricing.map((s) => (
                      <tr key={s.productId}>
                        <td>{s.groupName}</td>
                        <td>{s.name}</td>
                        <td>{fmt(s.subscriptionPeriod)}</td>
                        <td>{s.currency} {s.price}</td>
                        <td>{s.currency} {s.proceeds}</td>
                        <td><span className={`status-pill status-${s.state.toLowerCase()}`}>{fmt(s.state)}</span></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </>
            )}
          </>
        )}
      </div>
    </div>
  );
}
