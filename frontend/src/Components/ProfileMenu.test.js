import { fireEvent, render, screen } from "@testing-library/react";
import ProfileMenu from "./ProfileMenu";

const linkedProfile = {
  user: { id: 17, username: "listener" },
  spotify: {
    linked: true,
    display_name: "Listener Name",
    avatar_url: "https://i.scdn.co/image/listener",
    profile_url: "https://open.spotify.com/user/listener",
  },
  sync: { status: "completed", last_synced_at: "2026-10-12T08:55:00Z" },
};

const renderMenu = (overrides = {}) => {
  const props = {
    profile: linkedProfile,
    syncing: false,
    onSync: jest.fn(),
    onConnect: jest.fn(),
    onLogout: jest.fn(),
    ...overrides,
  };
  render(<ProfileMenu {...props} />);
  return props;
};

const openMenu = () => fireEvent.click(screen.getByRole("button", { name: /account menu/i }));

test("renders linked account actions", () => {
  renderMenu();
  expect(screen.getByRole("img", { name: /listener name/i })).toHaveAttribute(
    "src",
    linkedProfile.spotify.avatar_url
  );

  openMenu();

  expect(screen.getByText("Listener Name")).toBeInTheDocument();
  expect(screen.getByText("@listener")).toBeInTheDocument();
  expect(screen.queryByText("Spotify connected")).not.toBeInTheDocument();
  expect(screen.queryByText(/Last synced/)).not.toBeInTheDocument();
  expect(screen.getByRole("menuitem", { name: /sync now/i })).toBeInTheDocument();
  expect(screen.getByRole("menuitem", { name: /spotify profile/i })).toBeInTheDocument();
  expect(screen.getByRole("menuitem", { name: /log out/i })).toBeInTheDocument();
});

test("uses a Unicode username fallback when avatar is absent", () => {
  renderMenu({
    profile: {
      user: { id: 18, username: "🎧listener" },
      spotify: { linked: true, display_name: "", avatar_url: "" },
      sync: {},
    },
  });

  expect(screen.getByText("🎧")).toBeInTheDocument();
});

test("falls back when the avatar fails to load", () => {
  renderMenu();
  fireEvent.error(screen.getByRole("img", { name: /listener name/i }));
  expect(screen.getByText("L")).toBeInTheDocument();
  expect(screen.queryByRole("img", { name: /listener name/i })).not.toBeInTheDocument();
});

test("renders unlinked account actions", () => {
  renderMenu({
    profile: { user: { id: 19, username: "localuser" }, spotify: { linked: false }, sync: {} },
  });
  openMenu();

  expect(screen.queryByText("Spotify not connected")).not.toBeInTheDocument();
  expect(screen.queryByText("Not synced yet")).not.toBeInTheDocument();
  expect(screen.getByRole("menuitem", { name: /connect spotify/i })).toBeInTheDocument();
  expect(screen.queryByRole("menuitem", { name: /sync/i })).not.toBeInTheDocument();
  expect(screen.queryByRole("menuitem", { name: /spotify profile/i })).not.toBeInTheDocument();
});

test("renders an unavailable state without unsafe account actions", () => {
  renderMenu({
    profile: { user: { id: 20, username: "fallback" }, spotify: { linked: null }, sync: {} },
  });
  openMenu();

  expect(screen.queryByText("Spotify status unavailable")).not.toBeInTheDocument();
  expect(screen.queryByRole("menuitem", { name: /connect spotify/i })).not.toBeInTheDocument();
  expect(screen.queryByRole("menuitem", { name: /sync/i })).not.toBeInTheDocument();
  expect(screen.queryByRole("menuitem", { name: /spotify profile/i })).not.toBeInTheDocument();
  expect(screen.getByRole("menuitem", { name: /log out/i })).toBeInTheDocument();
});

test("uses the avatar as the only visible menu trigger", () => {
  renderMenu();
  const trigger = screen.getByRole("button", { name: /spotify connected/i });

  expect(trigger).toHaveTextContent("");
  expect(trigger.querySelector(".profile-menu__chevron")).not.toBeInTheDocument();
});

test("exposes menu ARIA state", () => {
  renderMenu();
  const trigger = screen.getByRole("button", { name: /account menu/i });
  expect(trigger).toHaveAttribute("aria-haspopup", "menu");
  expect(trigger).toHaveAttribute("aria-expanded", "false");
  fireEvent.click(trigger);
  expect(trigger).toHaveAttribute("aria-expanded", "true");
  expect(screen.getByRole("menu")).toBeInTheDocument();
});

test("supports keyboard and outside dismissal", () => {
  renderMenu();
  const trigger = screen.getByRole("button", { name: /account menu/i });

  trigger.focus();
  fireEvent.keyDown(trigger, { key: "Enter" });
  expect(screen.getByRole("menuitem", { name: /sync now/i })).toHaveFocus();

  fireEvent.keyDown(window, { key: "Escape" });
  expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  expect(trigger).toHaveFocus();

  fireEvent.click(trigger);
  fireEvent.pointerDown(document.body);
  expect(screen.queryByRole("menu")).not.toBeInTheDocument();
});

test("disables sync while in progress", () => {
  renderMenu({ syncing: true });
  openMenu();
  expect(screen.getByRole("menuitem", { name: /syncing/i })).toBeDisabled();
});

test("opens Spotify safely", () => {
  renderMenu();
  openMenu();
  expect(screen.getByRole("menuitem", { name: /spotify profile/i })).toHaveAttribute(
    "target",
    "_blank"
  );
  expect(screen.getByRole("menuitem", { name: /spotify profile/i })).toHaveAttribute(
    "rel",
    "noreferrer"
  );
});
