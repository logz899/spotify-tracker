import React from "react";
import "./BrandMark.css";

const BrandMark = ({ compact = false, className = "" }) => (
  <div className={`brand-mark${compact ? " brand-mark--compact" : ""} ${className}`.trim()}>
    <span className="brand-mark__symbol" aria-hidden="true">
      <svg viewBox="0 0 48 48" xmlns="http://www.w3.org/2000/svg">
        <path d="M6 34c8-1 11-11 19-13s10-9 17-10" />
        <path d="M7 40c10-1 14-9 22-11s9-7 13-8" />
        <circle cx="25" cy="21" r="3" />
        <circle cx="29" cy="29" r="3" />
      </svg>
    </span>
    <span className="brand-mark__word">Spotify Tracker</span>
  </div>
);

export default BrandMark;
