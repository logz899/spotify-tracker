import { handleUnauthorizedResponse, logout, resolveApiBaseUrl } from "./api";
import { getToken, setSession } from "./authSession";

beforeEach(() => {
  window.sessionStorage.clear();
  window.history.replaceState({}, "", "/repository/#/home");
});

test("401 response clears the tab session and returns to the root hash route", async () => {
  setSession("jwt-token", { username: "listener" });
  const error = { response: { status: 401 } };

  await expect(handleUnauthorizedResponse(error)).rejects.toBe(error);

  expect(getToken()).toBeNull();
  expect(window.location.pathname).toBe("/repository/");
  expect(window.location.hash).toBe("#/");
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
