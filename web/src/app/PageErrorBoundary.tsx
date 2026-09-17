import React, { Component, type ReactNode } from "react";
import { Link } from "react-router-dom";
import { Button, Card } from "../components/ui";

/** Keep navigation available when a route or its downloaded bundle fails. */
export class PageErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state: { error: Error | null } = { error: null };
  static getDerivedStateFromError(error: Error) { return { error }; }
  render() {
    if (!this.state.error) return this.props.children;
    return (
      <Card>
        <div role="alert">
          <h1>This page couldn’t load</h1>
          <p>Reload to try again. If the problem continues, include the details below when reporting it.</p>
          <Button onClick={() => window.location.reload()}>Reload page</Button>{" "}
          <Link to="/library">Back to Library</Link>
          <details><summary>Error details</summary><pre style={{ whiteSpace: "pre-wrap", overflowWrap: "anywhere" }}>{this.state.error.message}</pre></details>
        </div>
      </Card>
    );
  }
}
