import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import Home from "./Home";
import {
  getAllSongs,
  getCurrentUser,
  getRecentSongs,
  getSpotifyAuthURL,
  logout,
} from "../services/api";
import { clearSession, getToken, getUser } from "../services/authSession";

jest.mock("../services/api", () => ({
  getAllSongs: jest.fn(),
  getCurrentUser: jest.fn(),
  getRecentSongs: jest.fn(),
  getSpotifyAuthURL: jest.fn(),
  logout: jest.fn(),
}));

jest.mock("../services/authSession", () => ({
  clearSession: jest.fn(),
  getToken: jest.fn(),
  getUser: jest.fn(),
}));

const song = {
  song_name: "First Song",
  author_name: "First Artist",
  album_name: "First Album",
  song_count: 3,
  listening_date: "2026-10-12T08:00:00Z",
};

const linkedProfile = {
  user: { id: 17, username: "listener" },
  spotify: {
    linked: true,
    display_name: "Listener Name",
    avatar_url: "https://i.scdn.co/image/listener",
    profile_url: "https://open.spotify.com/user/listener",
  },
  sync: { status: "completed", last_synced_at: "2026-10-12T09:00:00Z" },
};

const renderHome = () =>
  render(
    <MemoryRouter initialEntries={["/home"]}>
      <Routes>
        <Route path="/" element={<div>Login page</div>} />
        <Route path="/home" element={<Home />} />
      </Routes>
    </MemoryRouter>
  );

beforeEach(() => {
  jest.clearAllMocks();
  getToken.mockReturnValue("jwt-token");
  getUser.mockReturnValue({ id: 17, username: "session-listener" });
  getAllSongs.mockResolvedValue({ data: [song] });
  getCurrentUser.mockResolvedValue(linkedProfile);
  getRecentSongs.mockResolvedValue({ message: [song] });
  getSpotifyAuthURL.mockResolvedValue("#spotify-auth");
});

test("profile and songs load independently on mount", async () => {
  renderHome();

  await waitFor(() => {
    expect(getCurrentUser).toHaveBeenCalledTimes(1);
    expect(getAllSongs).toHaveBeenCalledTimes(1);
  });
  expect(await screen.findByText("First Song")).toBeInTheDocument();
  expect(screen.getByRole("img", { name: /listener name/i })).toBeInTheDocument();
});

test("a non-401 profile failure preserves rankings with an unknown Spotify state", async () => {
  getCurrentUser.mockRejectedValue(new Error("profile unavailable"));
  renderHome();

  expect(await screen.findByText("First Song")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: /account menu/i }));
  expect(screen.getByText("session-listener")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /spotify status unavailable/i })).toBeInTheDocument();
  expect(screen.queryByRole("menuitem", { name: /connect spotify/i })).not.toBeInTheDocument();
  expect(screen.queryByRole("menuitem", { name: /sync/i })).not.toBeInTheDocument();
});

test("a profile 401 follows the authenticated session-expired navigation path", async () => {
  getCurrentUser.mockRejectedValue({ response: { status: 401 } });
  renderHome();

  expect(await screen.findByText("Login page")).toBeInTheDocument();
  expect(clearSession).toHaveBeenCalled();
});

test("Sync reloads songs and profile metadata", async () => {
  const updatedProfile = {
    ...linkedProfile,
    spotify: { ...linkedProfile.spotify, display_name: "Updated Listener" },
    sync: { status: "completed", last_synced_at: new Date().toISOString() },
  };
  getCurrentUser.mockResolvedValueOnce(linkedProfile).mockResolvedValueOnce(updatedProfile);
  getAllSongs
    .mockResolvedValueOnce({ data: [song] })
    .mockResolvedValueOnce({ data: [{ ...song, song_name: "Updated Song" }] });
  renderHome();

  await screen.findByText("First Song");
  fireEvent.click(screen.getByRole("button", { name: /account menu/i }));
  fireEvent.click(screen.getByRole("menuitem", { name: /sync now/i }));

  expect(await screen.findByText("Updated Song")).toBeInTheDocument();
  await waitFor(() => expect(getRecentSongs).toHaveBeenCalledTimes(1));
  expect(getAllSongs).toHaveBeenCalledTimes(2);
  expect(getCurrentUser).toHaveBeenCalledTimes(2);
  fireEvent.click(screen.getByRole("button", { name: /account menu/i }));
  expect(screen.getByText("Updated Listener")).toBeInTheDocument();
});

test("sync failure leaves the menu usable and shows a recoverable error", async () => {
  getRecentSongs.mockRejectedValue(new Error("sync unavailable"));
  renderHome();

  await screen.findByText("First Song");
  fireEvent.click(screen.getByRole("button", { name: /account menu/i }));
  fireEvent.click(screen.getByRole("menuitem", { name: /sync now/i }));

  expect(await screen.findByRole("alert")).toHaveTextContent("Failed to refresh songs");
  fireEvent.click(screen.getByRole("button", { name: /account menu/i }));
  expect(screen.getByRole("menuitem", { name: /log out/i })).toBeInTheDocument();
});

test("Connect and Logout use the existing actions", async () => {
  getCurrentUser.mockResolvedValue({
    user: { id: 17, username: "listener" },
    spotify: { linked: false },
    sync: { status: null },
  });
  renderHome();

  await screen.findByText("First Song");
  fireEvent.click(screen.getByRole("button", { name: /account menu/i }));
  fireEvent.click(screen.getByRole("menuitem", { name: /connect spotify/i }));
  await waitFor(() => expect(getSpotifyAuthURL).toHaveBeenCalledTimes(1));

  fireEvent.click(screen.getByRole("button", { name: /account menu/i }));
  fireEvent.click(screen.getByRole("menuitem", { name: /log out/i }));
  expect(logout).toHaveBeenCalledTimes(1);
});

test("the header has no standalone greeting, Sync, Connect, or Logout controls", async () => {
  renderHome();
  await screen.findByText("First Song");

  const header = screen.getByRole("banner");
  expect(header).not.toHaveTextContent(/Hey,/i);
  expect(screen.queryByRole("button", { name: /refresh songs/i })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: /connect your spotify account/i })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: /^log out$/i })).not.toBeInTheDocument();
});
