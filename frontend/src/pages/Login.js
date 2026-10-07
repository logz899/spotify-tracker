import React, { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { linkPendingSpotify, login, loginWithSpotify } from "../services/api";
import { setSession } from "../services/authSession";
import * as pendingSpotify from "../services/pendingSpotify";
import "./Login.css";

/* Spotify logo mark SVG — inline so we avoid an external dependency */
const SpotifyIcon = () => (
  <svg
    className="spotify-btn-icon"
    viewBox="0 0 24 24"
    xmlns="http://www.w3.org/2000/svg"
    aria-hidden="true"
  >
    <path d="M12 0C5.4 0 0 5.4 0 12s5.4 12 12 12 12-5.4 12-12S18.66 0 12 0zm5.521 17.34c-.24.359-.66.48-1.021.24-2.82-1.74-6.36-2.101-10.561-1.141-.418.122-.779-.179-.899-.539-.12-.421.18-.78.54-.9 4.56-1.021 8.52-.6 11.64 1.32.42.18.479.659.301 1.02zm1.44-3.3c-.301.42-.841.6-1.262.3-3.239-1.98-8.159-2.58-11.939-1.38-.479.12-1.02-.12-1.14-.6-.12-.48.12-1.021.6-1.141C9.6 9.9 15 10.561 18.72 12.84c.361.181.54.78.241 1.2zm.12-3.36C15.24 8.4 8.82 8.16 5.16 9.301c-.6.179-1.2-.181-1.38-.721-.18-.601.18-1.2.72-1.381 4.26-1.26 11.28-1.02 15.721 1.621.539.3.719 1.02.419 1.56-.299.421-1.02.599-1.559.3z" />
  </svg>
);

/* Music note icon for the brand logo */
const MusicNoteIcon = () => (
  <svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
    <path d="M12 3v10.55c-.59-.34-1.27-.55-2-.55-2.21 0-4 1.79-4 4s1.79 4 4 4 4-1.79 4-4V7h4V3h-6z" />
  </svg>
);

const Login = () => {
  const navigate = useNavigate();
  const [formData, setFormData] = useState({
    username: "",
    password: "",
  });
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [spotifyPending, setSpotifyPending] = useState(false);
  const [canContinue, setCanContinue] = useState(false);

  useEffect(() => {
    pendingSpotify.captureFromLocation();
    setSpotifyPending(Boolean(pendingSpotify.get()));
  }, []);

  const handleChange = (e) => {
    setFormData({
      ...formData,
      [e.target.name]: e.target.value,
    });
    setError("");
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
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

  const handleSpotifyLogin = () => {
    loginWithSpotify();
  };

  return (
    <div className="login-page">
      <div className="login-card">
        {/* Brand header */}
        <div className="login-brand">
          <div className="login-brand-logo" aria-hidden="true">
            <MusicNoteIcon />
          </div>
          <h1 className="login-brand-title">Spotify Tracker</h1>
          <p className="login-brand-subtitle">Track your most played songs</p>
        </div>

        <div className="login-body">
          {/* Spotify pending notice */}
          {spotifyPending && (
            <div className="login-notice login-notice--success" role="status">
              <span className="login-notice-dot" aria-hidden="true" />
              Spotify connected. Log in to link it to your account.
            </div>
          )}

          {/* Error notice */}
          {error && (
            <div className="login-notice login-notice--error" role="alert">
              <span className="login-notice-dot" aria-hidden="true" />
              {error}
              {canContinue && (
                <button type="button" className="text-link" onClick={() => navigate("/home")}>
                  Continue to Home
                </button>
              )}
            </div>
          )}

          {/* Credentials form */}
          <form className="login-form" onSubmit={handleSubmit} noValidate>
            <div className="form-field">
              <input
                type="text"
                name="username"
                value={formData.username}
                onChange={handleChange}
                placeholder="Username"
                autoComplete="username"
                autoCapitalize="none"
                spellCheck="false"
                required
                aria-label="Username"
              />
            </div>

            <div className="form-field">
              <input
                type="password"
                name="password"
                value={formData.password}
                onChange={handleChange}
                placeholder="Password"
                autoComplete="current-password"
                required
                aria-label="Password"
              />
            </div>

            <button
              type="submit"
              className="btn btn--primary"
              disabled={loading}
              aria-busy={loading}
            >
              {loading ? (
                <>
                  <span className="btn-spinner" aria-hidden="true" />
                  Logging in...
                </>
              ) : (
                "Log in"
              )}
            </button>
          </form>

          {/* Divider */}
          <div className="login-divider" aria-hidden="true">
            <span>or</span>
          </div>

          {/* Spotify OAuth */}
          <div className="spotify-btn-wrapper">
            <button
              className="btn btn--spotify"
              onClick={handleSpotifyLogin}
              aria-label="Continue with Spotify"
            >
              <SpotifyIcon />
              Continue with Spotify
            </button>
          </div>

          {/* Register link */}
          <p className="login-footer">
            Don't have an account?{" "}
            <button className="text-link" onClick={() => navigate("/register")} type="button">
              Sign up
            </button>
          </p>
        </div>
      </div>
    </div>
  );
};

export default Login;
