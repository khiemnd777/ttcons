/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'ui-sans-serif', 'system-ui', 'sans-serif'],
      },
      colors: {
        ink: '#172033',
        line: '#d9e2ec',
        surface: '#f7f9fc',
      },
      boxShadow: {
        panel: '0 10px 24px rgba(23, 32, 51, 0.08)',
      },
    },
  },
  plugins: [],
};
