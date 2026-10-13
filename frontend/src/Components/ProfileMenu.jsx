import React, { useEffect, useRef, useState } from "react";
import "./ProfileMenu.css";

const SpotifyIcon = () => (
  <svg className="profile-menu__icon" viewBox="0 0 24 24" aria-hidden="true">
    <path
      fill="currentColor"
      d="M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20Zm4.58 14.42a.63.63 0 0 1-.86.21c-2.37-1.45-5.35-1.78-8.86-.98a.63.63 0 0 1-.28-1.22c3.84-.88 7.13-.5 9.79 1.12.3.18.39.57.21.87Zm1.23-2.73a.78.78 0 0 1-1.08.26c-2.71-1.66-6.84-2.14-10.04-1.17a.78.78 0 1 1-.46-1.5c3.66-1.11 8.21-.57 11.32 1.33.37.23.48.71.26 1.08Zm.1-2.84C14.66 8.92 9.3 8.74 6.2 9.68a.94.94 0 0 1-.54-1.79c3.56-1.08 9.47-.87 13.21 1.35a.94.94 0 0 1-.96 1.61Z"
    />
  </svg>
);

const SyncIcon = () => (
  <svg className="profile-menu__icon" viewBox="0 0 24 24" aria-hidden="true">
    <path d="M20 11a8 8 0 0 0-14.93-4M4 3v4h4M4 13a8 8 0 0 0 14.93 4M20 21v-4h-4" />
  </svg>
);

const LogoutIcon = () => (
  <svg className="profile-menu__icon" viewBox="0 0 24 24" aria-hidden="true">
    <path d="M10 5H6a1 1 0 0 0-1 1v12a1 1 0 0 0 1 1h4M15 8l4 4-4 4M19 12H9" />
  </svg>
);

const ProfileMenu = ({ profile, syncing, onSync, onConnect, onLogout }) => {
  const [open, setOpen] = useState(false);
  const [imageFailed, setImageFailed] = useState(false);
  const rootRef = useRef(null);
  const triggerRef = useRef(null);
  const firstActionRef = useRef(null);
  const keyboardOpenRef = useRef(false);

  const username = profile?.user?.username?.trim() || "listener";
  const spotify = profile?.spotify || { linked: null };
  const displayName = spotify.display_name?.trim() || username;
  const initial = [...username][0]?.toUpperCase() || "?";
  const avatarURL = spotify.avatar_url?.trim();
  const connectionState =
    spotify.linked === true
      ? "Spotify connected"
      : spotify.linked === false
        ? "Spotify not connected"
        : "Spotify status unavailable";

  useEffect(() => setImageFailed(false), [avatarURL]);

  useEffect(() => {
    if (!open) return undefined;

    const closeOutside = (event) => {
      if (!rootRef.current?.contains(event.target)) setOpen(false);
    };
    const closeForRouteChange = () => setOpen(false);
    const closeOnEscape = (event) => {
      if (event.key !== "Escape") return;
      setOpen(false);
      triggerRef.current?.focus();
    };

    document.addEventListener("pointerdown", closeOutside);
    window.addEventListener("keydown", closeOnEscape);
    window.addEventListener("popstate", closeForRouteChange);
    window.addEventListener("hashchange", closeForRouteChange);
    return () => {
      document.removeEventListener("pointerdown", closeOutside);
      window.removeEventListener("keydown", closeOnEscape);
      window.removeEventListener("popstate", closeForRouteChange);
      window.removeEventListener("hashchange", closeForRouteChange);
    };
  }, [open]);

  useEffect(() => {
    if (open && keyboardOpenRef.current) {
      firstActionRef.current?.focus();
      keyboardOpenRef.current = false;
    }
  }, [open]);

  const toggleWithPointer = () => {
    keyboardOpenRef.current = false;
    setOpen((current) => !current);
  };

  const openWithKeyboard = (event) => {
    if (event.key !== "Enter" && event.key !== " ") return;
    event.preventDefault();
    keyboardOpenRef.current = true;
    setOpen(true);
  };

  const runAction = (action) => () => {
    setOpen(false);
    action?.();
  };

  const firstActionFor = (state) => (spotify.linked === state ? firstActionRef : undefined);

  const avatar = (className) =>
    avatarURL && !imageFailed ? (
      <img
        className={className}
        src={avatarURL}
        alt={`${displayName} Spotify profile`}
        onError={() => setImageFailed(true)}
      />
    ) : (
      <span className={`${className} profile-menu__initial`} aria-hidden="true">
        {initial}
      </span>
    );

  return (
    <div className="profile-menu" ref={rootRef}>
      <button
        ref={triggerRef}
        type="button"
        className="profile-menu__trigger"
        aria-label={`${open ? "Close" : "Open"} account menu for ${username}, ${connectionState}`}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={toggleWithPointer}
        onKeyDown={openWithKeyboard}
      >
        {avatar("profile-menu__avatar")}
        <span
          className={`profile-menu__badge profile-menu__badge--${
            spotify.linked === true ? "linked" : spotify.linked === false ? "unlinked" : "unknown"
          }`}
          aria-hidden="true"
        >
          <SpotifyIcon />
        </span>
      </button>

      {open && (
        <div className="profile-menu__panel" role="menu" aria-label="Account actions">
          <div className="profile-menu__identity">
            {avatar("profile-menu__identity-avatar")}
            <div className="profile-menu__identity-copy">
              <strong>{displayName}</strong>
              {displayName !== username && <span>@{username}</span>}
            </div>
          </div>

          <div className="profile-menu__actions">
            {spotify.linked === true && (
              <button
                ref={firstActionFor(true)}
                type="button"
                role="menuitem"
                disabled={syncing}
                onClick={runAction(onSync)}
              >
                <SyncIcon />
                {syncing ? "Syncing…" : "Sync now"}
              </button>
            )}
            {spotify.linked === false && (
              <button
                ref={firstActionFor(false)}
                type="button"
                role="menuitem"
                onClick={runAction(onConnect)}
              >
                <SpotifyIcon />
                Connect Spotify
              </button>
            )}
            {spotify.linked === true && spotify.profile_url && (
              <a
                role="menuitem"
                href={spotify.profile_url}
                target="_blank"
                rel="noreferrer"
                onClick={() => setOpen(false)}
              >
                <SpotifyIcon />
                Spotify profile
              </a>
            )}
          </div>

          <div className="profile-menu__separator" />
          <button
            ref={
              spotify.linked === null || spotify.linked === undefined ? firstActionRef : undefined
            }
            className="profile-menu__logout"
            type="button"
            role="menuitem"
            onClick={runAction(onLogout)}
          >
            <LogoutIcon />
            Log out
          </button>
        </div>
      )}
    </div>
  );
};

export default ProfileMenu;
