import { defineConfig } from "vite";

export default defineConfig({
  clearScreen: false,
  build: {
    outDir: "dist",
    emptyOutDir: true,
    sourcemap: false,
  },
});
