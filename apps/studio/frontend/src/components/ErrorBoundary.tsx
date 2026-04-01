import { Component, ReactNode } from "react";

type Props = { children: ReactNode };
type State = { error: string | null };

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error) {
    return { error: error.message };
  }

  render() {
    if (this.state.error) {
      return (
        <div className="empty-state" role="alert">
          <p className="empty-title">Something went wrong</p>
          <p className="empty-hint">{this.state.error}</p>
          <button
            className="toolbar-btn"
            type="button"
            onClick={() => this.setState({ error: null })}
          >
            Try again
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
