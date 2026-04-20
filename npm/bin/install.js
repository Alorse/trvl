#!/usr/bin/env node
"use strict";

const https = require("https");
const fs = require("fs");
const path = require("path");
const { execSync } = require("child_process");

const VERSION = require("../package.json").version;

function getPlatform() {
  const platform = process.platform;
  if (platform === "darwin") return "darwin";
  if (platform === "linux") return "linux";
  if (platform === "win32") return "windows";
  throw new Error(`Unsupported platform: ${platform}`);
}

function getArch() {
  const arch = process.arch;
  if (arch === "x64") return "amd64";
  if (arch === "arm64") return "arm64";
  throw new Error(`Unsupported architecture: ${arch}`);
}

function download(url) {
  return new Promise((resolve, reject) => {
    https.get(url, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        return download(res.headers.location).then(resolve).catch(reject);
      }
      if (res.statusCode !== 200) {
        return reject(new Error(`Download failed: HTTP ${res.statusCode} from ${url}`));
      }
      const chunks = [];
      res.on("data", (chunk) => chunks.push(chunk));
      res.on("end", () => resolve(Buffer.concat(chunks)));
      res.on("error", reject);
    }).on("error", reject);
  });
}

async function install() {
  const os = getPlatform();
  const arch = getArch();
  const filename = `trvl_${VERSION}_${os}_${arch}.tar.gz`;
  const url = `https://github.com/MikkoParkkola/trvl/releases/download/v${VERSION}/${filename}`;
  const binDir = path.join(__dirname);
  const binaryName = os === "windows" ? "trvl.exe" : "trvl";
  const binaryPath = path.join(binDir, binaryName);

  // Skip if binary already exists
  if (fs.existsSync(binaryPath)) {
    console.log(`trvl binary already exists at ${binaryPath}`);
    return;
  }

  console.log(`Downloading trvl v${VERSION} for ${os}/${arch}...`);
  console.log(`  ${url}`);

  let tarball;
  try {
    tarball = await download(url);
  } catch (err) {
    console.error(`\nFailed to download trvl binary:\n  ${err.message}\n`);
    console.error("You can manually download from:");
    console.error(`  https://github.com/MikkoParkkola/trvl/releases/tag/v${VERSION}\n`);
    process.exit(1);
  }

  // Write tarball to temp file and extract
  const tmpFile = path.join(binDir, filename);
  fs.writeFileSync(tmpFile, tarball);

  try {
    if (os === "windows") {
      // On Windows, use tar which is available since Windows 10
      execSync(`tar -xzf "${tmpFile}" -C "${binDir}" trvl.exe`, { stdio: "pipe" });
    } else {
      execSync(`tar -xzf "${tmpFile}" -C "${binDir}" trvl`, { stdio: "pipe" });
    }
  } catch (err) {
    console.error(`Failed to extract binary: ${err.message}`);
    process.exit(1);
  } finally {
    // Clean up tarball
    fs.unlinkSync(tmpFile);
  }

  // Make executable on Unix
  if (os !== "windows") {
    fs.chmodSync(binaryPath, 0o755);
  }

  console.log(`trvl v${VERSION} installed successfully.`);
}

install();
