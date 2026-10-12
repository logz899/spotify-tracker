import React, { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { linkPendingSpotify, register } from "../services/api";
import { setSession } from "../services/authSession";
import * as pendingSpotify from "../services/pendingSpotify";
import AuthBackdrop from "../Components/AuthBackdrop";
import "./Register.css";

const PANEL_EXIT_MS = 360;

const Register = () => {
  const navigate = useNavigate();
  const [formData, setFormData] = useState({
    username: "",
    email: "",
    password: "",
    confirmPassword: "",
  });
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [spotifyPending, setSpotifyPending] = useState(false);
  const [canContinue, setCanContinue] = useState(false);
  const [panelClosing, setPanelClosing] = useState(false);
  const closeTimer = useRef(null);

  const closePanel = useCallback(() => {
    if (panelClosing || closeTimer.current !== null) return;

    setPanelClosing(true);
    const reducedMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
    closeTimer.current = window.setTimeout(() => {
      closeTimer.current = null;
      navigate("/");
    }, reducedMotion ? 0 : PANEL_EXIT_MS);
  }, [navigate, panelClosing]);

  useEffect(() => () => {
    if (closeTimer.current !== null) window.clearTimeout(closeTimer.current);
  }, []);

  useEffect(() => {
    pendingSpotify.captureFromLocation();
    setSpotifyPending(Boolean(pendingSpotify.get()));
  }, []);

  useEffect(() => {
    const closeOnEscape = (event) => {
      if (event.key === "Escape") closePanel();
    };

    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [closePanel]);

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

    // Validation
    if (formData.password !== formData.confirmPassword) {
      setError("Passwords do not match");
      return;
    }

    if (formData.password.length < 10) {
      setError("Password must be at least 10 characters");
      return;
    }

    setLoading(true);

    try {
      const response = await register({
        username: formData.username,
        email: formData.email,
        password: formData.password,
      });

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

      // Redirect to home page
      navigate("/home");
    } catch (err) {
      setError(err.response?.data?.error || "Registration failed. Please try again.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <main className="auth-page register-page">
      <AuthBackdrop inactive onLogin={() => navigate("/")} onRegister={() => {}} />

      <div className={`register-overlay${panelClosing ? " register-overlay--closing" : ""}`}>
        <button
          className="register-overlay__backdrop"
          type="button"
          aria-label="Close registration backdrop"
          onClick={closePanel}
        />

        <section className="register-panel" role="dialog" aria-modal="true" aria-labelledby="register-title">
          <button className="register-panel__close" type="button" aria-label="Back to log in" onClick={closePanel}>
            <span /><span />
          </button>
        <div className="register-card">
          <div className="register-brand">
            <h1 id="register-title">Create account</h1>
          </div>

          {spotifyPending && (
            <div className="register-notice register-notice--success" role="status">
              <span className="register-notice-dot" aria-hidden="true" />
              Spotify connected. Your account will be linked after registration.
            </div>
          )}

          {error && (
            <div className="register-notice register-notice--error" role="alert">
              <span className="register-notice-dot" aria-hidden="true" />
              <span>{error}</span>
              {canContinue && (
                <button type="button" className="register-text-link" onClick={() => navigate("/home")}>
                  Continue to Home
                </button>
              )}
            </div>
          )}

          <form className="register-form" onSubmit={handleSubmit} noValidate>
            <label className="register-form-field">
              <span>Username</span>
              <input
                type="text"
                id="username"
                name="username"
                value={formData.username}
                onChange={handleChange}
                placeholder="Choose a username"
                autoComplete="username"
                autoCapitalize="none"
                spellCheck="false"
                required
                minLength={3}
                maxLength={50}
              />
            </label>

            <label className="register-form-field">
              <span>Email</span>
              <input
                type="email"
                id="email"
                name="email"
                value={formData.email}
                onChange={handleChange}
                placeholder="your@email.com"
                autoComplete="email"
                required
              />
            </label>

            <label className="register-form-field">
              <span>Password</span>
              <input
                type="password"
                id="password"
                name="password"
                value={formData.password}
                onChange={handleChange}
                placeholder="At least 10 characters"
                autoComplete="new-password"
                required
                minLength={10}
              />
            </label>

            <label className="register-form-field">
              <span>Confirm Password</span>
              <input
                type="password"
                id="confirmPassword"
                name="confirmPassword"
                value={formData.confirmPassword}
                onChange={handleChange}
                placeholder="Re-enter your password"
                autoComplete="new-password"
                required
              />
            </label>

            <button type="submit" className="register-btn" disabled={loading} aria-busy={loading}>
              <span>{loading ? "Creating account..." : "Create account"}</span>
              {!loading && <span aria-hidden="true">↗</span>}
            </button>
          </form>
        </div>
        <p className="register-privacy">Your Spotify credentials stay with Spotify.</p>
        </section>
      </div>
    </main>
  );
};

export default Register;
