import React, { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { linkPendingSpotify, login, loginWithSpotify } from "../services/api";
import { setSession } from "../services/authSession";
import * as pendingSpotify from "../services/pendingSpotify";
import AuthBackdrop from "../Components/AuthBackdrop";
import "./Login.css";

const SpotifyIcon = () => (
  <svg className="spotify-btn-icon" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
    <path d="M12 0C5.4 0 0 5.4 0 12s5.4 12 12 12 12-5.4 12-12S18.66 0 12 0zm5.521 17.34c-.24.359-.66.48-1.021.24-2.82-1.74-6.36-2.101-10.561-1.141-.418.122-.779-.179-.899-.539-.12-.421.18-.78.54-.9 4.56-1.021 8.52-.6 11.64 1.32.42.18.479.659.301 1.02zm1.44-3.3c-.301.42-.841.6-1.262.3-3.239-1.98-8.159-2.58-11.939-1.38-.479.12-1.02-.12-1.14-.6-.12-.48.12-1.021.6-1.141C9.6 9.9 15 10.561 18.72 12.84c.361.181.54.78.241 1.2zm.12-3.36C15.24 8.4 8.82 8.16 5.16 9.301c-.6.179-1.2-.181-1.38-.721-.18-.601.18-1.2.72-1.381 4.26-1.26 11.28-1.02 15.721 1.621.539.3.719 1.02.419 1.56-.299.421-1.02.599-1.559.3z" />
  </svg>
);

const PANEL_EXIT_MS = 360;

const Login = () => {
  const navigate = useNavigate();
  const [formData, setFormData] = useState({ username: "", password: "" });
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [spotifyPending, setSpotifyPending] = useState(false);
  const [canContinue, setCanContinue] = useState(false);
  const [panelOpen, setPanelOpen] = useState(false);
  const [panelClosing, setPanelClosing] = useState(false);
  const closeTimer = useRef(null);

  const openPanel = useCallback(() => {
    if (closeTimer.current !== null) window.clearTimeout(closeTimer.current);
    closeTimer.current = null;
    setPanelClosing(false);
    setPanelOpen(true);
  }, []);

  const closePanel = useCallback(() => {
    if (panelClosing || closeTimer.current !== null) return;

    setPanelClosing(true);
    const reducedMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
    closeTimer.current = window.setTimeout(() => {
      closeTimer.current = null;
      setPanelOpen(false);
      setPanelClosing(false);
    }, reducedMotion ? 0 : PANEL_EXIT_MS);
  }, [panelClosing]);

  useEffect(() => () => {
    if (closeTimer.current !== null) window.clearTimeout(closeTimer.current);
  }, []);

  useEffect(() => {
    pendingSpotify.captureFromLocation();
    const hasPendingSpotify = Boolean(pendingSpotify.get());
    setSpotifyPending(hasPendingSpotify);
    setPanelOpen(hasPendingSpotify);
  }, []);

  useEffect(() => {
    if (!panelOpen) return undefined;

    const closeOnEscape = (event) => {
      if (event.key === "Escape") closePanel();
    };

    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [closePanel, panelOpen]);

  const handleChange = (event) => {
    setFormData({ ...formData, [event.target.name]: event.target.value });
    setError("");
  };

  const handleSubmit = async (event) => {
    event.preventDefault();
    setError("");
    setLoading(true);

    try {
      const response = await login(formData.username, formData.password);
      setSession(response.token, response.user);

      const pendingKey = pendingSpotify.get();
      if (pendingKey) {
        try {
          await linkPendingSpotify(pendingKey);
          pendingSpotify.clear();
        } catch {
          pendingSpotify.clear();
          setSpotifyPending(false);
          setCanContinue(true);
          setError("Spotify connection expired. Continue to Home and connect Spotify again.");
          return;
        }
      }

      navigate("/home");
    } catch (err) {
      setError(err.response?.data?.error || "Login failed. Please check your credentials.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <main className="auth-page login-page">
      <AuthBackdrop inactive={panelOpen} onLogin={openPanel} onRegister={() => navigate("/register")} />

      {panelOpen && (
        <div className={`login-overlay${panelClosing ? " login-overlay--closing" : ""}`} onMouseDown={(event) => {
          if (event.target === event.currentTarget) closePanel();
        }}>
          <section className="login-panel" role="dialog" aria-modal="true" aria-labelledby="login-title">
            <button className="login-panel__close" type="button" aria-label="Close login" onClick={closePanel}>
              <span /><span />
            </button>

            <div className="login-card">
              <div className="login-brand">
                <h2 id="login-title">Welcome back</h2>
              </div>

              {spotifyPending && (
                <div className="login-notice login-notice--success" role="status">
                  <span className="login-notice-dot" aria-hidden="true" />
                  Spotify connected. Log in to link it to your account.
                </div>
              )}

              {error && (
                <div className="login-notice login-notice--error" role="alert">
                  <span className="login-notice-dot" aria-hidden="true" />
                  <span>{error}</span>
                  {canContinue && <button type="button" className="text-link" onClick={() => navigate("/home")}>Continue to Home</button>}
                </div>
              )}

              <form className="login-form" onSubmit={handleSubmit} noValidate>
                <label className="form-field">
                  <span>Username</span>
                  <input type="text" name="username" value={formData.username} onChange={handleChange} autoComplete="username" autoCapitalize="none" spellCheck="false" required />
                </label>
                <label className="form-field">
                  <span>Password</span>
                  <input type="password" name="password" value={formData.password} onChange={handleChange} autoComplete="current-password" required />
                </label>
                <button type="submit" className="panel-action panel-action--primary" disabled={loading} aria-busy={loading} aria-label="Log in to account">
                  <span>{loading ? "Logging in..." : "Log in"}</span>
                  {!loading && <span aria-hidden="true">↗</span>}
                </button>
              </form>

              <div className="login-divider"><span>or</span></div>

              <button className="panel-action panel-action--spotify" type="button" onClick={loginWithSpotify} aria-label="Continue with Spotify">
                <SpotifyIcon /><span>Continue with Spotify</span>
              </button>

              <p className="login-footer">No account? <button className="text-link" onClick={() => navigate("/register")} type="button">Create one</button></p>
            </div>

            <p className="login-privacy">Your Spotify credentials stay with Spotify.</p>
          </section>
        </div>
      )}
    </main>
  );
};

export default Login;
