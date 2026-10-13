import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
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

test("uses the Spotify Tracker brand", () => {
  renderRegister();

  expect(screen.getByText("Spotify Tracker")).toBeInTheDocument();
});

test("plays the exit state before returning to the landing screen", () => {
  jest.useFakeTimers();
  renderRegister();

  fireEvent.click(screen.getByRole("button", { name: /back to log in/i }));

  const dialog = screen.getByRole("dialog", { name: /create account/i });
  expect(dialog).toBeInTheDocument();
  expect(dialog.closest(".register-overlay")).toHaveClass("register-overlay--closing");

  act(() => jest.runOnlyPendingTimers());
  expect(window.location.hash).toBe("#/");
  jest.useRealTimers();
});

test("keeps the landing story visible behind the registration panel", () => {
  renderRegister();

  expect(screen.getByText(/music,/i)).toBeInTheDocument();
  expect(screen.getByRole("dialog", { name: /create account/i })).toBeInTheDocument();
});

test("closes registration when the backdrop is clicked", async () => {
  renderRegister();

  await userEvent.click(screen.getByRole("button", { name: /close registration backdrop/i }));

  await waitFor(() => expect(window.location.hash).toBe("#/"));
});

test("closes registration with the Escape key", async () => {
  renderRegister();

  await userEvent.keyboard("{Escape}");

  await waitFor(() => expect(window.location.hash).toBe("#/"));
});

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

test("offers Email login without dropping a pending Spotify authorization", async () => {
  renderRegister("#/register?spotify_connected=true&key=pending-key");

  await waitFor(() => expect(getPending()).toBe("pending-key"));
  await userEvent.click(screen.getByRole("button", { name: /already have an account/i }));

  expect(window.location.hash).toBe("#/");
  expect(getPending()).toBe("pending-key");
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

test("shows a reason beside every invalid registration field", async () => {
  renderRegister();

  await userEvent.type(screen.getByLabelText(/^username$/i), "ab");
  await userEvent.type(screen.getByLabelText(/^email$/i), "not-an-email");
  await userEvent.type(screen.getByLabelText(/^password$/i), "short");
  await userEvent.type(screen.getByLabelText(/confirm password/i), "different");
  await userEvent.click(screen.getByRole("button", { name: /create account/i }));

  expect(screen.getByText("Username must be between 3 and 50 characters.")).toBeInTheDocument();
  expect(screen.getByText("Enter a valid email address.")).toBeInTheDocument();
  expect(screen.getByText("Password must be at least 10 characters.")).toBeInTheDocument();
  expect(screen.getByText("Passwords do not match.")).toBeInTheDocument();
  expect(register).not.toHaveBeenCalled();
});

test.each([
  ["Username already exists", "Username is already in use."],
  ["Email already exists", "Email is already in use."],
])("maps backend conflict %s to its field", async (backendError, visibleReason) => {
  register.mockRejectedValue({ response: { status: 409, data: { error: backendError } } });
  renderRegister();

  await fillRegistration();

  expect(await screen.findByText(visibleReason)).toBeInTheDocument();
});

test("explains when the registration API cannot be reached", async () => {
  register.mockRejectedValue(new Error("Network Error"));
  renderRegister();

  await fillRegistration();

  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Cannot connect to the server. Check your connection and try again."
  );
});

test("uses a safe reason for unexpected server failures", async () => {
  register.mockRejectedValue({ response: { status: 500, data: { error: "sensitive database detail" } } });
  renderRegister();

  await fillRegistration();

  const alert = await screen.findByRole("alert");
  expect(alert).toHaveTextContent("Server could not create the account. Please try again.");
  expect(alert).not.toHaveTextContent("sensitive database detail");
});
