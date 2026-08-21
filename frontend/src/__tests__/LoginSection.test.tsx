import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuthContext, type AuthContextValue } from "../providers/AuthContext";
import { LoginSection } from "../components/forms/LoginSection";

// The real provider would hit the network on mount; the sections only need the
// context values, so hand them a wired-up stub directly.
function renderLogin(login: AuthContextValue["login"]) {
  const value: AuthContextValue = {
    user: null,
    isLoading: false,
    login,
    signup: vi.fn(),
    logout: vi.fn(),
  };
  const onClose = vi.fn();
  render(
    <AuthContext.Provider value={value}>
      <LoginSection onClose={onClose} />
    </AuthContext.Provider>,
  );
  return { onClose };
}

async function fillValidCredentials() {
  const user = userEvent.setup();
  await user.type(screen.getByLabelText("Email"), "forager@example.com");
  await user.type(screen.getByLabelText("Password"), "mushrooms123");
  return user;
}

test("starts out disabled until the fields validate", async () => {
  renderLogin(vi.fn());

  expect(screen.getByRole("button", { name: "Log In" })).toBeDisabled();

  await fillValidCredentials();
  expect(screen.getByRole("button", { name: "Log In" })).toBeEnabled();
});

test("logs in with the typed credentials and closes the modal", async () => {
  const login = vi.fn<AuthContextValue["login"]>().mockResolvedValue(undefined);
  const { onClose } = renderLogin(login);

  const user = await fillValidCredentials();
  await user.click(screen.getByRole("button", { name: "Log In" }));

  await waitFor(() => expect(login).toHaveBeenCalledWith("forager@example.com", "mushrooms123"));
  await waitFor(() => expect(onClose).toHaveBeenCalled());
});

test("shows the backend's message on bad credentials and stays open", async () => {
  // Matches the ApiError shape the interceptor normalises to.
  const login = vi.fn<AuthContextValue["login"]>().mockRejectedValue({
    status: 401,
    message: "Invalid email or password",
  });
  const { onClose } = renderLogin(login);

  const user = await fillValidCredentials();
  await user.click(screen.getByRole("button", { name: "Log In" }));

  expect(await screen.findByText("Invalid email or password")).toBeInTheDocument();
  expect(onClose).not.toHaveBeenCalled();
});

test("an unexpected failure falls back to a generic message", async () => {
  const login = vi.fn<AuthContextValue["login"]>().mockRejectedValue(new Error("boom"));
  const { onClose } = renderLogin(login);

  const user = await fillValidCredentials();
  await user.click(screen.getByRole("button", { name: "Log In" }));

  expect(await screen.findByText("Something went wrong. Please try again.")).toBeInTheDocument();
  expect(onClose).not.toHaveBeenCalled();
});
