import { FinanceRegionsState } from "../../types";

type FinanceViewProps = {
  financeRegions: FinanceRegionsState;
};

export function FinanceView({ financeRegions }: FinanceViewProps) {
  return (
    <div className="app-detail-view">
      <div className="app-detail-section">
        <h3 className="section-label">Finance Regions</h3>
        {financeRegions.loading ? (
          <p className="empty-hint">Loading…</p>
        ) : financeRegions.error ? (
          <p className="empty-hint">{financeRegions.error}</p>
        ) : financeRegions.regions.length === 0 ? (
          <p className="empty-hint">No finance regions found.</p>
        ) : (
          <>
            <div className="section-header-row">
              <span className="section-count">{financeRegions.regions.length} regions</span>
            </div>
            <table className="data-table">
              <thead><tr><th>Region</th><th>Code</th><th>Currency</th><th>Countries</th></tr></thead>
              <tbody>
                {financeRegions.regions.map((r) => (
                  <tr key={r.regionCode}>
                    <td style={{ fontWeight: 500 }}>{r.reportRegion}</td>
                    <td className="mono">{r.regionCode}</td>
                    <td>{r.reportCurrency}</td>
                    <td>{r.countriesOrRegions}</td>
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
