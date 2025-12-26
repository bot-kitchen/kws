// tailwind.config.js
// ============================================
// THEME CUSTOMIZATION – Linear.app color system
// ============================================

const themeColors = {
  // Accent blue occasionally used for highlights
  accent: "#007AFF",
  // Linear primary violet
  primary: "#6E56CF",
  "primary-hover": "#5B41C1",
  // Light mode backgrounds (Linear is mostly dark-first)
  "background-light": "#F7F7F9",
  "background-dark": "#0E0F14",
  "surface-dark": "#15171C",
  "surface-highlight": "#1C1F26",
  "border-dark": "#2A2D36",
  "text-primary": "#EDEEF2",
  "text-secondary": "#8D8FA3",
};

module.exports = {
  content: ["./web/templates/**/*.html", "./web/static/js/**/*.js"],
  darkMode: "class",
  plugins: [
    require("@tailwindcss/forms"),
    require("@tailwindcss/container-queries"),
  ],
  theme: {
    extend: {
      colors: themeColors,
      fontFamily: {
        display: ["Nunito", "sans-serif"],
      },
    },
  },
};
