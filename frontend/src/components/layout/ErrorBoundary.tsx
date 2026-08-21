import { Component, type ReactNode } from "react";

interface Props {
  children: ReactNode; // whatever the boundary wraps
}
interface State {
  hasError: boolean;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false };

  // a child threw during render -> flip to the fallback UI
  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  // recvieves the actual error -> place for side effects like logging
  componentDidCatch(error: unknown) {
    console.error("Uncaught error:", error);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="mx-auto max-w-2xl p-9 text-center">
          <h1 className="text-foreground text-2xl font-bold">Something went wrong</h1>
          <p className="text-muted mt-2"> Please refresh the page.</p>
        </div>
      );
    }
    return this.props.children; // no error -> render normally
  }
}
