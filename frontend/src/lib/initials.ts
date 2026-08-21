// Avatar initial from the username.
//
// Assumes a signed-in user: the backend guarantees a non-empty username, so
// there is deliberately no "?" fallback here - call sites render "?" based on
// signed-out state instead. One rule everywhere keeps the header mini avatar
// and the profile page avatar consistent.

// First character of a string, multibyte safe; whitespace-only counts as unset.
function firstChar(value?: string | null): string {
  const trimmed = value?.trim() ?? "";
  return Array.from(trimmed)[0] ?? "";
}

export function deriveInitials(username: string): string {
  return firstChar(username).toUpperCase();
}
