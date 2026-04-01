import { GroupTestersState, TestFlightState } from "../../types";
import { fmt } from "../../utils";

type TestFlightViewProps = {
  testflightData: TestFlightState;
  selectedGroup: string | null;
  groupTesters: GroupTestersState;
  onSelectGroup: (groupId: string) => void;
  onBackToGroups: () => void;
};

export function TestFlightView({
  testflightData,
  selectedGroup,
  groupTesters,
  onSelectGroup,
  onBackToGroups,
}: TestFlightViewProps) {
  // Detail view for a group's testers
  if (selectedGroup) {
    const group = testflightData.groups.find((g) => g.id === selectedGroup);
    // Compute state breakdown
    const stateCounts: Record<string, number> = {};
    for (const t of groupTesters.testers) {
      stateCounts[t.state] = (stateCounts[t.state] || 0) + 1;
    }
    return (
      <div className="app-detail-view">
        <div className="app-detail-section">
          <button className="back-link" type="button" onClick={onBackToGroups} aria-label="Back to TestFlight groups">&larr; TestFlight</button>
          <p className="app-detail-name" style={{ marginTop: 8 }}>{group?.name ?? "Group"}</p>
          <p style={{ margin: "4px 0 0", fontSize: 12, color: "var(--text-secondary)" }}>
            {group?.isInternal ? "Internal" : "External"} &middot; {group?.testerCount ?? 0} testers
            {group?.publicLink && <> &middot; <a href={group.publicLink} target="_blank" rel="noopener" style={{ color: "var(--accent)" }}>TestFlight Link</a></>}
          </p>

          {groupTesters.loading ? (
            <p className="empty-hint">Loading testers…</p>
          ) : groupTesters.error ? (
            <p className="empty-hint">{groupTesters.error}</p>
          ) : groupTesters.testers.length === 0 ? (
            <p className="empty-hint">No testers in this group.</p>
          ) : (
            <>
              {/* State summary */}
              <div style={{ display: "flex", gap: 10, margin: "12px 0" }}>
                {Object.entries(stateCounts).map(([state, count]) => (
                  <div key={state} style={{ textAlign: "center" }}>
                    <span style={{ fontSize: 20, fontWeight: 600, color: "var(--text-primary)" }}>{count}</span>
                    <p style={{ margin: 0, fontSize: 10, color: "var(--text-secondary)", textTransform: "uppercase" }}>{fmt(state)}</p>
                  </div>
                ))}
              </div>

              {/* Tester table */}
              <table className="data-table" style={{ marginTop: 8 }}>
                <thead>
                  <tr>
                    <th>Email</th>
                    <th>Name</th>
                    <th>Invite</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {groupTesters.testers.map((t, i) => (
                    <tr key={i}>
                      <td className="mono">{t.email || "\u2014"}</td>
                      <td>{[t.firstName, t.lastName].filter(Boolean).join(" ") || "Anonymous"}</td>
                      <td>{fmt(t.inviteType)}</td>
                      <td><span className={`status-pill status-${t.state.toLowerCase()}`}>{fmt(t.state)}</span></td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <p style={{ marginTop: 8, fontSize: 11, color: "var(--text-secondary)" }}>
                {groupTesters.testers.length} testers loaded
              </p>
            </>
          )}
        </div>
      </div>
    );
  }

  // Groups list
  return (
    <div className="app-detail-view">
      <div className="app-detail-section">
        <h3 className="section-label">TestFlight</h3>
        {testflightData.loading ? (
          <p className="empty-hint">Loading…</p>
        ) : testflightData.error ? (
          <p className="empty-hint">{testflightData.error}</p>
        ) : testflightData.groups.length === 0 ? (
          <p className="empty-hint">No beta groups found.</p>
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th>Group</th>
                <th>Type</th>
                <th>Testers</th>
                <th>Public Link</th>
                <th>Feedback</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {testflightData.groups.map((g) => (
                <tr
                  key={g.id}
                  className="clickable-row"
                  tabIndex={0}
                  role="button"
                  aria-label={`View testers for ${g.name}`}
                  onClick={() => onSelectGroup(g.id)}
                  onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onSelectGroup(g.id); } }}
                >
                  <td style={{ fontWeight: 500 }}>{g.name}</td>
                  <td>{g.isInternal ? "Internal" : "External"}</td>
                  <td>{g.testerCount}</td>
                  <td>{g.publicLink ? <a href={g.publicLink} target="_blank" rel="noopener" style={{ color: "var(--accent)" }} onClick={(e) => e.stopPropagation()}>{g.publicLink.replace("https://testflight.apple.com/join/", "")}</a> : "\u2014"}</td>
                  <td>{g.feedbackEnabled ? "On" : "Off"}</td>
                  <td>{g.createdDate.split("T")[0]}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
