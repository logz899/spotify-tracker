import React from "react";
import { HashRouter as Router, Routes, Route, Navigate } from "react-router-dom";
import Login from "./pages/Login";
import Register from "./pages/Register";
import Home from "./pages/Home";
import { getToken } from "./services/authSession";
import "./App.css";

const PublicOnlyRoute = ({ children }) => (getToken() ? <Navigate to="/home" replace /> : children);

const ProtectedRoute = ({ children }) => (getToken() ? children : <Navigate to="/" replace />);

const DefaultRoute = () => <Navigate to={getToken() ? "/home" : "/"} replace />;

function App() {
  return (
    <Router>
      <div className="App">
        <Routes>
          <Route
            path="/"
            element={
              <PublicOnlyRoute>
                <Login />
              </PublicOnlyRoute>
            }
          />
          <Route
            path="/register"
            element={
              <PublicOnlyRoute>
                <Register />
              </PublicOnlyRoute>
            }
          />
          <Route
            path="/home"
            element={
              <ProtectedRoute>
                <Home />
              </ProtectedRoute>
            }
          />
          <Route path="*" element={<DefaultRoute />} />
        </Routes>
      </div>
    </Router>
  );
}

export default App;
