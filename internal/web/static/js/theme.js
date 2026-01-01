// Theme Management with System Preference Detection
(function() {
  'use strict';

  const STORAGE_KEY = 'theme';
  const DARK_CLASS = 'dark';

  // Get initial theme from localStorage or system preference
  function getInitialTheme() {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === 'dark' || stored === 'light') {
      return stored;
    }
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }

  // Apply theme to document
  function applyTheme(theme) {
    if (theme === 'dark') {
      document.documentElement.classList.add(DARK_CLASS);
    } else {
      document.documentElement.classList.remove(DARK_CLASS);
    }
  }

  // Initialize theme immediately to prevent flash
  applyTheme(getInitialTheme());

  // Listen for system preference changes
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
    // Only update if user hasn't explicitly set a preference
    if (!localStorage.getItem(STORAGE_KEY)) {
      applyTheme(e.matches ? 'dark' : 'light');
    }
  });

  // Expose toggle function globally
  window.toggleTheme = function() {
    const isDark = document.documentElement.classList.contains(DARK_CLASS);
    const newTheme = isDark ? 'light' : 'dark';
    localStorage.setItem(STORAGE_KEY, newTheme);
    applyTheme(newTheme);
  };

  // Expose function to get current theme
  window.getCurrentTheme = function() {
    return document.documentElement.classList.contains(DARK_CLASS) ? 'dark' : 'light';
  };
})();
