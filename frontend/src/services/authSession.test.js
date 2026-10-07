import { clearSession, getToken, getUser, setSession } from "./authSession";

beforeEach(() => {
  window.sessionStorage.clear();
  window.localStorage.clear();
});

test("stores and reads authentication only from the current tab session", () => {
  const user = { id: 7, username: "listener" };

  setSession("jwt-token", user);

  expect(getToken()).toBe("jwt-token");
  expect(getUser()).toEqual(user);
  expect(window.localStorage.getItem("token")).toBeNull();
  expect(window.localStorage.getItem("user")).toBeNull();
});

test("returns null for absent or malformed session user data", () => {
  expect(getUser()).toBeNull();

  window.sessionStorage.setItem("user", "{invalid-json");
  expect(getUser()).toBeNull();
  expect(window.sessionStorage.getItem("user")).toBeNull();
});

test("clears both token and user from the tab session", () => {
  setSession("jwt-token", { username: "listener" });

  clearSession();

  expect(getToken()).toBeNull();
  expect(getUser()).toBeNull();
});
