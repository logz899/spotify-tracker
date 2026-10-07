import { captureFromLocation, clear, get } from "./pendingSpotify";

beforeEach(() => {
  window.sessionStorage.clear();
  window.history.replaceState({}, "", "/");
});

test("captures a pending key from the hash query and immediately cleans the URL", () => {
  window.location.hash = "#/?spotify_connected=true&key=pending-key";
  const replaceState = jest.spyOn(window.history, "replaceState");

  expect(captureFromLocation()).toBe("pending-key");

  expect(get()).toBe("pending-key");
  expect(window.location.hash).toBe("#/");
  expect(replaceState).toHaveBeenCalledTimes(1);
  expect(replaceState.mock.calls[0][2]).not.toContain("pending-key");
  replaceState.mockRestore();
});

test("pending key persists only in session storage until explicitly cleared", () => {
  window.location.hash = "#/?spotify_connected=true&key=one-tab-key";
  captureFromLocation();

  expect(get()).toBe("one-tab-key");
  expect(window.localStorage.getItem("spotify_pending_key")).toBeNull();

  clear();
  expect(get()).toBeNull();
});

test("rejects an empty pending key and still removes OAuth query material", () => {
  window.location.hash = "#/?spotify_connected=true&key=%20%20";

  expect(captureFromLocation()).toBeNull();

  expect(get()).toBeNull();
  expect(window.location.hash).toBe("#/");
});
