#!/usr/bin/env node
"use strict";

const { spawn } = require("child_process");
const path = require("path");
const fs = require("fs");

const binDir = __dirname;
const isWindows = process.platform === "win32";
const binaryName = isWindows ? "trvl.exe" : "trvl";
const binaryPath = path.join(binDir, binaryName);

if (!fs.existsSync(binaryPath)) {
  console.error("trvl binary not found. Run the postinstall script:");
  console.error("  node " + path.join(binDir, "install.js"));
  process.exit(1);
}

const child = spawn(binaryPath, ["mcp"], {
  stdio: "inherit",
});

child.on("error", (err) => {
  console.error(`Failed to start trvl: ${err.message}`);
  process.exit(1);
});

child.on("exit", (code) => {
  process.exit(code ?? 0);
});
