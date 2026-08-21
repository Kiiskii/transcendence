import { deriveInitials } from "../lib/initials";

// The backend guarantees a signed-in user has a non-empty username, so there
// is no "?" branch here - that decision belongs to the call sites.
describe("deriveInitials", () => {
  test("takes the first letter of the username, uppercased", () => {
    expect(deriveInitials("or99")).toBe("O");
    expect(deriveInitials("Or99")).toBe("O");
  });

  test("ignores leading whitespace", () => {
    expect(deriveInitials("  or99")).toBe("O");
  });

  test("is multibyte safe", () => {
    expect(deriveInitials("émile")).toBe("É");
  });
});
