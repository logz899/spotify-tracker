import React, { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { linkPendingSpotify, register } from "../services/api";
import { setSession } from "../services/authSession";
import * as pendingSpotify from "../services/pendingSpotify";
import AuthBackdrop from "../Components/AuthBackdrop";
import "./Register.css";

const PANEL_EXIT_MS = 360;
const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

const validateRegistration = ({ username, email, password, confirmPassword }) => {
  const errors = {};
  const normalizedUsername = username.trim();
  const normalizedEmail = email.trim();

  if (!normalizedUsername) {
    errors.username = "Username is required.";
  } else if (normalizedUsername.length < 3 || normalizedUsername.length > 50) {
    errors.username = "Username must be between 3 and 50 characters.";
  }

  if (!normalizedEmail) {
    errors.email = "Email is required.";
  } else if (!EMAIL_PATTERN.test(normalizedEmail)) {
    errors.email = "Enter a valid email address.";
  }

  if (!password) {
    errors.password = "Password is required.";
  } else if (password.length < 10) {
    errors.password = "Password must be at least 10 characters.";
  }

  if (!confirmPassword) {
    errors.confirmPassword = "Confirm your password.";
  } else if (password !== confirmPassword) {
    errors.confirmPassword = "Passwords do not match.";
  }

  return errors;
};

const Register = () => {
  const navigate = useNavigate();
  const [formData, setFormData] = useState({
    username: "",
    email: "",
    password: "",
    confirmPassword: "",
  });
  const [error, setError] = useState("");
  const [fieldErrors, setFieldErrors] = useState({});
  const [loading, setLoading] = useState(false);
  const [spotifyPending, setSpotifyPending] = useState(false);
  const [canContinue, setCanContinue] = useState(false);
  const [panelClosing, setPanelClosing] = useState(false);
  const closeTimer = useRef(null);

  const closePanel = useCallback(() => {
    if (panelClosing || closeTimer.current !== null) return;

    setPanelClosing(true);
    const reducedMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
    closeTimer.current = window.setTimeout(
      () => {
        closeTimer.current = null;
        navigate("/");
      },
      reducedMotion ? 0 : PANEL_EXIT_MS
    );
  }, [navigate, panelClosing]);

  useEffect(
    () => () => {
      if (closeTimer.current !== null) window.clearTimeout(closeTimer.current);
    },
    []
  );

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
    const { name, value } = e.target;
    setFormData((current) => ({ ...current, [name]: value }));
    setFieldErrors((current) => {
      const next = { ...current };
      delete next[name];
      if (name === "password") delete next.confirmPassword;
      return next;
    });
    setError("");
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError("");

    const validationErrors = validateRegistration(formData);
    setFieldErrors(validationErrors);
    if (Object.keys(validationErrors).length > 0) {
      return;
    }

    setLoading(true);

    try {
      const response = await register({
        username: formData.username.trim(),
        email: formData.email.trim().toLowerCase(),
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
      const status = err.response?.status;
      const backendError = err.response?.data?.error;

      if (backendError === "Username already exists") {
        setFieldErrors({ username: "Username is already in use." });
      } else if (backendError === "Email already exists") {
        setFieldErrors({ email: "Email is already in use." });
      } else if (!err.response) {
        setError("Cannot connect to the server. Check your connection and try again.");
      } else if (status === 429) {
        setError("Too many registration attempts. Please wait and try again.");
      } else if (status >= 500) {
        setError("Server could not create the account. Please try again.");
      } else if (status === 400) {
        setError("Please check your registration details and try again.");
      } else if (status === 409) {
        setError("That username or email is already in use.");
      } else {
        setError("Registration could not be completed. Please try again.");
      }
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

        <section
          className="register-panel"
          role="dialog"
          aria-modal="true"
          aria-labelledby="register-title"
        >
          <button
            className="register-panel__close"
            type="button"
            aria-label="Back to log in"
            onClick={closePanel}
          >
            <span />
            <span />
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
                  <button
                    type="button"
                    className="register-text-link"
                    onClick={() => navigate("/home")}
                  >
                    Continue to Home
                  </button>
                )}
              </div>
            )}

            <form className="register-form" onSubmit={handleSubmit} noValidate>
              <label
                className={`register-form-field${fieldErrors.username ? " register-form-field--invalid" : ""}`}
              >
                <span className="register-field-label">Username</span>
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
                  aria-invalid={Boolean(fieldErrors.username)}
                  aria-describedby={fieldErrors.username ? "username-error" : undefined}
                />
                {fieldErrors.username && (
                  <span id="username-error" className="register-field-error" role="alert">
                    {fieldErrors.username}
                  </span>
                )}
              </label>

              <label
                className={`register-form-field${fieldErrors.email ? " register-form-field--invalid" : ""}`}
              >
                <span className="register-field-label">Email</span>
                <input
                  type="email"
                  id="email"
                  name="email"
                  value={formData.email}
                  onChange={handleChange}
                  placeholder="your@email.com"
                  autoComplete="email"
                  required
                  aria-invalid={Boolean(fieldErrors.email)}
                  aria-describedby={fieldErrors.email ? "email-error" : undefined}
                />
                {fieldErrors.email && (
                  <span id="email-error" className="register-field-error" role="alert">
                    {fieldErrors.email}
                  </span>
                )}
              </label>

              <label
                className={`register-form-field${fieldErrors.password ? " register-form-field--invalid" : ""}`}
              >
                <span className="register-field-label">Password</span>
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
                  aria-invalid={Boolean(fieldErrors.password)}
                  aria-describedby={fieldErrors.password ? "password-error" : undefined}
                />
                {fieldErrors.password && (
                  <span id="password-error" className="register-field-error" role="alert">
                    {fieldErrors.password}
                  </span>
                )}
              </label>

              <label
                className={`register-form-field${fieldErrors.confirmPassword ? " register-form-field--invalid" : ""}`}
              >
                <span className="register-field-label">Confirm Password</span>
                <input
                  type="password"
                  id="confirmPassword"
                  name="confirmPassword"
                  value={formData.confirmPassword}
                  onChange={handleChange}
                  placeholder="Re-enter your password"
                  autoComplete="new-password"
                  required
                  aria-invalid={Boolean(fieldErrors.confirmPassword)}
                  aria-describedby={
                    fieldErrors.confirmPassword ? "confirm-password-error" : undefined
                  }
                />
                {fieldErrors.confirmPassword && (
                  <span id="confirm-password-error" className="register-field-error" role="alert">
                    {fieldErrors.confirmPassword}
                  </span>
                )}
              </label>

              <button type="submit" className="register-btn" disabled={loading} aria-busy={loading}>
                <span>{loading ? "Creating account..." : "Create account"}</span>
                {!loading && <span aria-hidden="true">↗</span>}
              </button>
            </form>

            <p className="register-login-footer">
              Already have an account?{" "}
              <button
                type="button"
                className="register-text-link"
                aria-label="Already have an account? Log in"
                onClick={() => navigate("/")}
              >
                Log in
              </button>
            </p>
          </div>
          <p className="register-privacy">Your Spotify credentials stay with Spotify.</p>
        </section>
      </div>
    </main>
  );
};

export default Register;
