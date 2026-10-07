import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HashRouter } from "react-router-dom";
import Login from "./Login";
import { linkPendingSpotify, login } from "../services/api";
import { getToken } from "../services/authSession";
import { get as getPending } from "../services/pendingSpotify";

jest.mock("../services/api", () => ({
  login: jest.fn(),
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
  await userEvent.type(screen.getByLabelText(/username/i), "listener");
  await userEvent.type(screen.getByLabelText(/password/i), "long-password");
  await userEvent.click(screen.getByRole("button", { name: /^log in$/i }));
}

test("stores a normal login in the tab session and navigates home", async () => {
  login.mockResolvedValue({ token: "jwt-token", user: { username: "listener" } });
  renderLogin();

  await submitLogin();

  await waitFor(() => expect(window.location.hash).toBe("#/home"));
  expect(getToken()).toBe("jwt-token");
  expect(window.localStorage.getItem("token")).toBeNull();
  expect(linkPendingSpotify).not.toHaveBeenCalled();
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
