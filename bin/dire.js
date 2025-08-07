#!/usr/bin/env node

import { spawn } from "child_process";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

// Get __dirname equivalent in ES modules
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Determine binary name based on platform
const binaryName = process.platform === "win32" ? "dire.exe" : "dire";
const binaryPath = path.join(__dirname, "..", "bin", binaryName);

// Check if binary exists
if (!fs.existsSync(binaryPath)) {
  console.error("Error: dire binary not found.");
  console.error("Try running: npm install dire --force");
  process.exit(1);
}

// Spawn the binary with user's arguments
const child = spawn(binaryPath, process.argv.slice(2), { stdio: "inherit" });
child.on("exit", (code) => process.exit(code));
