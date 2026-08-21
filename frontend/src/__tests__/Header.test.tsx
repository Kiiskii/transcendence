import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import Header from "../components/layout/Header";
import { ModalProvider } from "../providers/ModalProvider";
import { AuthProvider } from "../providers/AuthProvider";
import { AuthContext, type AuthContextValue } from "../providers/AuthContext";

function renderHeader(auth?: Partial<AuthContextValue>) {
  const value: AuthContextValue = {
    user: null,
    isLoading: false,
    login: vi.fn(),
    signup: vi.fn(),
    logout: vi.fn(),
    ...auth,
  };
  render(
    // The real provider fires a session restore on mount; tests pass their own
    // values unless they specifically want that round trip.
    auth ? (
      <AuthContext.Provider value={value}>
        <ModalProvider>
          <MemoryRouter>
            <Header />
          </MemoryRouter>
        </ModalProvider>
      </AuthContext.Provider>
    ) : (
      // AuthProvider clears the query cache on logout, so it needs a
      // QueryClient in scope even though this suite never triggers that path.
      <QueryClientProvider client={new QueryClient()}>
        <AuthProvider>
          <ModalProvider>
            <MemoryRouter>
              <Header />
            </MemoryRouter>
          </ModalProvider>
        </AuthProvider>
      </QueryClientProvider>
    ),
  );
}

test("renders the brand name", () => {
  renderHeader();
  expect(screen.getByText("Metsätori")).toBeInTheDocument();
});

test("offers login when signed out", () => {
  renderHeader();
  expect(screen.getByText("?")).toBeInTheDocument();
});

test("shows who is signed in", () => {
  renderHeader({
    user: { id: "u1", username: "forager", email: "f@example.com", role: "USER" },
  });
  // Single initial from the username - names live on the profile.
  expect(screen.getByText("F")).toBeInTheDocument();
});
