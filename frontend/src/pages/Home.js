import React, { useState, useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { getRecentSongs, getAllSongs, getSpotifyAuthURL, logout } from "../services/api";
import { clearSession, getToken, getUser } from "../services/authSession";
import TimePeriodFilter from "../Components/TimePeriodFilter";
import "./Home.css";

/* ============================================
   Inline SVG Icons (no extra dependency)
   ============================================ */

const MusicNoteIcon = () => (
  <svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
    <path d="M12 3v10.55c-.59-.34-1.27-.55-2-.55-2.21 0-4 1.79-4 4s1.79 4 4 4 4-1.79 4-4V7h4V3h-6z" />
  </svg>
);

const SpotifyMark = () => (
  <svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
    <path d="M12 0C5.4 0 0 5.4 0 12s5.4 12 12 12 12-5.4 12-12S18.66 0 12 0zm5.521 17.34c-.24.359-.66.48-1.021.24-2.82-1.74-6.36-2.101-10.561-1.141-.418.122-.779-.179-.899-.539-.12-.421.18-.78.54-.9 4.56-1.021 8.52-.6 11.64 1.32.42.18.479.659.301 1.02zm1.44-3.3c-.301.42-.841.6-1.262.3-3.239-1.98-8.159-2.58-11.939-1.38-.479.12-1.02-.12-1.14-.6-.12-.48.12-1.021.6-1.141C9.6 9.9 15 10.561 18.72 12.84c.361.181.54.78.241 1.2zm.12-3.36C15.24 8.4 8.82 8.16 5.16 9.301c-.6.179-1.2-.181-1.38-.721-.18-.601.18-1.2.72-1.381 4.26-1.26 11.28-1.02 15.721 1.621.539.3.719 1.02.419 1.56-.299.421-1.02.599-1.559.3z" />
  </svg>
);

const PlaceholderNoteIcon = () => (
  <svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
    <path d="M12 3v10.55c-.59-.34-1.27-.55-2-.55-2.21 0-4 1.79-4 4s1.79 4 4 4 4-1.79 4-4V7h4V3h-6z" />
  </svg>
);

/* ============================================
   Thumbnail helper (image with fallback)
   ============================================ */

const Thumbnail = ({ src, alt }) => {
  const [errored, setErrored] = useState(false);

  if (!src || errored) {
    return (
      <div className="track-thumb-placeholder" aria-hidden="true">
        <PlaceholderNoteIcon />
      </div>
    );
  }

  return (
    <img
      className="track-thumb"
      src={src}
      alt={alt}
      loading="lazy"
      onError={() => setErrored(true)}
    />
  );
};

/* ============================================
   Track row component
   ============================================ */

const TrackRow = ({ rank, imageUrl, primaryText, secondaryText, playCount }) => {
  const isTop3 = rank <= 3;

  return (
    <div className={`track-row${isTop3 ? " track-row--top3" : ""}`}>
      <span className={`track-rank track-rank--${rank}`} aria-label={`Rank ${rank}`}>
        {rank}
      </span>

      <Thumbnail src={imageUrl} alt={primaryText} />

      <div className="track-info">
        <span className="track-name" title={primaryText}>
          {primaryText}
        </span>
        {secondaryText && (
          <span className="track-sub" title={secondaryText}>
            {secondaryText}
          </span>
        )}
      </div>

      <div className="track-plays">
        <span className="track-plays-count">{playCount}</span>
        <span className="track-plays-label">{playCount === 1 ? "play" : "plays"}</span>
      </div>
    </div>
  );
};

/* ============================================
   Main Home component
   ============================================ */

const Home = () => {
  const [allSongs, setAllSongs] = useState([]); // Store all songs from backend
  const [songs, setSongs] = useState([]); // Currently displayed songs
  const [loading, setLoading] = useState(true);
  const [filterLoading, setFilterLoading] = useState(false);
  const [error, setError] = useState(null);
  const [notice, setNotice] = useState(null);
  const [user, setUser] = useState(null);
  const [successMessage, setSuccessMessage] = useState(null);
  const [viewMode, setViewMode] = useState("songs"); // 'songs', 'artists', or 'albums'
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  useEffect(() => {
    // Check if user is authenticated
    const token = getToken();

    if (!token) {
      navigate("/");
      return;
    }

    setUser(getUser());

    // Check if Spotify was just linked
    if (searchParams.get("spotify_linked") === "true") {
      setSuccessMessage("Spotify account linked successfully!");
      navigate("/home", { replace: true });
      setTimeout(() => setSuccessMessage(null), 5000);
    }

    fetchSongs();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [navigate, searchParams]);

  const fetchSongs = async () => {
    try {
      setLoading(true);
      setError(null);

      const data = await getAllSongs();

      if (data.notice) {
        setNotice(data.notice);
      }

      if (data.data && Array.isArray(data.data)) {
        setAllSongs(data.data);
        setSongs(data.data);
      } else if (data.message && Array.isArray(data.message)) {
        // Fallback for backward compatibility
        setAllSongs(data.message);
        setSongs(data.message);
      }
    } catch (err) {
      if (err.response?.status === 401) {
        clearSession();
        navigate("/");
      } else {
        setError("Failed to fetch songs. Please try again.");
      }
    } finally {
      setLoading(false);
    }
  };

  // Handle time period filter change — interface unchanged
  const handleFilterChange = (startDate, endDate) => {
    setFilterLoading(true);
    setTimeout(() => {
      if (!startDate || !endDate) {
        setSongs(allSongs);
      } else {
        const filtered = allSongs.filter((song) => {
          if (!song.listening_date) return false;

          const songDate = new Date(song.listening_date);
          const start = new Date(startDate);
          const end = new Date(endDate);

          start.setHours(0, 0, 0, 0);
          end.setHours(23, 59, 59, 999);

          return songDate >= start && songDate <= end;
        });
        setSongs(filtered);
      }
      setFilterLoading(false);
    }, 300);
  };

  const handleRefresh = async () => {
    try {
      setLoading(true);
      setError(null);

      await getRecentSongs();
      await fetchSongs();
    } catch (err) {
      setError("Failed to refresh songs. Please try again.");
      setLoading(false);
    }
  };

  const handleLogout = () => {
    logout();
  };

  const handleSpotifyConnect = async () => {
    try {
      const authURL = await getSpotifyAuthURL();
      if (authURL) {
        window.location.href = authURL;
      } else {
        setError("Failed to get Spotify authorization URL");
      }
    } catch {
      setError("Failed to connect to Spotify. Please try again.");
    }
  };

  // Compute artist rankings
  const getArtistRankings = () => {
    const artistMap = {};

    songs.forEach((song) => {
      const artistName = song.author_name || "Unknown Artist";
      if (!artistMap[artistName]) {
        artistMap[artistName] = {
          name: artistName,
          playCount: 0,
          songCount: 0,
          imageUrl: song.image_url,
        };
      }
      artistMap[artistName].playCount += song.song_count || 1;
      artistMap[artistName].songCount += 1;
    });

    return Object.values(artistMap).sort((a, b) => b.playCount - a.playCount);
  };

  // Compute album rankings
  const getAlbumRankings = () => {
    const albumMap = {};

    songs.forEach((song) => {
      const albumName = song.album_name || "Unknown Album";
      const artistName = song.author_name || "Unknown Artist";
      const key = `${albumName}|${artistName}`;

      if (!albumMap[key]) {
        albumMap[key] = {
          name: albumName,
          artist: artistName,
          playCount: 0,
          songCount: 0,
          imageUrl: song.image_url,
        };
      }
      albumMap[key].playCount += song.song_count || 1;
      albumMap[key].songCount += 1;
    });

    return Object.values(albumMap).sort((a, b) => b.playCount - a.playCount);
  };

  /* ---- Derived data for tabs ---- */
  const artistRankings = getArtistRankings();
  const albumRankings = getAlbumRankings();
  const tabCounts = {
    songs: songs.length,
    artists: artistRankings.length,
    albums: albumRankings.length,
  };

  /* ---- Username display ---- */
  const displayName = user?.username || user?.name || "there";

  return (
    <div className="home-page">
      {/* ---- Sticky header ---- */}
      <header className="home-header">
        <div className="home-header-left">
          <div className="home-logo-mark" aria-hidden="true">
            <MusicNoteIcon />
          </div>
          <span className="home-title">Spotify Tracker</span>
        </div>

        <div className="home-header-right">
          {user && (
            <span className="home-greeting" aria-label={`Logged in as ${displayName}`}>
              Hey, {displayName}
            </span>
          )}

          <button
            className="hdr-btn hdr-btn--refresh"
            onClick={handleRefresh}
            disabled={loading}
            aria-label="Refresh songs from Spotify"
            title="Sync latest plays from Spotify"
          >
            <span className="hdr-refresh-icon" aria-hidden="true">
              &#8635;
            </span>
            <span className="hdr-btn-label">Sync</span>
          </button>

          {songs.length === 0 && !loading && (
            <button
              className="hdr-btn hdr-btn--connect"
              onClick={handleSpotifyConnect}
              aria-label="Connect your Spotify account"
            >
              <span style={{ width: 14, height: 14, display: "inline-flex", alignItems: "center" }}>
                <SpotifyMark />
              </span>
              <span className="hdr-btn-label">Connect</span>
            </button>
          )}

          <button className="hdr-btn hdr-btn--logout" onClick={handleLogout} aria-label="Log out">
            <span className="hdr-btn-label">Log out</span>
          </button>
        </div>
      </header>

      {/* ---- Main content ---- */}
      <main className="home-body">
        {/* Success toast */}
        {successMessage && (
          <div className="home-toast home-toast--success" role="status" aria-live="polite">
            <span className="home-toast-dot" aria-hidden="true" />
            {successMessage}
          </div>
        )}

        {/* Error toast */}
        {error && (
          <div className="home-toast home-toast--error" role="alert">
            <span className="home-toast-dot" aria-hidden="true" />
            {error}
            <button className="home-toast-retry" onClick={handleRefresh}>
              Retry
            </button>
          </div>
        )}

        {/* ---- Loading state ---- */}
        {loading && (
          <div className="home-loading" aria-live="polite" aria-busy="true">
            <div className="home-spinner" aria-hidden="true" />
            <span>Loading your listening history...</span>
          </div>
        )}

        {/* ---- Connect Spotify prompt (backend notice + no songs) ---- */}
        {!loading && notice && songs.length === 0 && (
          <div className="home-connect-prompt">
            <div className="home-connect-icon" aria-hidden="true">
              <SpotifyMark />
            </div>
            <h2>Connect your Spotify</h2>
            <p>
              Link your Spotify account to start tracking your listening history and see your top
              songs, artists, and albums.
            </p>
            <button className="home-connect-btn" onClick={handleSpotifyConnect}>
              <span style={{ width: 18, height: 18, display: "inline-flex", alignItems: "center" }}>
                <SpotifyMark />
              </span>
              Connect with Spotify
            </button>
          </div>
        )}

        {/* ---- Main data view ---- */}
        {!loading && !error && allSongs.length > 0 && (
          <>
            {/* Time Period Filter */}
            <TimePeriodFilter
              onFilterChange={handleFilterChange}
              totalSongs={allSongs.length}
              filteredSongs={songs.length}
            />

            {/* Tab bar */}
            <nav className="home-tabs" role="tablist" aria-label="View mode">
              <button
                role="tab"
                aria-selected={viewMode === "songs"}
                className={`home-tab${viewMode === "songs" ? " home-tab--active" : ""}`}
                onClick={() => setViewMode("songs")}
              >
                Songs
                <span className="home-tab-badge" aria-hidden="true">
                  {tabCounts.songs}
                </span>
              </button>

              <button
                role="tab"
                aria-selected={viewMode === "artists"}
                className={`home-tab${viewMode === "artists" ? " home-tab--active" : ""}`}
                onClick={() => setViewMode("artists")}
              >
                Artists
                <span className="home-tab-badge" aria-hidden="true">
                  {tabCounts.artists}
                </span>
              </button>

              <button
                role="tab"
                aria-selected={viewMode === "albums"}
                className={`home-tab${viewMode === "albums" ? " home-tab--active" : ""}`}
                onClick={() => setViewMode("albums")}
              >
                Albums
                <span className="home-tab-badge" aria-hidden="true">
                  {tabCounts.albums}
                </span>
              </button>
            </nav>

            {/* ---- Content panel ---- */}
            {filterLoading ? (
              <div className="home-filter-loading" aria-live="polite" aria-busy="true">
                <div className="home-spinner" aria-hidden="true" />
                <span>Filtering...</span>
              </div>
            ) : songs.length > 0 ? (
              <>
                {/* Songs tab */}
                {viewMode === "songs" && (
                  <section aria-label="Top songs">
                    <div className="home-section-header">
                      <h2 className="home-section-title">Top Songs</h2>
                      <span className="home-section-count">{songs.length} tracks</span>
                    </div>
                    <div className="track-list" role="list" aria-label="Song rankings">
                      {songs.map((song, index) => (
                        <div role="listitem" key={`${song.song_name}-${index}`}>
                          <TrackRow
                            rank={index + 1}
                            imageUrl={song.image_url}
                            primaryText={song.song_name}
                            secondaryText={[song.author_name, song.album_name]
                              .filter(Boolean)
                              .join(" · ")}
                            playCount={song.song_count || 1}
                          />
                        </div>
                      ))}
                    </div>
                  </section>
                )}

                {/* Artists tab */}
                {viewMode === "artists" && (
                  <section aria-label="Top artists">
                    <div className="home-section-header">
                      <h2 className="home-section-title">Top Artists</h2>
                      <span className="home-section-count">{artistRankings.length} artists</span>
                    </div>
                    <div className="track-list" role="list" aria-label="Artist rankings">
                      {artistRankings.map((artist, index) => (
                        <div role="listitem" key={`${artist.name}-${index}`}>
                          <TrackRow
                            rank={index + 1}
                            imageUrl={artist.imageUrl}
                            primaryText={artist.name}
                            secondaryText={`${artist.songCount} ${artist.songCount === 1 ? "song" : "songs"}`}
                            playCount={artist.playCount}
                          />
                        </div>
                      ))}
                    </div>
                  </section>
                )}

                {/* Albums tab */}
                {viewMode === "albums" && (
                  <section aria-label="Top albums">
                    <div className="home-section-header">
                      <h2 className="home-section-title">Top Albums</h2>
                      <span className="home-section-count">{albumRankings.length} albums</span>
                    </div>
                    <div className="track-list" role="list" aria-label="Album rankings">
                      {albumRankings.map((album, index) => (
                        <div role="listitem" key={`${album.name}-${index}`}>
                          <TrackRow
                            rank={index + 1}
                            imageUrl={album.imageUrl}
                            primaryText={album.name}
                            secondaryText={album.artist}
                            playCount={album.playCount}
                          />
                        </div>
                      ))}
                    </div>
                  </section>
                )}
              </>
            ) : (
              /* Empty state for period filter */
              <div className="home-empty">
                <h3>No {viewMode} found for this period</h3>
                <p>Try selecting a different time range or sync your latest plays.</p>
              </div>
            )}
          </>
        )}

        {/* ---- Absolute empty state (no data at all, no notice) ---- */}
        {!loading && !error && !notice && allSongs.length === 0 && (
          <div className="home-no-data">
            <span>No songs found. Start playing music on Spotify and sync!</span>
          </div>
        )}
      </main>
    </div>
  );
};

export default Home;
