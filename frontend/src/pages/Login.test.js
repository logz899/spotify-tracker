import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
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
  if (!screen.queryByLabelText(/username/i)) {
    await userEvent.click(screen.getByRole("button", { name: /^log in$/i }));
  }
  await userEvent.type(screen.getByLabelText(/username/i), "listener");
  await userEvent.type(screen.getByLabelText(/password/i), "long-password");
  await userEvent.click(screen.getByRole("button", { name: /^log in to account$/i }));
}

test("presents a restrained landing screen before opening the login panel", () => {
  renderLogin();

  expect(screen.getByText("Spotify Tracker")).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: /music, in order/i })).toBeInTheDocument();
  expect(screen.queryByText(/your listening/i)).not.toBeInTheDocument();
  expect(screen.queryByRole("dialog", { name: /welcome back/i })).not.toBeInTheDocument();
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
