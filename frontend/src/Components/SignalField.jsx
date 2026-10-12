import React from "react";
import "./SignalField.css";

const SignalField = () => (
  <div className="signal-field" aria-hidden="true">
    <div className="signal-field__glow" />
    <svg
      className="signal-field__lines"
      viewBox="0 0 1600 900"
      preserveAspectRatio="xMidYMid slice"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path className="signal-line signal-line--one" d="M-120 714C210 520 390 641 650 490S1080 161 1730 304" />
      <path className="signal-line signal-line--two" d="M-180 790C185 591 435 716 710 550S1170 258 1710 382" />
      <path className="signal-line signal-line--three" d="M-90 848C310 683 513 765 811 617S1275 386 1720 462" />
      <g className="signal-pulse signal-pulse--one"><circle cx="649" cy="491" r="4" /><circle className="signal-pulse__ring" cx="649" cy="491" r="12" /></g>
      <g className="signal-pulse signal-pulse--two"><circle cx="1167" cy="260" r="4" /><circle className="signal-pulse__ring" cx="1167" cy="260" r="12" /></g>
      <g className="signal-pulse signal-pulse--three"><circle cx="812" cy="617" r="4" /><circle className="signal-pulse__ring" cx="812" cy="617" r="12" /></g>
    </svg>
    <div className="signal-field__grain" />
  </div>
);

export default SignalField;
