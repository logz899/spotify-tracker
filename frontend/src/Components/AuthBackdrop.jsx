import React from "react";
import BrandMark from "./BrandMark";
import SignalField from "./SignalField";
import "./AuthBackdrop.css";

const Arrow = () => (
  <svg viewBox="0 0 18 18" aria-hidden="true"><path d="M3 9h11M10 5l4 4-4 4" /></svg>
);

const AuthBackdrop = ({ inactive = false, onLogin, onRegister }) => (
  <div className="auth-backdrop" aria-hidden={inactive || undefined}>
    <SignalField />

    <header className="auth-nav">
      <BrandMark className="auth-nav__brand" />
      <button className="auth-nav__action" type="button" onClick={onRegister}>
        <span>Create account</span>
        <Arrow />
      </button>
    </header>

    <section className="auth-hero" aria-labelledby="auth-hero-title">
      <div className="auth-hero__content">
        <p className="auth-hero__eyebrow">Listening trends across time</p>
        <h1 id="auth-hero-title">Music,<br />in order.</h1>
        <p className="auth-hero__tagline">See what changed. Know what stayed.</p>
        <button className="auth-hero__action" type="button" onClick={onLogin}>
          <span>Log in</span>
          <Arrow />
        </button>
      </div>
    </section>
  </div>
);

export default AuthBackdrop;
