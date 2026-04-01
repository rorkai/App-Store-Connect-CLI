type ToolViewProps = {
  title: string;
  description: string;
  commandHint: string;
};

export function ToolView({ title, description, commandHint }: ToolViewProps) {
  return (
    <div className="app-detail-view">
      <div className="app-detail-section">
        <h3 className="section-label">{title}</h3>
        <p style={{ fontSize: 13, color: "var(--text-secondary)" }}>
          {description}
        </p>
        <p style={{ fontSize: 12, color: "var(--text-tertiary)", marginTop: 8 }}>
          {commandHint}
        </p>
      </div>
    </div>
  );
}
