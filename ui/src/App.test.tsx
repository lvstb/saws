import { render, screen } from "@testing-library/react";

import App from "./App";

describe("App", () => {
  it("renders the app shell title", () => {
    render(<App />);
    expect(screen.getByText("Saws Desktop")).toBeInTheDocument();
  });

  it("renders the connect placeholder", () => {
    render(<App />);
    expect(screen.getByText("Connect to AWS SSO")).toBeInTheDocument();
  });
});
