import { SubscriptionsState } from "../../types";

type SubscriptionsViewProps = {
  subscriptions: SubscriptionsState;
  selectedSub: string | null;
  onSelectSub: (id: string | null) => void;
};

export function SubscriptionsView({ subscriptions, selectedSub, onSelectSub }: SubscriptionsViewProps) {
  const sub = selectedSub ? subscriptions.items.find((s) => s.id === selectedSub) : null;
  if (sub) {
    // Detail view for a single subscription
    return (
      <div className="app-detail-view">
        <div className="app-detail-section">
          <button className="back-link" type="button" onClick={() => onSelectSub(null)}>&larr; Subscriptions</button>
          {subscriptions.error && (
            <p className="empty-hint" style={{ marginTop: 12 }}>{subscriptions.error}</p>
          )}
          <p className="app-detail-name" style={{ marginTop: 8 }}>{sub.name}</p>
          <div className="env-grid" style={{ marginTop: 12 }}>
            <div className="env-row">
              <span className="env-key">Status</span>
              <span className="env-value"><span className={`status-pill status-${sub.state.toLowerCase()}`}>{sub.state}</span></span>
            </div>
            <div className="env-row">
              <span className="env-key">Product ID</span>
              <span className="env-value mono">{sub.productId}</span>
            </div>
            <div className="env-row">
              <span className="env-key">Subscription Duration</span>
              <span className="env-value">{sub.subscriptionPeriod.replace(/_/g, " ").toLowerCase()}</span>
            </div>
            <div className="env-row">
              <span className="env-key">Group</span>
              <span className="env-value">{sub.groupName}</span>
            </div>
            <div className="env-row">
              <span className="env-key">Group Level</span>
              <span className="env-value">{sub.groupLevel}</span>
            </div>
            {sub.reviewNote && (
              <div className="env-row">
                <span className="env-key">Review Notes</span>
                <span className="env-value">{sub.reviewNote}</span>
              </div>
            )}
            <div className="env-row">
              <span className="env-key">Apple ID</span>
              <span className="env-value mono">{sub.id}</span>
            </div>
          </div>
        </div>
      </div>
    );
  }

  // List view
  return (
    <div className="app-detail-view">
      <div className="app-detail-section">
        <h3 className="section-label">Subscriptions</h3>
        {subscriptions.loading ? (
          <p className="empty-hint">Loading…</p>
        ) : subscriptions.items.length === 0 && subscriptions.error ? (
          <p className="empty-hint">{subscriptions.error}</p>
        ) : subscriptions.items.length === 0 ? (
          <p className="empty-hint">No subscriptions found.</p>
        ) : (() => {
          const groups = [...new Set(subscriptions.items.map((s) => s.groupName))];
          return groups.map((group) => (
            <div key={group} className="sub-group">
              {group === groups[0] && subscriptions.error && (
                <p className="empty-hint" style={{ marginBottom: 12 }}>{subscriptions.error}</p>
              )}
              <p className="sub-group-name">{group}</p>
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Product ID</th>
                    <th>Period</th>
                    <th>Level</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {subscriptions.items
                    .filter((s) => s.groupName === group)
                    .sort((a, b) => a.groupLevel - b.groupLevel)
                    .map((s) => (
                      <tr key={s.productId} className="clickable-row" onClick={() => onSelectSub(s.id)}>
                        <td>{s.name}</td>
                        <td className="mono">{s.productId}</td>
                        <td>{s.subscriptionPeriod.replace(/_/g, " ").toLowerCase()}</td>
                        <td>{s.groupLevel}</td>
                        <td><span className={`status-pill status-${s.state.toLowerCase()}`}>{s.state}</span></td>
                      </tr>
                    ))}
                </tbody>
              </table>
            </div>
          ));
        })()}
      </div>
    </div>
  );
}
