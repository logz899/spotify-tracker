const SPOTIFY_LOGIN_TICKET_KEY = "spotify_login_ticket";

export const get = () => window.sessionStorage.getItem(SPOTIFY_LOGIN_TICKET_KEY);

export const clear = () => window.sessionStorage.removeItem(SPOTIFY_LOGIN_TICKET_KEY);

export const captureFromLocation = (location = window.location, history = window.history) => {
  const hash = location.hash.startsWith("#") ? location.hash.slice(1) : location.hash;
  const queryIndex = hash.indexOf("?");
  if (queryIndex < 0) {
    return null;
  }

  const route = hash.slice(0, queryIndex) || "/";
  const params = new URLSearchParams(hash.slice(queryIndex + 1));
  const isSpotifyLoginCallback = params.has("spotify_login") || params.has("ticket");
  if (!isSpotifyLoginCallback) {
    return null;
  }

  const ticket = params.get("spotify_login") === "true" ? params.get("ticket")?.trim() : "";
  if (ticket) {
    window.sessionStorage.setItem(SPOTIFY_LOGIN_TICKET_KEY, ticket);
  }

  history.replaceState(history.state, "", `${location.pathname}${location.search}#${route}`);
  return ticket || null;
};
