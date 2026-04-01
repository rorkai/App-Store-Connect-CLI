import { SectionCacheEntry } from "../../types";
import { fmt } from "../../utils";

type InsightsViewProps = {
  insightsWeek: string;
  insightsCache: SectionCacheEntry | undefined;
};

export function InsightsView({ insightsWeek, insightsCache }: InsightsViewProps) {
  return (
    <div className="app-detail-view">
      <div className="app-detail-section">
        <h3 className="section-label">Weekly Insights</h3>
        <p style={{ fontSize: 12, color: "var(--text-secondary)", margin: "0 0 12px" }}>
          Week of {insightsWeek} — analytics source
        </p>
        {!insightsCache || insightsCache.loading ? (
          <p className="empty-hint">Loading…</p>
        ) : insightsCache.error ? (
          <p className="empty-hint">{insightsCache.error}</p>
        ) : insightsCache.items.length === 0 ? (
          <p className="empty-hint">No insights data.</p>
        ) : (
            <table className="data-table">
              <thead><tr><th>Metric</th><th>Status</th><th>Value</th></tr></thead>
              <tbody>
                {insightsCache.items.map((m, i) => (
                  <tr key={i}>
                    <td>{String(m.name ?? "").replace(/_/g, " ")}</td>
                    <td><span className={`status-pill status-${String(m.status ?? "").toLowerCase()}`}>{fmt(String(m.status ?? ""))}</span></td>
                    <td>{m.thisWeek != null ? String(m.thisWeek) : (m.reason ? String(m.reason) : "\u2014")}</td>
                  </tr>
                ))}
              </tbody>
            </table>
        )}
      </div>
    </div>
  );
}
