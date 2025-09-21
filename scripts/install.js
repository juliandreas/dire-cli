#!/usr/bin/env node

import { createWriteStream } from "fs";
import { mkdir, chmod, unlink } from "fs/promises";
import { fileURLToPath } from "url";
import { createRequire } from "module";
import { exec } from "child_process";
import { promisify } from "util";
import path from "path";
import https from "https";

// Get __dirname equivalent and package.json
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const require = createRequire(import.meta.url);
const packageJson = require("../package.json");
const execAsync = promisify(exec);

// Platform mapping
const platformMap = {
  "darwin-x64": "dire-darwin-amd64.zip",
  "darwin-arm64": "dire-darwin-arm64.zip",
  "linux-x64": "dire-linux-amd64.zip",
  "linux-arm64": "dire-linux-arm64.zip",
  "win32-x64": "dire-windows-amd64.zip",
  "win32-arm64": "dire-windows-arm64.zip",
};

async function install() {
  try {
    // Detect platform
    const platformKey = `${process.platform}-${process.arch}`;
    const zipName = platformMap[platformKey];

    if (!zipName) {
      throw new Error(`Unsupported platform: ${platformKey}`);
    }

    // Build download URL
    const version = packageJson.version;
    const downloadUrl = `https://github.com/juliandreas/dire-cli/releases/download/v${version}/${zipName}`;

    console.log(`Downloading Dire for ${platformKey}...`);

    // Create bin directory
    const binDir = path.join(__dirname, "..", "bin");
    await mkdir(binDir, { recursive: true });

    // Download and extract
    const zipPath = path.join(binDir, zipName);
    await downloadFile(downloadUrl, zipPath);

    console.log("Setting up Dire...");
    await extractZip(zipPath, binDir);
    await unlink(zipPath); // Clean up zip file

    // Make binary executable on Unix
    if (process.platform !== "win32") {
      const binaryPath = path.join(binDir, "dire-cli");
      try {
        await chmod(binaryPath, 0o755);
      } catch (chmodError) {
        console.warn("Failed to make binary executable:", chmodError.message);
        console.warn("You may need to run: chmod +x", binaryPath);
      }
    }

    console.log("Ready to use!");

    // Force exit to prevent hanging
    process.exit(0);
  } catch (error) {
    console.warn("Failed to download dire binary:", error.message);
    console.warn(
      "You can manually download from: https://github.com/juliandreas/dire-cli/releases"
    );
    console.warn("Package installed with warnings - binary setup incomplete");
    process.exit(0);
  }
}

async function downloadFile(url, dest) {
  return new Promise((resolve, reject) => {
    https
      .get(url, (response) => {
        // Check status codes first, before creating file
        if (response.statusCode === 302 || response.statusCode === 301) {
          // Follow redirect
          return downloadFile(response.headers.location, dest)
            .then(resolve)
            .catch(reject);
        }

        if (response.statusCode !== 200) {
          reject(new Error(`Download failed: HTTP ${response.statusCode}`));
          return;
        }

        // Only create file stream if we have a successful response
        const file = createWriteStream(dest);
        response.pipe(file);

        file.on("finish", () => {
          file.close((err) => {
            if (err) reject(err);
            else resolve();
          });
        });

        file.on("error", (err) => {
          file.close();
          reject(err);
        });
      })
      .on("error", (err) => {
        reject(err);
      });
  });
}

async function extractZip(zipPath, extractDir) {
  if (process.platform === "win32") {
    // Windows: Use built-in PowerShell
    await execAsync(
      `powershell -command "Expand-Archive -Path '${zipPath}' -DestinationPath '${extractDir}' -Force"`
    );
  } else {
    // Unix: Use unzip for better compatibility
    await execAsync(`unzip -o "${zipPath}" -d "${extractDir}"`);
  }
}

install();
