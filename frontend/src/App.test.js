import { render, screen, waitFor } from "@testing-library/react";
import App from "./App";

jest.mock("./services/api", () => ({
  getAllSongs: jest.fn().mockResolvedValue({ data: [] }),
  getRecentSongs: jest.fn().mockResolvedValue({ message: [] }),
  logout: jest.fn(),
}));

function renderAt(hash, authenticated = false) {
  window.sessionStorage.clear();
  window.localStorage.clear();
  if (authenticated) {
    window.sessionStorage.setItem("token", "test-token");
    window.sessionStorage.setItem("user", JSON.stringify({ username: "test-user" }));
  }
  window.location.hash = hash;
  return render(<App />);
}

test("renders login at the root hash route", () => {
  renderAt("#/");
  expect(screen.getByRole("heading", { name: /music, in order/i })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /^log in$/i })).toBeInTheDocument();
});

test("renders registration on direct hash navigation", () => {
  renderAt("#/register");
  expect(screen.getByRole("heading", { name: /create account/i })).toBeInTheDocument();
});

test("redirects an authenticated user from the login route to home", async () => {
  renderAt("#/", true);

  await waitFor(() => expect(window.location.hash).toBe("#/home"));
  expect(await screen.findByLabelText(/account menu for test-user/i)).toBeInTheDocument();
});

test("redirects an authenticated user from registration to home", async () => {
  renderAt("#/register", true);

  await waitFor(() => expect(window.location.hash).toBe("#/home"));
  expect(await screen.findByLabelText(/account menu for test-user/i)).toBeInTheDocument();
});

test("redirects an unauthenticated user from home to login", async () => {
  renderAt("#/home");

  await waitFor(() => expect(window.location.hash).toBe("#/"));
  expect(screen.getByRole("button", { name: /^log in$/i })).toBeInTheDocument();
});

test("renders protected home on direct hash navigation with a tab session", async () => {
  renderAt("#/home", true);
  expect(await screen.findByLabelText(/account menu for test-user/i)).toBeInTheDocument();
  expect(screen.getByText("Spotify Tracker")).toBeInTheDocument();
});

test("unknown hash route returns to login without a server navigation", async () => {
  renderAt("#/missing");

  await waitFor(() => expect(window.location.hash).toBe("#/"));
  expect(screen.getByRole("button", { name: /^log in$/i })).toBeInTheDocument();
});

test("unknown hash route returns an authenticated user to home", async () => {
  renderAt("#/missing", true);

  await waitFor(() => expect(window.location.hash).toBe("#/home"));
  expect(await screen.findByLabelText(/account menu for test-user/i)).toBeInTheDocument();
});
