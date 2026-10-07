import React, { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { linkPendingSpotify, register } from "../services/api";
import { setSession } from "../services/authSession";
import * as pendingSpotify from "../services/pendingSpotify";
import "./Register.css";

/* Music note icon for brand logo — same mark used in Login */
const MusicNoteIcon = () => (
  <svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
    <path d="M12 3v10.55c-.59-.34-1.27-.55-2-.55-2.21 0-4 1.79-4 4s1.79 4 4 4 4-1.79 4-4V7h4V3h-6z" />
  </svg>
);

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
    <div className="register-page">
      <div className="register-card">
        {/* Brand header */}
        <div className="register-brand">
          <div className="register-brand-logo" aria-hidden="true">
            <MusicNoteIcon />
          </div>
          <h1 className="register-brand-title">Create account</h1>
          <p className="register-brand-subtitle">Join Spotify Tracker today</p>
        </div>

        <div className="register-body">
          {/* Spotify pending notice */}
          {spotifyPending && (
            <div className="register-notice register-notice--success" role="status">
              <span className="register-notice-dot" aria-hidden="true" />
              Spotify connected. Your account will be linked after registration.
            </div>
          )}

          {/* Error notice */}
          {error && (
            <div className="register-notice register-notice--error" role="alert">
              <span className="register-notice-dot" aria-hidden="true" />
              {error}
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

          {/* Registration form */}
          <form className="register-form" onSubmit={handleSubmit} noValidate>
            <div className="register-form-field">
              <label htmlFor="username">Username</label>
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
            </div>

            <div className="register-form-field">
              <label htmlFor="email">Email</label>
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
            </div>

            <div className="register-form-field">
              <label htmlFor="password">Password</label>
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
            </div>

            <div className="register-form-field">
              <label htmlFor="confirmPassword">Confirm Password</label>
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
            </div>

            <div className="register-submit-wrapper">
              <button type="submit" className="register-btn" disabled={loading} aria-busy={loading}>
                {loading ? (
                  <>
                    <span className="register-btn-spinner" aria-hidden="true" />
                    Creating account...
                  </>
                ) : (
                  "Create account"
                )}
              </button>
            </div>
          </form>

          {/* Login link */}
          <p className="register-footer">
            Already have an account?{" "}
            <button type="button" className="register-text-link" onClick={() => navigate("/")}>
              Log in
            </button>
          </p>
        </div>
      </div>
    </div>
  );
};

export default Register;
