import { OfferCodesState } from "../../types";
import { fmt } from "../../utils";

type PromoCodesViewProps = {
  offerCodes: OfferCodesState;
};

export function PromoCodesView({ offerCodes }: PromoCodesViewProps) {
  return (
    <div className="app-detail-view">
      <div className="app-detail-section">
        <h3 className="section-label">Offer Codes</h3>
        {offerCodes.loading ? (
          <p className="empty-hint">Loading…</p>
        ) : offerCodes.codes.length === 0 && offerCodes.error ? (
          <p className="empty-hint">{offerCodes.error}</p>
        ) : offerCodes.codes.length === 0 ? (
          <p className="empty-hint">No offer codes found for this app's subscriptions.</p>
        ) : (
          <>
            {offerCodes.error && <p className="empty-hint">{offerCodes.error}</p>}
            <div className="section-header-row">
              <span className="section-count">{offerCodes.codes.length} offer codes</span>
            </div>
            <table className="data-table">
              <thead>
                <tr>
                  <th>Subscription</th>
                  <th>Offer Name</th>
                  <th>Duration</th>
                  <th>Mode</th>
                  <th>Eligibility</th>
                  <th>Total Codes</th>
                  <th>Remaining</th>
                </tr>
              </thead>
              <tbody>
                {offerCodes.codes.map((c, i) => (
                  <tr key={i}>
                    <td style={{ fontWeight: 500 }}>{c.subscriptionName}</td>
                    <td>{c.name}</td>
                    <td>{fmt(c.duration)}</td>
                    <td>{fmt(c.offerMode)}</td>
                    <td>{fmt(c.offerEligibility)}</td>
                    <td>{c.totalNumberOfCodes}</td>
                    <td>{c.productionCodeCount}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </>
        )}
      </div>
    </div>
  );
}
