import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HashRouter } from "react-router-dom";
import Login from "./Login";
import {
  exchangeSpotifySession,
  linkPendingSpotify,
  login,
  loginWithSpotify,
} from "../services/api";
import {
  consumeSessionExpiredNotice,
  getToken,
  setSessionExpiredNotice,
} from "../services/authSession";
import { get as getPending } from "../services/pendingSpotify";

jest.mock("../services/api", () => ({
  login: jest.fn(),
  exchangeSpotifySession: jest.fn(),
  linkPendingSpotify: jest.fn(),
  loginWithSpotify: jest.fn(),
}));

beforeEach(() => {
  jest.clearAllMocks();
  window.sessionStorage.clear();
  window.localStorage.clear();
  window.history.replaceState({}, "", "/");
  window.location.hash = "#/";
});

function renderLogin(hash = "#/") {
  window.location.hash = hash;
  return render(
    <HashRouter>
      <Login />
    </HashRouter>
  );
}

async function submitLogin() {
  if (!screen.queryByLabelText(/^email$/i)) {
    await userEvent.click(screen.getByRole("button", { name: /^log in$/i }));
  }
  await userEvent.type(screen.getByLabelText(/^email$/i), "listener@example.test");
  await userEvent.type(screen.getByLabelText(/password/i), "long-password");
  await userEvent.click(screen.getByRole("button", { name: /^log in to account$/i }));
}

test("uses an Email field for application login", async () => {
  renderLogin();
  await userEvent.click(screen.getByRole("button", { name: /^log in$/i }));

  const email = screen.getByLabelText(/^email$/i);
  expect(email).toHaveAttribute("type", "email");
  expect(email).toHaveAttribute("autocomplete", "email");
});

test("presents a restrained landing screen before opening the login panel", () => {
  renderLogin();

  expect(screen.getByText("Spotify Tracker")).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: /music, in order/i })).toBeInTheDocument();
  expect(screen.queryByText(/your listening/i)).not.toBeInTheDocument();
  expect(screen.queryByRole("dialog", { name: /welcome back/i })).not.toBeInTheDocument();
});

test("opens login with a one-time message after the session expires", async () => {
  setSessionExpiredNotice();

  renderLogin();

  expect(await screen.findByRole("dialog", { name: /welcome back/i })).toBeInTheDocument();
  expect(screen.getByRole("alert")).toHaveTextContent("Your session expired. Please log in again.");
  expect(consumeSessionExpiredNotice()).toBe(false);
});

test("plays the exit state before removing the login panel", async () => {
  jest.useFakeTimers();
  renderLogin();

  fireEvent.click(screen.getByRole("button", { name: /^log in$/i }));

  const dialog = screen.getByRole("dialog", { name: /welcome back/i });
  expect(screen.getByRole("heading", { name: /welcome back/i })).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: /close login/i }));

  expect(dialog).toBeInTheDocument();
  expect(dialog.closest(".login-overlay")).toHaveClass("login-overlay--closing");

  act(() => jest.runOnlyPendingTimers());
  expect(screen.queryByRole("dialog", { name: /welcome back/i })).not.toBeInTheDocument();
  jest.useRealTimers();
});

test("closes the login panel with the Escape key", async () => {
  renderLogin();

  await userEvent.click(screen.getByRole("button", { name: /^log in$/i }));
  expect(screen.getByRole("dialog", { name: /welcome back/i })).toBeInTheDocument();

  await userEvent.keyboard("{Escape}");
  await waitFor(() => {
    expect(screen.queryByRole("dialog", { name: /welcome back/i })).not.toBeInTheDocument();
  });
});

test("stores a normal login in the tab session and navigates home", async () => {
  login.mockResolvedValue({ token: "jwt-token", user: { username: "listener" } });
  renderLogin();

  await submitLogin();

  await waitFor(() => expect(window.location.hash).toBe("#/home"));
  expect(getToken()).toBe("jwt-token");
  expect(window.localStorage.getItem("token")).toBeNull();
  expect(linkPendingSpotify).not.toHaveBeenCalled();
  expect(login).toHaveBeenCalledWith("listener@example.test", "long-password");
});

test("exchanges a linked Spotify ticket once, stores the session, and navigates home", async () => {
  exchangeSpotifySession.mockResolvedValue({
    token: "spotify-jwt",
    user: { username: "listener" },
  });

  renderLogin("#/?spotify_login=true&ticket=login-ticket");

  await waitFor(() => expect(window.location.hash).toBe("#/home"));
  expect(exchangeSpotifySession).toHaveBeenCalledTimes(1);
  expect(exchangeSpotifySession).toHaveBeenCalledWith("login-ticket");
  expect(getToken()).toBe("spotify-jwt");
  expect(window.sessionStorage.getItem("spotify_login_ticket")).toBeNull();
});

test("clears an expired Spotify ticket and offers Spotify retry or Email login", async () => {
  exchangeSpotifySession.mockRejectedValue({ response: { status: 400 } });

  renderLogin("#/?spotify_login=true&ticket=expired-ticket");

  expect(await screen.findByRole("alert")).toHaveTextContent(/spotify login expired/i);
  expect(window.sessionStorage.getItem("spotify_login_ticket")).toBeNull();
  expect(screen.getByRole("button", { name: /try spotify again/i })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /use email login/i })).toBeInTheDocument();

  await userEvent.click(screen.getByRole("button", { name: /try spotify again/i }));
  expect(loginWithSpotify).toHaveBeenCalledTimes(1);
});

test("captures, links, and clears a pending Spotify key after login", async () => {
  login.mockResolvedValue({ token: "jwt-token", user: { username: "listener" } });
  linkPendingSpotify.mockResolvedValue({ linked: true });
  renderLogin("#/?spotify_connected=true&key=pending-key");

  await waitFor(() => expect(window.location.hash).toBe("#/"));
  expect(getPending()).toBe("pending-key");
  await submitLogin();

  await waitFor(() => expect(linkPendingSpotify).toHaveBeenCalledWith("pending-key"));
  expect(getPending()).toBeNull();
  expect(window.location.hash).toBe("#/home");
});

test("expired pending link is cleared and offers a recoverable path to home", async () => {
  login.mockResolvedValue({ token: "jwt-token", user: { username: "listener" } });
  linkPendingSpotify.mockRejectedValue({ response: { status: 400 } });
  renderLogin("#/?spotify_connected=true&key=expired-key");

  await submitLogin();

  expect(await screen.findByRole("alert")).toHaveTextContent(/spotify connection expired/i);
  expect(getPending()).toBeNull();
  expect(getToken()).toBe("jwt-token");
  await userEvent.click(screen.getByRole("button", { name: /continue to home/i }));
  expect(window.location.hash).toBe("#/home");
});

test("shows the backend login error without creating a session", async () => {
  login.mockRejectedValue({ response: { data: { error: "Invalid credentials" } } });
  renderLogin();

  await submitLogin();

  expect(await screen.findByRole("alert")).toHaveTextContent("Invalid credentials");
  expect(getToken()).toBeNull();
});
