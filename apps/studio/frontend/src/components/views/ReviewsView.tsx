import { ReviewsState } from "../../types";
import { fmt } from "../../utils";

type ReviewsViewProps = {
  reviews: ReviewsState;
};

export function ReviewsView({ reviews }: ReviewsViewProps) {
  return (
    <div className="app-detail-view">
      <div className="app-detail-section">
        <div className="section-header-row">
          <h3 className="section-label">Ratings and Reviews</h3>
          {reviews.items.length > 0 && <span className="section-count">{reviews.items.length} reviews</span>}
        </div>
        {reviews.loading ? (
          <p className="empty-hint">Loading…</p>
        ) : reviews.error ? (
          <p className="empty-hint">{reviews.error}</p>
        ) : reviews.items.length === 0 ? (
          <p className="empty-hint">No reviews found.</p>
        ) : (
          <div className="reviews-list">
            {reviews.items.map((r, i) => (
              <div key={i} className="review-card">
                <div className="review-header">
                  <span className="review-stars" role="img" aria-label={`${r.rating} out of 5 stars`}>{"\u2605".repeat(r.rating)}{"\u2606".repeat(5 - r.rating)}</span>
                  <span className="review-meta">{r.reviewerNickname} &middot; {r.territory} &middot; {fmt(r.createdDate)}</span>
                </div>
                {r.title && <p className="review-title">{r.title}</p>}
                {r.body && <p className="review-body">{r.body}</p>}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
