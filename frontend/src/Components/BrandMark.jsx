import React from "react";
import "./BrandMark.css";

const BrandMark = ({ compact = false, className = "" }) => (
  <div className={`brand-mark${compact ? " brand-mark--compact" : ""} ${className}`.trim()}>
    <span className="brand-mark__symbol" aria-hidden="true">
      <svg viewBox="0 0 34 28" xmlns="http://www.w3.org/2000/svg">
        <path d="M2.5 21.5c4.6 0 5.7-5.5 10.1-5.5s5.4-7.5 10-7.5h7" />
        <path
          className="brand-mark__echo"
          d="M5 26c4.5-.2 6-4.8 10.3-4.8 3.8 0 5.5-4.5 9.4-4.5H30"
        />
        <circle cx="29.5" cy="8.5" r="2.2" />
      </svg>
    </span>
    <span className="brand-mark__word">Spotify Tracker</span>
  </div>
);

export default BrandMark;
