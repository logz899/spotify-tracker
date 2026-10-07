import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HashRouter } from "react-router-dom";
import Register from "./Register";
import { linkPendingSpotify, register } from "../services/api";
import { getToken } from "../services/authSession";
import { get as getPending } from "../services/pendingSpotify";

jest.mock("../services/api", () => ({
  register: jest.fn(),
  linkPendingSpotify: jest.fn(),
}));

beforeEach(() => {
  jest.clearAllMocks();
  window.sessionStorage.clear();
  window.localStorage.clear();
  window.history.replaceState({}, "", "/");
  window.location.hash = "#/register";
});

function renderRegister(hash = "#/register") {
  window.location.hash = hash;
  return render(
    <HashRouter>
      <Register />
    </HashRouter>
  );
}

async function fillRegistration(password = "long-password", confirmation = password) {
  await userEvent.type(screen.getByLabelText(/^username$/i), "listener");
  await userEvent.type(screen.getByLabelText(/^email$/i), "listener@example.test");
  await userEvent.type(screen.getByLabelText(/^password$/i), password);
  await userEvent.type(screen.getByLabelText(/confirm password/i), confirmation);
  await userEvent.click(screen.getByRole("button", { name: /create account/i }));
}

test("stores a normal registration in the tab session and navigates home", async () => {
  register.mockResolvedValue({ token: "jwt-token", user: { username: "listener" } });
  renderRegister();

  await fillRegistration();

  await waitFor(() => expect(window.location.hash).toBe("#/home"));
  expect(getToken()).toBe("jwt-token");
  expect(window.localStorage.getItem("token")).toBeNull();
});

test("captures, links, and clears a pending Spotify key after registration", async () => {
  register.mockResolvedValue({ token: "jwt-token", user: { username: "listener" } });
  linkPendingSpotify.mockResolvedValue({ linked: true });
  renderRegister("#/register?spotify_connected=true&key=pending-key");

  await waitFor(() => expect(window.location.hash).toBe("#/register"));
  await fillRegistration();

  await waitFor(() => expect(linkPendingSpotify).toHaveBeenCalledWith("pending-key"));
  expect(getPending()).toBeNull();
  expect(window.location.hash).toBe("#/home");
});

test("rejects mismatched passwords before calling the API", async () => {
  renderRegister();

  await fillRegistration("long-password", "different-password");

  expect(screen.getByRole("alert")).toHaveTextContent(/passwords do not match/i);
  expect(register).not.toHaveBeenCalled();
});

test("rejects passwords below the backend ten-character minimum", async () => {
  renderRegister();

  await fillRegistration("shortpass");

  expect(screen.getByRole("alert")).toHaveTextContent(/at least 10 characters/i);
  expect(register).not.toHaveBeenCalled();
});
