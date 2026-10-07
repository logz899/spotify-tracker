const TOKEN_KEY = "token";
const USER_KEY = "user";

export const getToken = () => window.sessionStorage.getItem(TOKEN_KEY);

export const getUser = () => {
  const rawUser = window.sessionStorage.getItem(USER_KEY);
  if (!rawUser) {
    return null;
  }
  try {
    return JSON.parse(rawUser);
  } catch {
    window.sessionStorage.removeItem(USER_KEY);
    return null;
  }
};

export const setSession = (token, user) => {
  window.sessionStorage.setItem(TOKEN_KEY, token);
  window.sessionStorage.setItem(USER_KEY, JSON.stringify(user));
};

export const clearSession = () => {
  window.sessionStorage.removeItem(TOKEN_KEY);
  window.sessionStorage.removeItem(USER_KEY);
};
