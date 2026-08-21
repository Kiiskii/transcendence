import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ContactDetailsSection } from "../components/forms/ContactDetailsSection";
import { useOwnProfile, useUpdateOwnProfile } from "../api/profile";
import type { OwnProfile } from "../api/types";

// The hooks own the network; replace them wholesale so tests control exactly
// what the profile cache would hand back and capture what gets PATCHed.
vi.mock("../api/profile", () => ({
  useOwnProfile: vi.fn(),
  useUpdateOwnProfile: vi.fn(),
}));

const mockedProfile = vi.mocked(useOwnProfile);
const mockedUpdate = vi.mocked(useUpdateOwnProfile);

const PROFILE: OwnProfile = {
  id: "u1",
  username: "or99",
  email: "oscarrogers@example.com",
  firstname: "Oscar",
  lastname: "Rogers",
  bio: null,
  phone_number: "+358 123456",
  date_of_birth: null,
  location: "Espoo",
};

function profileQuery(overrides: Partial<ReturnType<typeof useOwnProfile>> = {}) {
  return { data: PROFILE, isLoading: false, error: null, ...overrides } as ReturnType<
    typeof useOwnProfile
  >;
}

beforeEach(() => {
  vi.clearAllMocks();
  mockedProfile.mockReturnValue(profileQuery());
  mockedUpdate.mockReturnValue({
    mutateAsync: vi.fn().mockResolvedValue(PROFILE),
  } as unknown as ReturnType<typeof useUpdateOwnProfile>);
});

test("shows the saved details before editing", () => {
  render(<ContactDetailsSection />);

  expect(screen.getByText("Oscar")).toBeInTheDocument();
  expect(screen.getByText("+358 123456")).toBeInTheDocument();
});

test("prefills the inputs when entering edit mode", async () => {
  const user = userEvent.setup();
  render(<ContactDetailsSection />);

  await user.click(screen.getByRole("button", { name: "Edit Details" }));

  expect(screen.getByLabelText("First name")).toHaveValue("Oscar");
  expect(screen.getByLabelText("Last name")).toHaveValue("Rogers");
  expect(screen.getByLabelText("Phone")).toHaveValue("+358 123456");
  expect(screen.getByLabelText("City")).toHaveValue("Espoo");
});

test("saves merged values and leaves edit mode", async () => {
  const mutateAsync = vi.fn().mockResolvedValue(PROFILE);
  mockedUpdate.mockReturnValue({ mutateAsync } as unknown as ReturnType<
    typeof useUpdateOwnProfile
  >);
  const user = userEvent.setup();
  render(<ContactDetailsSection />);

  await user.click(screen.getByRole("button", { name: "Edit Details" }));
  const firstName = screen.getByLabelText("First name");
  await user.clear(firstName);
  await user.type(firstName, "Ozzy");
  await user.click(screen.getByRole("button", { name: "Save" }));

  await waitFor(() =>
    expect(mutateAsync).toHaveBeenCalledWith({
      firstname: "Ozzy",
      lastname: "Rogers",
      phone_number: "+358 123456",
      location: "Espoo",
    }),
  );
  // Untouched fields must survive the round trip untouched.
  expect(await screen.findByRole("button", { name: "Edit Details" })).toBeInTheDocument();
});

test("a server error shows inline and keeps edit mode open", async () => {
  mockedUpdate.mockReturnValue({
    mutateAsync: vi.fn().mockRejectedValue({ status: 400, message: "Location too long" }),
  } as unknown as ReturnType<typeof useUpdateOwnProfile>);
  const user = userEvent.setup();
  render(<ContactDetailsSection />);

  await user.click(screen.getByRole("button", { name: "Edit Details" }));
  await user.click(screen.getByRole("button", { name: "Save" }));

  expect(await screen.findByText("Location too long")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Save" })).toBeInTheDocument();
});

test("cancel discards edits instead of saving them", async () => {
  const mutateAsync = vi.fn().mockResolvedValue(PROFILE);
  mockedUpdate.mockReturnValue({ mutateAsync } as unknown as ReturnType<
    typeof useUpdateOwnProfile
  >);
  const user = userEvent.setup();
  render(<ContactDetailsSection />);

  await user.click(screen.getByRole("button", { name: "Edit Details" }));
  const firstName = screen.getByLabelText("First name");
  await user.clear(firstName);
  await user.type(firstName, "Wrong Name");
  await user.click(screen.getByRole("button", { name: "Cancel" }));

  expect(mutateAsync).not.toHaveBeenCalled();
  expect(screen.getByText("Oscar")).toBeInTheDocument();

  // Re-entering edit mode restores the saved values, not the typed ones.
  await user.click(screen.getByRole("button", { name: "Edit Details" }));
  expect(screen.getByLabelText("First name")).toHaveValue("Oscar");
});
