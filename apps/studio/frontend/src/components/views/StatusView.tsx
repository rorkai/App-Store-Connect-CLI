import { AppStatusState } from "../../types";
import { fmt } from "../../utils";

type StatusViewProps = {
  appStatus: AppStatusState;
};

export function StatusView({ appStatus }: StatusViewProps) {
  return (
    <div className="app-detail-view">
      <div className="app-detail-section">
        <h3 className="section-label">Release Status</h3>
        {appStatus.loading ? (
          <p className="empty-hint">Loading…</p>
        ) : appStatus.error ? (
          <p className="empty-hint">{appStatus.error}</p>
        ) : appStatus.data ? (() => {
          const s = appStatus.data;
          return (
            <>
              {/* Health summary */}
              <div className="status-health" style={{ marginBottom: 16 }}>
                <span className={`status-pill status-health-${s.summary?.health}`} style={{ fontSize: 13, padding: "4px 10px" }}>
                  {s.summary?.health === "green" ? "Healthy" : s.summary?.health === "yellow" ? "Attention" : s.summary?.health === "red" ? "Blocked" : s.summary?.health}
                </span>
                {s.summary?.nextAction && <p style={{ margin: "8px 0 0", fontSize: 13, color: "var(--text-secondary)" }}>{s.summary.nextAction}</p>}
              </div>

              {/* Blockers */}
              {(s.summary?.blockers?.length ?? 0) > 0 && (
                <div className="status-blockers" style={{ marginBottom: 16 }}>
                  {s.summary!.blockers!.map((b: string, i: number) => (
                    <div key={i} className="blocker-row">
                      <span className="blocker-icon" role="img" aria-label="Blocker">!</span>
                      <span>{b}</span>
                    </div>
                  ))}
                </div>
              )}

              <table className="data-table" style={{ marginBottom: 20 }}>
                <tbody>
                  <tr><td className="vcard-label">App Store Version</td><td>{s.appstore?.version} — <span className={`status-pill status-${(s.appstore?.state ?? "").toLowerCase().replace(/_/g, "-")}`}>{fmt(s.appstore?.state ?? "")}</span></td></tr>
                  <tr><td className="vcard-label">Platform</td><td>{fmt(s.appstore?.platform ?? "")}</td></tr>
                  <tr><td className="vcard-label">Latest Build</td><td>{s.builds?.latest?.version} (#{s.builds?.latest?.buildNumber}) — <span className={`status-pill status-${(s.builds?.latest?.processingState ?? "").toLowerCase()}`}>{fmt(s.builds?.latest?.processingState ?? "")}</span></td></tr>
                  <tr><td className="vcard-label">Uploaded</td><td>{fmt(s.builds?.latest?.uploadedDate ?? "")}</td></tr>
                  <tr><td className="vcard-label">Review</td><td><span className={`status-pill status-${(s.review?.state ?? "").toLowerCase()}`}>{fmt(s.review?.state ?? "")}</span> {s.review?.submittedDate ? `(submitted ${s.review.submittedDate.split("T")[0]})` : ""}</td></tr>
                  <tr><td className="vcard-label">TestFlight</td><td><span className={`status-pill status-${(s.testflight?.betaReviewState ?? "").toLowerCase()}`}>{fmt(s.testflight?.betaReviewState ?? "")}</span></td></tr>
                  <tr><td className="vcard-label">Phased Release</td><td>{s.phasedRelease?.configured ? "Configured" : "Not configured"}</td></tr>
                  <tr><td className="vcard-label">Submission</td><td>{s.submission?.inFlight ? "In flight" : "None"}{s.submission?.blockingIssues?.length ? ` — ${s.submission.blockingIssues.length} blocking` : ""}</td></tr>
                </tbody>
              </table>

              {/* Links */}
              {s.links && (
                <div style={{ marginTop: 8 }}>
                  <p className="metadata-label">Links</p>
                  <div style={{ display: "flex", gap: 12 }}>
                    {s.links.appStoreConnect && <a href={s.links.appStoreConnect} target="_blank" rel="noopener" style={{ color: "var(--accent)", fontSize: 12 }}>App Store Connect</a>}
                    {s.links.testFlight && <a href={s.links.testFlight} target="_blank" rel="noopener" style={{ color: "var(--accent)", fontSize: 12 }}>TestFlight</a>}
                    {s.links.review && <a href={s.links.review} target="_blank" rel="noopener" style={{ color: "var(--accent)", fontSize: 12 }}>Review</a>}
                  </div>
                </div>
              )}
            </>
          );
        })() : null}
      </div>
    </div>
  );
}
