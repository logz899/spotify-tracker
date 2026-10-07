const PENDING_KEY = "spotify_pending_key";

export const get = () => window.sessionStorage.getItem(PENDING_KEY);

export const clear = () => window.sessionStorage.removeItem(PENDING_KEY);

export const captureFromLocation = (location = window.location, history = window.history) => {
  const hash = location.hash.startsWith("#") ? location.hash.slice(1) : location.hash;
  const queryIndex = hash.indexOf("?");
  if (queryIndex < 0) {
    return null;
  }

  const route = hash.slice(0, queryIndex) || "/";
  const params = new URLSearchParams(hash.slice(queryIndex + 1));
  const isSpotifyCallback = params.has("spotify_connected") || params.has("key");
  if (!isSpotifyCallback) {
    return null;
  }

  const pendingKey = params.get("spotify_connected") === "true" ? params.get("key")?.trim() : "";
  if (pendingKey) {
    window.sessionStorage.setItem(PENDING_KEY, pendingKey);
  }

  history.replaceState(history.state, "", `${location.pathname}${location.search}#${route}`);
  return pendingKey || null;
};
