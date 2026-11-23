#!/usr/bin/env node
// Công cụ QA cho CC-08: tạo fixture, lập manifest SHA-256 và đối chiếu nguồn/đích.
// Dùng: node scripts/qa/fixtures.mjs <make|manifest|compare> ...
import { createHash } from "node:crypto";
import { createReadStream, createWriteStream } from "node:fs";
import { mkdir, readdir, stat, writeFile, readFile } from "node:fs/promises";
import path from "node:path";

const USAGE = `Cách dùng:
  make <thư-mục> [--large-mb N]   Tạo fixture (mặc định tệp lớn 256 MB, 0 để bỏ qua)
  manifest <đường-dẫn> [--out f]  In manifest (tệp, thư mục rỗng, kích thước, SHA-256)
  compare <nguồn> <đích>          Đối chiếu từng mục của nguồn với đích (thoát mã 1 nếu lệch)`;

function sha256File(file) {
  return new Promise((resolve, reject) => {
    const hash = createHash("sha256");
    createReadStream(file)
      .on("error", reject)
      .on("data", (chunk) => hash.update(chunk))
      .on("end", () => resolve(hash.digest("hex").toUpperCase()));
  });
}

// Duyệt cây thư mục, trả về { files: [{path,size,sha256}], emptyDirs: [path] }.
// `path` luôn dùng dấu "/" và tương đối so với thư mục gốc để so được giữa các máy.
async function walk(root) {
  const files = [];
  const emptyDirs = [];
  async function visit(abs, rel) {
    const entries = await readdir(abs, { withFileTypes: true });
    if (entries.length === 0 && rel !== "") emptyDirs.push(rel);
    entries.sort((a, b) => a.name.localeCompare(b.name));
    for (const entry of entries) {
      const childAbs = path.join(abs, entry.name);
      const childRel = rel === "" ? entry.name : `${rel}/${entry.name}`;
      if (entry.isDirectory()) {
        await visit(childAbs, childRel);
      } else if (entry.isFile()) {
        const info = await stat(childAbs);
        files.push({ path: childRel, size: info.size, sha256: await sha256File(childAbs) });
      } else {
        throw new Error(`Loại mục không được hỗ trợ trong fixture: ${childRel}`);
      }
    }
  }
  const info = await stat(root);
  if (info.isFile()) {
    files.push({ path: path.basename(root), size: info.size, sha256: await sha256File(root) });
  } else {
    await visit(root, "");
  }
  return { files, emptyDirs };
}

// Ghi tệp lớn bằng dữ liệu giả ngẫu nhiên xác định (cùng seed -> cùng nội dung).
async function writeLargeFile(file, megabytes) {
  const chunk = Buffer.alloc(1024 * 1024);
  let state = 0x9e3779b1;
  const out = createWriteStream(file);
  for (let i = 0; i < megabytes; i++) {
    for (let j = 0; j < chunk.length; j += 4) {
      state ^= state << 13; state >>>= 0;
      state ^= state >>> 17;
      state ^= state << 5; state >>>= 0;
      chunk.writeUInt32LE(state, j);
    }
    if (!out.write(chunk)) await new Promise((r) => out.once("drain", r));
  }
  await new Promise((resolve, reject) => out.end((err) => (err ? reject(err) : resolve())));
}

async function make(dir, largeMb) {
  await mkdir(dir, { recursive: true });
  const existing = await readdir(dir);
  if (existing.length > 0) throw new Error(`Thư mục ${dir} không trống; dùng thư mục mới.`);
  await writeFile(path.join(dir, "small.txt"), "CrocCoc fixture\nxin chào\n", "utf8");
  const tree = path.join(dir, "Thư mục thử nghiệm");
  await mkdir(path.join(tree, "nhánh-con", "cháu"), { recursive: true });
  await mkdir(path.join(tree, "thư mục rỗng"), { recursive: true });
  await writeFile(path.join(tree, "tệp có khoảng trắng.txt"), "nội dung 1\n", "utf8");
  await writeFile(path.join(tree, "nhánh-con", "dữ liệu.bin"), Buffer.from(Array.from({ length: 4096 }, (_, i) => i % 251)));
  await writeFile(path.join(tree, "nhánh-con", "cháu", "ghi-chú.md"), "# ghi chú\n", "utf8");
  if (largeMb > 0) await writeLargeFile(path.join(dir, "large.bin"), largeMb);
  const manifest = await walk(dir);
  await writeFile(path.join(dir, "..", `${path.basename(dir)}.manifest.json`), JSON.stringify(manifest, null, 2));
  console.log(`Đã tạo fixture tại ${dir}`);
  console.log(`Manifest: ${path.join(path.dirname(dir), path.basename(dir) + ".manifest.json")}`);
  console.log(`${manifest.files.length} tệp, ${manifest.emptyDirs.length} thư mục rỗng.`);
}

async function compare(source, dest) {
  const src = await walk(source);
  const dst = await walk(dest);
  const dstFiles = new Map(dst.files.map((f) => [f.path, f]));
  const dstDirs = new Set(dst.emptyDirs);
  // Nếu nguồn là một tệp đơn, tìm tệp cùng tên ở gốc đích. Nếu nguồn là thư mục,
  // đích nhận các mục gốc của nó trực tiếp (Croc nhận vào thư mục đã chọn).
  const problems = [];
  let matched = 0;
  for (const f of src.files) {
    const got = dstFiles.get(f.path);
    if (!got) problems.push(`THIẾU   ${f.path}`);
    else if (got.size !== f.size) problems.push(`SAI CỠ  ${f.path} (nguồn ${f.size}, đích ${got.size})`);
    else if (got.sha256 !== f.sha256) problems.push(`SAI HASH ${f.path}`);
    else matched++;
  }
  for (const d of src.emptyDirs) {
    if (!dstDirs.has(d)) problems.push(`THIẾU THƯ MỤC RỖNG ${d}`);
  }
  const srcNames = new Set(src.files.map((f) => f.path));
  const extras = dst.files.filter((f) => !srcNames.has(f.path)).map((f) => f.path);
  console.log(`Khớp ${matched}/${src.files.length} tệp, ${src.emptyDirs.length - problems.filter((p) => p.startsWith("THIẾU THƯ MỤC")).length}/${src.emptyDirs.length} thư mục rỗng.`);
  if (extras.length) console.log(`Ghi chú: đích có ${extras.length} tệp ngoài nguồn (không tính là lỗi).`);
  if (problems.length) {
    console.log("KẾT LUẬN: FAIL");
    for (const p of problems) console.log("  " + p);
    process.exitCode = 1;
  } else {
    console.log("KẾT LUẬN: PASS");
  }
}

async function main() {
  const [cmd, ...rest] = process.argv.slice(2);
  const flag = (name, fallback) => {
    const i = rest.indexOf(name);
    return i >= 0 ? rest[i + 1] : fallback;
  };
  if (cmd === "make" && rest[0]) {
    await make(path.resolve(rest[0]), Number(flag("--large-mb", "256")));
  } else if (cmd === "manifest" && rest[0]) {
    const result = JSON.stringify(await walk(path.resolve(rest[0])), null, 2);
    const out = flag("--out");
    if (out) await writeFile(out, result); else console.log(result);
  } else if (cmd === "compare" && rest[0] && rest[1]) {
    await compare(path.resolve(rest[0]), path.resolve(rest[1]));
  } else {
    console.log(USAGE);
    process.exitCode = 2;
  }
}

main().catch((err) => {
  console.error(`Lỗi: ${err.message}`);
  process.exitCode = 2;
});
