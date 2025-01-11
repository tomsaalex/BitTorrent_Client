/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}"],
  theme: {
    extend: {
      secondary: '#648381',
      neutral: '#575761', 
      success: '#AAF683',
      highlight: '#DBC7BE',
      primary: '#9BC53D'
    },
  },
  plugins: [],
}

