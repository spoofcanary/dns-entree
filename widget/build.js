const esbuild = require("esbuild");
const { execFileSync } = require("child_process");
const fs = require("fs");
const path = require("path");
const { gzipSync } = require("zlib");

const isServe = process.argv.includes("--serve");

const shared = {
  entryPoints: [path.resolve(__dirname, "src/index.ts")],
  bundle: true,
  minify: !isServe,
  sourcemap: isServe,
  target: "es2020",
  logLevel: "info",
};

async function build() {
  // IIFE build - exposes DnsEntree global
  await esbuild.build({
    ...shared,
    outfile: path.resolve(__dirname, "dist/widget.js"),
    format: "iife",
    globalName: "DnsEntree",
    footer: {
      // Flatten the default export so DnsEntree.open() works directly
      js: "if(typeof DnsEntree!=='undefined'&&DnsEntree.default){Object.assign(DnsEntree,DnsEntree.default);}",
    },
  });

  // ESM build
  await esbuild.build({
    ...shared,
    outfile: path.resolve(__dirname, "dist/widget.mjs"),
    format: "esm",
  });

  // Generate TypeScript declarations
  console.log("Generating type declarations...");
  execFileSync("npx", ["tsc", "--emitDeclarationOnly", "--declaration", "--declarationDir", "dist"], {
    cwd: __dirname,
    stdio: "inherit",
  });

  // Copy index.d.ts to widget.d.ts for package.json "types" field
  const srcDts = path.resolve(__dirname, "dist/index.d.ts");
  const dstDts = path.resolve(__dirname, "dist/widget.d.ts");
  if (fs.existsSync(srcDts)) {
    fs.copyFileSync(srcDts, dstDts);
  }

  // Report bundle sizes and enforce gzip limit
  const jsPath = path.resolve(__dirname, "dist/widget.js");
  const mjsPath = path.resolve(__dirname, "dist/widget.mjs");
  const jsSize = fs.statSync(jsPath).size;
  const mjsSize = fs.statSync(mjsPath).size;
  const jsGzip = gzipSync(fs.readFileSync(jsPath)).length;
  const mjsGzip = gzipSync(fs.readFileSync(mjsPath)).length;
  console.log("  widget.js:  " + (jsSize / 1024).toFixed(1) + " KB (" + (jsGzip / 1024).toFixed(1) + " KB gzip)");
  console.log("  widget.mjs: " + (mjsSize / 1024).toFixed(1) + " KB (" + (mjsGzip / 1024).toFixed(1) + " KB gzip)");

  const GZIP_LIMIT = 20 * 1024;
  if (jsGzip > GZIP_LIMIT) {
    console.error("FAIL: widget.js exceeds 20KB gzip limit (" + jsGzip + " bytes)");
    process.exit(1);
  }

  console.log("Build complete.");
}

async function serve() {
  const ctx = await esbuild.context({
    ...shared,
    outfile: path.resolve(__dirname, "dist/widget.js"),
    format: "iife",
    globalName: "DnsEntree",
    footer: {
      js: "if(typeof DnsEntree!=='undefined'&&DnsEntree.default){Object.assign(DnsEntree,DnsEntree.default);}",
    },
    sourcemap: true,
    minify: false,
  });

  const { host, port } = await ctx.serve({
    servedir: path.resolve(__dirname),
    port: 8100,
  });

  console.log(`Dev server running at http://${host}:${port}/test.html`);
}

if (isServe) {
  serve().catch((e) => { console.error(e); process.exit(1); });
} else {
  build().catch((e) => { console.error(e); process.exit(1); });
}
