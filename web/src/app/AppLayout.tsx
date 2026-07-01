import React, { useCallback, useEffect, useState } from "react";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { Menu, Moon, Sun, X } from "lucide-react";
import { navItems } from "./nav";
import { useAPIState } from "../lib/queries";

type ThemeMode = "light" | "dark";

function currentTheme(): ThemeMode {
  return document.documentElement.getAttribute("data-theme") === "dark" ? "dark" : "light";
}

export function AppLayout() {
  const [theme, setTheme] = useState<ThemeMode>(currentTheme);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const location = useLocation();
  const apiState = useAPIState();

  const toggleTheme = useCallback(() => {
    setTheme((mode) => {
      const next = mode === "dark" ? "light" : "dark";
      document.documentElement.setAttribute("data-theme", next);
      try {
        window.localStorage.setItem("librarry-theme", next);
      } catch {
        // Private-mode storage failures only lose the preference.
      }
      return next;
    });
  }, []);

  // Close the mobile drawer on navigation and on Escape.
  useEffect(() => {
    setDrawerOpen(false);
  }, [location.pathname]);
  useEffect(() => {
    if (!drawerOpen) return;
    const onKey = (event: KeyboardEvent) => event.key === "Escape" && setDrawerOpen(false);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [drawerOpen]);

  const active = navItems.find((item) => location.pathname.startsWith(item.path));

  const nav = (
    <nav className="side-nav" aria-label="Primary">
      {navItems.map((item) => {
        const Icon = item.icon;
        return (
          <NavLink
            key={item.id}
            to={item.path}
            className={({ isActive }) => `side-nav-link${isActive ? " active" : ""}`}
            title={item.label}
          >
            <Icon size={17} aria-hidden />
            <span className="side-nav-label">{item.label}</span>
          </NavLink>
        );
      })}
    </nav>
  );

  const statusDot = (
    <span className={`conn-status conn-${apiState}`} title={`Backend: ${apiState}`}>
      <span className="conn-dot" aria-hidden />
      <span className="conn-label">{apiState === "live" ? "Connected" : apiState === "demo" ? "Demo data" : "Offline"}</span>
    </span>
  );

  return (
    <div className="app-shell">
      {/* Mobile top bar: brand + hamburger. Hidden on wider screens. */}
      <header className="mobile-bar">
        <button type="button" className="icon-btn" aria-label="Open navigation" onClick={() => setDrawerOpen(true)}>
          <Menu size={18} />
        </button>
        <span className="mobile-bar-title">{active?.label ?? "Librarry"}</span>
        <button type="button" className="icon-btn" aria-label="Toggle theme" onClick={toggleTheme}>
          {theme === "dark" ? <Sun size={16} /> : <Moon size={16} />}
        </button>
      </header>

      <aside className={`sidebar${drawerOpen ? " open" : ""}`}>
        <div className="sidebar-brand">
          <span className="brand-mark" aria-hidden>
            L
          </span>
          <span className="brand-text">
            <strong>Librarry</strong>
            <small>Readarr replacement</small>
          </span>
          <button type="button" className="icon-btn sidebar-close" aria-label="Close navigation" onClick={() => setDrawerOpen(false)}>
            <X size={16} />
          </button>
        </div>
        {nav}
        <div className="sidebar-foot">
          {statusDot}
          <button type="button" className="theme-toggle" onClick={toggleTheme}>
            {theme === "dark" ? <Sun size={14} aria-hidden /> : <Moon size={14} aria-hidden />}
            <span>{theme === "dark" ? "Light mode" : "Dark mode"}</span>
          </button>
        </div>
      </aside>
      {drawerOpen ? <div className="sidebar-scrim" onClick={() => setDrawerOpen(false)} aria-hidden /> : null}

      <main className="workspace">
        <Outlet />
      </main>
    </div>
  );
}
