import {
  AxiosError,
  type AxiosAdapter,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from "axios";
import { api, isApiError, setOnSessionChange } from "../api/client";
import { ACCESS_TOKEN_KEY } from "../providers/AuthContext";

// Drives the real axios instance through its real interceptors; only the
// network layer is swapped out, so what's under test is the wiring itself.
// Handlers that only reject resolve to void; anything else is an AxiosResponse.
type Handler = (config: InternalAxiosRequestConfig) => Promise<unknown>;

interface RecordedRequest {
  method?: string;
  url?: string;
  auth?: string;
}

let requests: RecordedRequest[] = [];

function install(handler: Handler) {
  api.defaults.adapter = (async (config) => handler(config)) as AxiosAdapter;
}

function respond(config: InternalAxiosRequestConfig, status: number, data: unknown): AxiosResponse {
  return { data, status, statusText: "", headers: {}, config, request: {} };
}

function deny(
  config: InternalAxiosRequestConfig,
  data: unknown = { error: "unauthorized" },
): never {
  throw new AxiosError(
    "Unauthorized",
    AxiosError.ERR_BAD_REQUEST,
    config,
    undefined,
    respond(config, 401, data),
  );
}

function record(config: InternalAxiosRequestConfig) {
  requests.push({
    method: config.method?.toLowerCase(),
    url: config.url,
    auth: String(config.headers?.get?.("Authorization") ?? ""),
  });
}

function sessionFor(token: string) {
  return {
    access_token: token,
    user: { id: "u1", username: "forager", email: "f@example.com", role: "USER" },
  };
}

let sessions: unknown[] = [];

beforeEach(() => {
  localStorage.clear();
  requests = [];
  sessions = [];
  // Capture what AuthProvider would receive from background refreshes.
  setOnSessionChange((auth) => sessions.push(auth));
});

afterEach(() => {
  setOnSessionChange(null);
});

describe("401 silent refresh interceptor", () => {
  test("refreshes once and replays the failed request under the new token", async () => {
    install(async (config) => {
      record(config);
      if (config.url === "/auth/refresh") return respond(config, 200, sessionFor("new-token"));
      if (!config._retriedAfterRefresh) deny(config);
      return respond(config, 200, { ok: true });
    });

    localStorage.setItem(ACCESS_TOKEN_KEY, "stale");

    const res = await api.get("/things");

    expect(res.data).toEqual({ ok: true });
    expect(requests.map((r) => `${r.method} ${r.url}`)).toEqual([
      "get /things",
      "post /auth/refresh",
      "get /things",
    ]);
    // The replay must not go out with the stale credential.
    expect(requests[2].auth).toBe("Bearer new-token");
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBe("new-token");
    expect(sessions).toStrictEqual([sessionFor("new-token")]);
  });

  test("a dead refresh signs the user out and surfaces the original 401", async () => {
    install(async (config) => {
      record(config);
      if (config.url === "/auth/refresh") return deny(config, { error: "bad cookie" });
      deny(config, { error: "token expired" });
    });

    localStorage.setItem(ACCESS_TOKEN_KEY, "stale");

    const err = await api.get("/things").catch((e: unknown) => e);

    expect(isApiError(err)).toBe(true);
    // The caller hears about ITS request failing, not the refresh behind it.
    expect((err as { message: string }).message).toBe("token expired");
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull();
    expect(sessions).toStrictEqual([null]);
    // No replay after a failed refresh.
    expect(requests.map((r) => r.url)).toEqual(["/things", "/auth/refresh"]);
  });

  test("never fires for login, signup, logout or refresh itself", async () => {
    install(async (config) => {
      record(config);
      deny(config);
    });

    for (const url of ["/auth/login", "/auth/signup", "/auth/logout"]) {
      await expect(api.post(url, {})).rejects.toMatchObject({ status: 401 });
    }

    expect(requests.map((r) => r.url)).toEqual(["/auth/login", "/auth/signup", "/auth/logout"]);
  });

  test("parallel 401s share one refresh", async () => {
    let refreshCount = 0;

    install(async (config) => {
      record(config);
      if (config.url === "/auth/refresh") {
        refreshCount++;
        // Give the second 401 time to arrive while this one is in flight.
        await new Promise((resolve) => setTimeout(resolve, 0));
        return respond(config, 200, sessionFor("shared-token"));
      }
      if (!config._retriedAfterRefresh) deny(config);
      return respond(config, 200, {});
    });

    const [a, b] = await Promise.all([api.get("/a"), api.get("/b")]);

    expect(a.status).toBe(200);
    expect(b.status).toBe(200);
    expect(refreshCount).toBe(1);
    expect(requests.filter((r) => r.url === "/auth/refresh")).toHaveLength(1);
  });

  test("does not loop when the replayed request 401s again", async () => {
    install(async (config) => {
      record(config);
      if (config.url === "/auth/refresh") return respond(config, 200, sessionFor("new-token"));
      deny(config);
    });

    await expect(api.get("/things")).rejects.toMatchObject({ status: 401 });

    expect(requests.map((r) => r.url)).toEqual(["/things", "/auth/refresh", "/things"]);
  });
});
