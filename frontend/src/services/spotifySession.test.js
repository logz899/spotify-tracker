import { captureFromLocation, clear, get } from "./spotifySession";

beforeEach(() => {
  window.sessionStorage.clear();
  window.history.replaceState({}, "", "/");
});

test("captures a linked-login ticket and removes it from the URL immediately", () => {
  window.location.hash = "#/?spotify_login=true&ticket=login-ticket";
  const replaceState = jest.spyOn(window.history, "replaceState");

  expect(captureFromLocation()).toBe("login-ticket");
  expect(get()).toBe("login-ticket");
  expect(window.location.hash).toBe("#/");
  expect(replaceState.mock.calls[0][2]).not.toContain("login-ticket");
  replaceState.mockRestore();
});

test("ignores unrelated hash parameters", () => {
  window.location.hash = "#/?source=landing";
  const replaceState = jest.spyOn(window.history, "replaceState");

  expect(captureFromLocation()).toBeNull();
  expect(get()).toBeNull();
  expect(window.location.hash).toBe("#/?source=landing");
  expect(replaceState).not.toHaveBeenCalled();
  replaceState.mockRestore();
});

test("rejects an empty ticket, cleans callback material, and clears consumed state", () => {
  window.location.hash = "#/?spotify_login=true&ticket=%20%20";

  expect(captureFromLocation()).toBeNull();
  expect(window.location.hash).toBe("#/");

  window.sessionStorage.setItem("spotify_login_ticket", "stored-ticket");
  clear();
  expect(get()).toBeNull();
});
