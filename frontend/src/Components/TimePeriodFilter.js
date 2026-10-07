import React, { useState } from 'react';
import './TimePeriodFilter.css';

/**
 * TimePeriodFilter
 *
 * Pill-style preset date-range selector. Calls onFilterChange(startDate, endDate)
 * with Date objects (or null, null for "All Time").
 *
 * Props:
 *   onFilterChange(start: Date|null, end: Date|null) — required
 *   totalSongs: number — total songs in the unfiltered set
 *   filteredSongs: number — songs currently shown after filtering
 */

const PRESETS = [
  { id: 'all', label: 'All Time' },
  { id: 'last_7', label: 'Last 7 Days' },
  { id: 'this_month', label: 'This Month' },
  { id: 'last_month', label: 'Last Month' },
];

const TimePeriodFilter = ({ onFilterChange, totalSongs, filteredSongs }) => {
  const [activePreset, setActivePreset] = useState('all');

  const handlePreset = (presetId) => {
    setActivePreset(presetId);

    const today = new Date();
    let start = null;
    let end = null;

    switch (presetId) {
      case 'all':
        // null, null means no filter — show everything
        break;

      case 'last_7':
        start = new Date(today);
        start.setDate(today.getDate() - 7);
        end = today;
        break;

      case 'this_month':
        start = new Date(today.getFullYear(), today.getMonth(), 1);
        end = new Date(today.getFullYear(), today.getMonth() + 1, 0);
        break;

      case 'last_month':
        start = new Date(today.getFullYear(), today.getMonth() - 1, 1);
        end = new Date(today.getFullYear(), today.getMonth(), 0);
        break;

      default:
        break;
    }

    onFilterChange(start, end);
  };

  const isFiltered = activePreset !== 'all';

  return (
    <div className="tpf-wrapper">

      {/* Pill buttons */}
      <nav className="tpf-pills" aria-label="Time period filter">
        {PRESETS.map((preset) => (
          <button
            key={preset.id}
            className={`tpf-pill${activePreset === preset.id ? ' tpf-pill--active' : ''}`}
            onClick={() => handlePreset(preset.id)}
            aria-pressed={activePreset === preset.id}
            type="button"
          >
            {preset.label}
          </button>
        ))}
      </nav>

      {/* Filtered count — only visible when a filter is active */}
      {isFiltered && (
        <span className="tpf-count" aria-live="polite">
          <strong>{filteredSongs}</strong> of {totalSongs}
        </span>
      )}

    </div>
  );
};

export default TimePeriodFilter;
