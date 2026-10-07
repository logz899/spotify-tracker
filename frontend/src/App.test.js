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
  expect(screen.getByRole("heading", { name: /spotify tracker/i })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /^log in$/i })).toBeInTheDocument();
});

test("renders registration on direct hash navigation", () => {
  renderAt("#/register");
  expect(screen.getByRole("heading", { name: /create account/i })).toBeInTheDocument();
});

test("renders protected home on direct hash navigation with a tab session", async () => {
  renderAt("#/home", true);
  expect(await screen.findByLabelText(/logged in as test-user/i)).toBeInTheDocument();
});

test("unknown hash route returns to login without a server navigation", async () => {
  renderAt("#/missing");

  await waitFor(() => expect(window.location.hash).toBe("#/"));
  expect(screen.getByRole("button", { name: /^log in$/i })).toBeInTheDocument();
});
