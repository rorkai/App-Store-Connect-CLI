import { FeedbackState } from "../../types";
import { fmt } from "../../utils";

type FeedbackViewProps = {
  feedbackData: FeedbackState;
};

export function FeedbackView({ feedbackData }: FeedbackViewProps) {
  return (
    <div className="app-detail-view">
      <div className="app-detail-section">
        <div className="section-header-row">
          <h3 className="section-label">TestFlight Feedback</h3>
          <span className="section-count">{feedbackData.total} submissions</span>
        </div>
        {feedbackData.loading ? (
          <p className="empty-hint">Loading feedback…</p>
        ) : feedbackData.error ? (
          <p className="empty-hint">{feedbackData.error}</p>
        ) : feedbackData.items.length === 0 ? (
          <p className="empty-hint">No feedback submissions.</p>
        ) : (
          <div className="feedback-grid">
            {feedbackData.items.map((fb) => {
              const daysAgo = Math.floor((Date.now() - new Date(fb.createdDate).getTime()) / 86400000);
              const device = fb.deviceModel || "Unknown";
              const family = fb.deviceFamily === "IPAD" ? "iPad" : fb.deviceFamily === "IPHONE" ? "iPhone" : fb.deviceFamily === "MAC" ? "Mac" : fb.deviceFamily || "";
              return (
                <div key={fb.id} className="feedback-card">
                  <div className="feedback-card-header">
                    <span className="feedback-author">{fb.email || "Anonymous"}</span>
                    <span className="feedback-date">{daysAgo}d ago</span>
                  </div>
                  <div className="feedback-device">
                    {family} &middot; {device} &middot; {fmt(fb.appPlatform)} {fb.osVersion}
                  </div>
                  {fb.screenshots && fb.screenshots.length > 0 && (
                    <div className="feedback-screenshots">
                      {fb.screenshots.map((s, si) => (
                        <img
                          key={si}
                          src={s.url}
                          alt={`Feedback screenshot ${si + 1}`}
                          className={`feedback-screenshot ${s.width > s.height ? "landscape" : ""}`}
                        />
                      ))}
                    </div>
                  )}
                  {fb.comment && (
                    <p className="feedback-comment">{fb.comment}</p>
                  )}
                  {(fb.locale || fb.connectionType) && (
                    <div className="feedback-meta">
                      {fb.locale && <span>{fb.locale}</span>}
                      {fb.timeZone && <span>{fb.timeZone}</span>}
                      {fb.connectionType && <span>{fb.connectionType}</span>}
                      {fb.batteryPercentage > 0 && <span>{fb.batteryPercentage}%</span>}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
