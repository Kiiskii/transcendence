import { api } from "../api/client";
import { getCurrentUser, login, logout, refresh, signup } from "../api/auth";

vi.mock("../api/client", () => ({
  api: { post: vi.fn(), get: vi.fn() },
}));

const post = vi.mocked(api.post);
const get = vi.mocked(api.get);

// The functions return res.data, so the stubs need that envelope.
function ok(data: unknown) {
  return Promise.resolve({ data });
}

const session = {
  access_token: "tok",
  user: { id: "u1", username: "forager", email: "f@example.com", role: "USER" },
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe("auth api", () => {
  test("signup posts credentials and unwraps the session", async () => {
    post.mockReturnValue(ok(session));

    await expect(
      signup({ username: "forager", email: "f@example.com", password: "secret12" }),
    ).resolves.toBe(session);
    expect(post).toHaveBeenCalledWith(
      "/auth/signup",
      { username: "forager", email: "f@example.com", password: "secret12" },
      { withCredentials: true },
    );
  });

  test("login posts to /auth/login", async () => {
    post.mockReturnValue(ok(session));

    await expect(login({ email: "f@example.com", password: "secret12" })).resolves.toBe(session);
    expect(post).toHaveBeenCalledWith(
      "/auth/login",
      { email: "f@example.com", password: "secret12" },
      { withCredentials: true },
    );
  });

  test("logout posts without a body and returns nothing", async () => {
    post.mockReturnValue(ok(undefined));

    await expect(logout()).resolves.toBeUndefined();
    expect(post).toHaveBeenCalledWith("/auth/logout", undefined, { withCredentials: true });
  });

  test("refresh posts empty-bodied and expects a session back", async () => {
    post.mockReturnValue(ok(session));

    await expect(refresh()).resolves.toBe(session);
    expect(post).toHaveBeenCalledWith("/auth/refresh", undefined, { withCredentials: true });
  });

  test("getCurrentUser reads /auth/me", async () => {
    get.mockReturnValue(ok(session.user));

    await expect(getCurrentUser()).resolves.toBe(session.user);
    expect(get).toHaveBeenCalledWith("/auth/me");
  });

  test("errors propagate untouched for callers to normalise", async () => {
    post.mockReturnValue(Promise.reject({ status: 401, message: "nope" }));

    await expect(login({ email: "f@example.com", password: "bad" })).rejects.toEqual({
      status: 401,
      message: "nope",
    });
  });
});
