import axios from "axios";
import { clearSession, getToken } from "./authSession";

// Production builds must set REACT_APP_API_URL; only local development falls back to localhost.
export const resolveApiBaseUrl = (env) => {
  if (env.REACT_APP_API_URL) {
    return env.REACT_APP_API_URL;
  }
  return env.NODE_ENV === "production" ? "" : "http://localhost:8000";
};

const API_BASE_URL =
  process.env.NODE_ENV === "production"
    ? process.env.REACT_APP_API_URL || ""
    : resolveApiBaseUrl(process.env);

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    "Content-Type": "application/json",
  },
});

// Add auth token to requests
api.interceptors.request.use((config) => {
  const token = getToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Handle 401 errors (logout)
export const handleUnauthorizedResponse = (error) => {
  if (error.response?.status === 401) {
    clearSession();
    window.location.hash = "#/";
  }
  return Promise.reject(error);
};

api.interceptors.response.use((response) => response, handleUnauthorizedResponse);

// Auth endpoints
export const register = async (userData) => {
  const response = await api.post("/api/v1/auth/register", userData);
  return response.data;
};

export const login = async (username, password) => {
  const response = await api.post("/api/v1/auth/login", { username, password });
  return response.data;
};

export const getCurrentUser = async () => {
  const response = await api.get("/api/v1/user/me");
  return response.data;
};

export const linkPendingSpotify = async (pendingKey) => {
  const response = await api.post("/api/v1/auth/link-spotify", { pending_key: pendingKey });
  return response.data;
};

// Health check
export const healthCheck = async () => {
  const response = await api.get("/api/v1/health");
  return response.data;
};

// Get recently played songs (protected)
export const getRecentSongs = async () => {
  const response = await api.get("/api/v1/songs/recent");
  return response.data;
};

// Get all songs from database (protected)
export const getAllSongs = async () => {
  const response = await api.get("/api/v1/songs/all");
  return response.data;
};

// Redirect to Spotify login (with optional auth)
export const loginWithSpotify = () => {
  window.location.href = `${API_BASE_URL}/api/v1/auth/spotify/login`;
};

export const getSpotifyAuthURL = async () => {
  const response = await api.get("/api/v1/auth/spotify/login", {
    headers: { Accept: "application/json" },
  });
  return response.data.auth_url;
};

// Logout
export const logout = () => {
  clearSession();
  window.location.hash = "#/";
};

export default api;
