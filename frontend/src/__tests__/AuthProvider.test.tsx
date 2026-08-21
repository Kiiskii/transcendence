import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuthProvider } from "../providers/AuthProvider";
import { useAuth } from "../hooks/useAuth";
import * as authApi from "../api/auth";
import { ACCESS_TOKEN_KEY } from "../providers/AuthContext";

vi.mock("../api/auth");
const mockedAuthApi = vi.mocked(authApi);

const session = {
  access_token: "tok",
  user: { id: "u1", username: "forager", email: "f@example.com", role: "USER" as const },
};

// Stands in for a page's own query (e.g. Profile's useOwnProfile) that was
// already mounted - and fetched, possibly erroring if signed out - before the
// session changed. Counting calls (rather than reading the cache directly)
// sidesteps the race between queryClient.clear() and the automatic refetch
// it triggers for any query an observer like this is still watching.
const probeFetcher = vi.fn().mockResolvedValue("fresh");

function ProbeQuery() {
  const { status } = useQuery({
    queryKey: ["probe"],
    queryFn: probeFetcher,
    retry: false,
  });
  return <span>probe:{status}</span>;
}

function Consumer() {
  const { user, login, signup, logout } = useAuth();
  return (
    <>
      <span>{user ? user.username : "signed-out"}</span>
      <button onClick={() => void login("f@example.com", "secret12")}>Log In</button>
      <button onClick={() => void signup("forager", "f@example.com", "secret12")}>Register</button>
      <button onClick={() => void logout()}>Log Out</button>
      <ProbeQuery />
    </>
  );
}

function renderApp() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

  render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <Consumer />
      </AuthProvider>
    </QueryClientProvider>,
  );
  return { queryClient };
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  // No token yet: restoreSession falls through to a refresh attempt, which
  // has nothing to recover on a clean slate.
  mockedAuthApi.refresh.mockRejectedValue({ status: 401, message: "no session" });
});

test("a successful login drops every cached query so mounted pages refetch", async () => {
  mockedAuthApi.login.mockResolvedValue(session);
  const user = userEvent.setup();
  renderApp();

  await screen.findByText("probe:success");
  expect(probeFetcher).toHaveBeenCalledTimes(1);

  await user.click(screen.getByRole("button", { name: "Log In" }));

  await screen.findByText("forager");
  // The clear forces the still-mounted probe to refetch under the new session.
  await waitFor(() => expect(probeFetcher).toHaveBeenCalledTimes(2));
  expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBe("tok");
});

test("a successful registration drops every cached query so mounted pages refetch", async () => {
  mockedAuthApi.signup.mockResolvedValue(session);
  const user = userEvent.setup();
  renderApp();

  await screen.findByText("probe:success");
  expect(probeFetcher).toHaveBeenCalledTimes(1);

  await user.click(screen.getByRole("button", { name: "Register" }));

  await screen.findByText("forager");
  await waitFor(() => expect(probeFetcher).toHaveBeenCalledTimes(2));
});

test("logging out also drops every cached query", async () => {
  mockedAuthApi.login.mockResolvedValue(session);
  mockedAuthApi.logout.mockResolvedValue(undefined);
  const user = userEvent.setup();
  renderApp();

  await user.click(screen.getByRole("button", { name: "Log In" }));
  await screen.findByText("forager");
  await waitFor(() => expect(probeFetcher).toHaveBeenCalledTimes(2));

  await user.click(screen.getByRole("button", { name: "Log Out" }));

  await screen.findByText("signed-out");
  // A query fetched while signed in gets cleared on the way out too.
  await waitFor(() => expect(probeFetcher).toHaveBeenCalledTimes(3));
  expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull();
});
