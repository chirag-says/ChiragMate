/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./internal/**/*.templ",
    "./internal/**/*.go",
    "./assets/**/*.js"
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
      },
    },
  },
  plugins: [],
}
