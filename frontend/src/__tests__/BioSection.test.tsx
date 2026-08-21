import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BioSection } from "../components/forms/BioSection";
import { useOwnProfile, useUpdateOwnProfile } from "../api/profile";
import type { OwnProfile } from "../api/types";

vi.mock("../api/profile", () => ({
  useOwnProfile: vi.fn(),
  useUpdateOwnProfile: vi.fn(),
}));

const mockedProfile = vi.mocked(useOwnProfile);
const mockedUpdate = vi.mocked(useUpdateOwnProfile);

const BIO = "Forager of chanterelles.";

function makeProfile(bio: string | null): OwnProfile {
  return {
    id: "u1",
    username: "or99",
    email: "oscarrogers@example.com",
    firstname: null,
    lastname: null,
    bio,
    phone_number: null,
    date_of_birth: null,
    location: null,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockedProfile.mockReturnValue({
    data: makeProfile(BIO),
    isLoading: false,
    error: null,
  } as ReturnType<typeof useOwnProfile>);
  mockedUpdate.mockReturnValue({
    mutateAsync: vi.fn().mockResolvedValue(makeProfile(BIO)),
  } as unknown as ReturnType<typeof useUpdateOwnProfile>);
});

test("shows the saved bio before editing", () => {
  render(<BioSection />);
  expect(screen.getByText(BIO)).toBeInTheDocument();
});

test("prefills the textarea when entering edit mode", async () => {
  const user = userEvent.setup();
  render(<BioSection />);

  await user.click(screen.getByRole("button", { name: "Edit Text" }));

  expect(screen.getByRole("textbox")).toHaveValue(BIO);
});

test("saves the edited bio and clears an emptied one", async () => {
  const mutateAsync = vi.fn().mockResolvedValue(makeProfile(""));
  mockedUpdate.mockReturnValue({ mutateAsync } as unknown as ReturnType<
    typeof useUpdateOwnProfile
  >);
  const user = userEvent.setup();
  render(<BioSection />);

  await user.click(screen.getByRole("button", { name: "Edit Text" }));
  const textarea = screen.getByRole("textbox");
  await user.clear(textarea);
  await user.click(screen.getByRole("button", { name: "Save" }));

  // An empty textarea sends "" - the backend treats that as "clear".
  await waitFor(() => expect(mutateAsync).toHaveBeenCalledWith({ bio: "" }));
  expect(await screen.findByRole("button", { name: "Edit Text" })).toBeInTheDocument();
});

test("a server error shows inline and keeps edit mode open", async () => {
  mockedUpdate.mockReturnValue({
    mutateAsync: vi.fn().mockRejectedValue({ status: 500, message: "Could not save right now" }),
  } as unknown as ReturnType<typeof useUpdateOwnProfile>);
  const user = userEvent.setup();
  render(<BioSection />);

  await user.click(screen.getByRole("button", { name: "Edit Text" }));
  await user.click(screen.getByRole("button", { name: "Save" }));

  expect(await screen.findByText("Could not save right now")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Save" })).toBeInTheDocument();
});
