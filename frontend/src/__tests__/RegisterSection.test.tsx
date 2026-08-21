import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuthContext, type AuthContextValue } from "../providers/AuthContext";
import { RegisterSection } from "../components/forms/RegisterSection";

function renderRegister(signup: AuthContextValue["signup"]) {
  const value: AuthContextValue = {
    user: null,
    isLoading: false,
    login: vi.fn(),
    signup,
    logout: vi.fn(),
  };
  const onClose = vi.fn();
  render(
    <AuthContext.Provider value={value}>
      <RegisterSection onClose={onClose} />
    </AuthContext.Provider>,
  );
  return { onClose };
}

async function fillValidDetails() {
  const user = userEvent.setup();
  await user.type(screen.getByLabelText("Username"), "forager42");
  await user.type(screen.getByLabelText("Email"), "forager@example.com");
  await user.type(screen.getByLabelText("Password"), "mushrooms123");
  return user;
}

test("signs up with the typed details and closes the modal", async () => {
  const signup = vi.fn<AuthContextValue["signup"]>().mockResolvedValue(undefined);
  const { onClose } = renderRegister(signup);

  const user = await fillValidDetails();
  await user.click(screen.getByRole("button", { name: "Register" }));

  await waitFor(() =>
    expect(signup).toHaveBeenCalledWith("forager42", "forager@example.com", "mushrooms123"),
  );
  // Signup signs the user in too, so closing is all that's left.
  await waitFor(() => expect(onClose).toHaveBeenCalled());
});

test("a name clash lands under the username field", async () => {
  const signup = vi.fn<AuthContextValue["signup"]>().mockRejectedValue({
    status: 409,
    message: "username already taken",
    details: { username: "That username is taken" },
  });
  const { onClose } = renderRegister(signup);

  const user = await fillValidDetails();
  await user.click(screen.getByRole("button", { name: "Register" }));

  expect(await screen.findByText("That username is taken")).toBeInTheDocument();
  expect(onClose).not.toHaveBeenCalled();
});

test("an email clash lands under the email field", async () => {
  const signup = vi.fn<AuthContextValue["signup"]>().mockRejectedValue({
    status: 409,
    message: "email already registered",
    details: { email: "That email is already registered" },
  });
  const { onClose } = renderRegister(signup);

  const user = await fillValidDetails();
  await user.click(screen.getByRole("button", { name: "Register" }));

  expect(await screen.findByText("That email is already registered")).toBeInTheDocument();
  expect(onClose).not.toHaveBeenCalled();
});

test("without field details the backend message shows at the form level", async () => {
  const signup = vi.fn<AuthContextValue["signup"]>().mockRejectedValue({
    status: 429,
    message: "Too many requests. Slow down.",
  });
  const { onClose } = renderRegister(signup);

  const user = await fillValidDetails();
  await user.click(screen.getByRole("button", { name: "Register" }));

  expect(await screen.findByText("Too many requests. Slow down.")).toBeInTheDocument();
  expect(onClose).not.toHaveBeenCalled();
});
