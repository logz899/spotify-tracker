import { mockApi } from "axios";
import {
  exchangeSpotifySession,
  getCurrentUser,
  handleUnauthorizedResponse,
  login,
  logout,
  resolveApiBaseUrl,
} from "./api";
import { consumeSessionExpiredNotice, getToken, setSession } from "./authSession";

jest.mock("axios", () => {
  const instance = {
    get: jest.fn(),
    post: jest.fn(),
    interceptors: {
      request: { use: jest.fn() },
      response: { use: jest.fn() },
    },
  };
  return {
    __esModule: true,
    default: { create: jest.fn(() => instance) },
    mockApi: instance,
  };
});

beforeEach(() => {
  window.sessionStorage.clear();
  window.history.replaceState({}, "", "/repository/#/home");
  mockApi.get.mockReset();
  mockApi.post.mockReset();
});

test("current user profile uses the protected profile endpoint", async () => {
  const profile = { user: { id: 17, username: "listener" }, spotify: { linked: false }, sync: { status: null } };
  mockApi.get.mockResolvedValue({ data: profile });

  await expect(getCurrentUser()).resolves.toEqual(profile);

  expect(mockApi.get).toHaveBeenCalledWith("/api/v1/user/me");
});

test("Email login sends the literal email and password payload", async () => {
  mockApi.post.mockResolvedValue({ data: { token: "jwt" } });

  await login("listener@example.test", "long-password");

  expect(mockApi.post).toHaveBeenCalledWith("/api/v1/auth/login", {
    email: "listener@example.test",
    password: "long-password",
  });
});

test("Spotify session exchange sends only the one-time ticket", async () => {
  mockApi.post.mockResolvedValue({ data: { token: "jwt" } });

  await exchangeSpotifySession("login-ticket");

  expect(mockApi.post).toHaveBeenCalledWith("/api/v1/auth/spotify/session", { ticket: "login-ticket" });
});

test("authenticated 401 response records expiry, clears the session, and returns to login", async () => {
  setSession("jwt-token", { username: "listener" });
  const error = { response: { status: 401 } };

  await expect(handleUnauthorizedResponse(error)).rejects.toBe(error);

  expect(getToken()).toBeNull();
  expect(consumeSessionExpiredNotice()).toBe(true);
  expect(window.location.pathname).toBe("/repository/");
  expect(window.location.hash).toBe("#/");
});

test("unauthenticated 401 response does not create a session-expired notice", async () => {
  const error = { response: { status: 401 } };

  await expect(handleUnauthorizedResponse(error)).rejects.toBe(error);

  expect(consumeSessionExpiredNotice()).toBe(false);
});

test("logout clears the tab session and keeps navigation inside the hash router", () => {
  setSession("jwt-token", { username: "listener" });

  logout();

  expect(getToken()).toBeNull();
  expect(window.location.pathname).toBe("/repository/");
  expect(window.location.hash).toBe("#/");
});

test("API base URL prefers the configured value", () => {
  expect(
    resolveApiBaseUrl({ NODE_ENV: "production", REACT_APP_API_URL: "https://api.example.test" })
  ).toBe("https://api.example.test");
});

test("API base URL falls back to localhost only outside production", () => {
  expect(resolveApiBaseUrl({ NODE_ENV: "development" })).toBe("http://localhost:8000");
  expect(resolveApiBaseUrl({ NODE_ENV: "production" })).toBe("");
});
